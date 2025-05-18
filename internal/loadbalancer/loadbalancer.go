package loadbalancer

import (
	"context"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SlayerSv/load-balancer/internal/apperrors"
	"github.com/SlayerSv/load-balancer/internal/config"
	"github.com/SlayerSv/load-balancer/internal/logger"
	"github.com/SlayerSv/load-balancer/pkg"
)

// ILoadBalancer is the interface for load balancer service
type ILoadBalancer interface {
	GetBackend() (*Backend, error)
	http.Handler
}

// LoadBalancer is the implementation of a ILoadBalancer using round robin algorithm
type LoadBalancer struct {
	// backends is the list of all backends to which load balancer will forward requests
	backends []*Backend
	// current is the pointer to current backend in backends slice
	current atomic.Int64
	// cfg is the config for load balancer
	cfg *config.ConfigLoadBalancer
	// Log is the logger for load balancer
	Log logger.Logger
}

// Backend represents backend instance for forwarding requests
type Backend struct {
	// URL is the url object for parsed request url
	URL *url.URL
	// isHealthy reports if backend is currently healthy
	isHealthy bool
	// mu is RW mutex for accessing Backend
	mu sync.RWMutex
}

func NewLoadBalancer(log logger.Logger, cfg *config.ConfigLoadBalancer) (*LoadBalancer, error) {
	lb := &LoadBalancer{Log: log, cfg: cfg}
	lb.backends = make([]*Backend, 0, len(lb.cfg.BackendURLs))
	for _, u := range lb.cfg.BackendURLs {
		parsedURL, err := url.Parse(u)
		if err != nil {
			return nil, err
		}
		b := &Backend{URL: parsedURL, isHealthy: true}
		lb.backends = append(lb.backends, b)
	}
	return lb, nil
}

// GetBackend returns next healthy backend using round robin algorithm or error
// if no backends are currently healthy
func (lb *LoadBalancer) GetBackend() (*Backend, error) {
	for range lb.backends {
		ind := lb.current.Add(1) % int64(len(lb.backends))
		b := lb.backends[ind]
		b.mu.RLock()
		defer b.mu.RUnlock()
		if b.isHealthy {
			return b, nil
		}
	}
	return nil, apperrors.ErrServiceUnavailable
}

// StartHealthChecks launches a worker pool for periodic backend health checks
func (lb *LoadBalancer) StartHealthChecks(ctx context.Context, wg *sync.WaitGroup) {
	// tasks is the channel to distribute health check tasks
	tasks := make(chan *Backend, len(lb.backends))
	// client is the http client for doing health checks by goroutines
	client := http.Client{
		Timeout: time.Second * time.Duration(lb.cfg.HealthCheckTimeout),
	}
	for range lb.cfg.HealthCheckWorkerCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case b := <-tasks:
					lb.healthCheck(client, b)
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	wg.Add(1)
	// Start ticker to periodically queue health checks
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(time.Duration(lb.cfg.HealthCheckInterval) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				// put all backends in channel for health checks
				for _, b := range lb.backends {
					tasks <- b
				}
			case <-ctx.Done():
				close(tasks)
				return
			}
		}
	}()
}

// healthCheck performs a health check on a single backend
func (lb *LoadBalancer) healthCheck(client http.Client, b *Backend) {
	resp, err := client.Get(b.URL.String() + "/health")
	// lock mutex AFTER get request for reducing lock times
	b.mu.Lock()
	defer b.mu.Unlock()
	// udpate state, log if state of backend changes
	if err != nil || resp.StatusCode != http.StatusOK {
		if b.isHealthy {
			b.isHealthy = false
			lb.Log.Warn("Backend is down", "backend_url", b.URL.String(), "error", err)
		}
	} else {
		if !b.isHealthy {
			b.isHealthy = true
			lb.Log.Info("Backend is up", "backend_url", b.URL.String())
		}
	}
}

func (lb *LoadBalancer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	backend, err := lb.GetBackend()
	if err != nil {
		apperrors.Error(w, r, err)
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(backend.URL)
	lb.Log.Info("Forwarding request", "request_id", r.Context().Value(pkg.RequestID), "backend_url", backend.URL.Path)
	proxy.ServeHTTP(w, r)
}
