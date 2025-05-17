package loadbalancer

import (
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SlayerSv/load-balancer/internal/apperrors"
	"github.com/SlayerSv/load-balancer/internal/config"
	"github.com/SlayerSv/load-balancer/internal/logger"
)

var errNoAvailableBackends = errors.New("no available backends")

type LoadBalancer struct {
	backends []*Backend
	current  atomic.Int64
	cfg      *config.Config
	Log      logger.Logger
}

type Backend struct {
	URL       *url.URL
	isHealthy bool
	mu        sync.RWMutex
}

func NewLoadBalancer(log logger.Logger, cfg *config.Config) (*LoadBalancer, error) {
	lb := &LoadBalancer{Log: log}
	lb.backends = make([]*Backend, 0, len(cfg.BackendURLs))
	for _, u := range cfg.BackendURLs {
		parsedURL, err := url.Parse(u)
		if err != nil {
			return nil, err
		}
		b := &Backend{URL: parsedURL, isHealthy: true}
		lb.backends = append(lb.backends, b)
	}
	return lb, nil
}

func (lb *LoadBalancer) NextBackend() (*Backend, error) {
	for range lb.backends {
		ind := lb.current.Add(1) % int64(len(lb.backends))
		b := lb.backends[ind]
		b.mu.RLock()
		defer b.mu.RUnlock()
		if b.isHealthy {
			return b, nil
		}
	}
	return nil, errNoAvailableBackends
}

// StartHealthChecks launches a worker pool for periodic backend health checks
func (lb *LoadBalancer) StartHealthChecks() {
	// Channel to distribute health check tasks
	tasks := make(chan *Backend, len(lb.backends))
	client := http.Client{
		Timeout: time.Second, // Set timeout to 10 seconds
	}

	// Start worker pool
	var wg sync.WaitGroup
	for i := 0; i < lb.cfg.WorkerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for b := range tasks {
				lb.healthCheck(client, b)
			}
		}()
	}

	// Start ticker to periodically queue health checks
	go func() {
		ticker := time.NewTicker(time.Duration(lb.cfg.HealthCheckInterval) * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			for _, b := range lb.backends {
				tasks <- b
			}
		}
	}()
}

// healthCheck performs a health check on a single backend
func (lb *LoadBalancer) healthCheck(client http.Client, b *Backend) {
	resp, err := client.Get(b.URL.String() + "/health")
	b.mu.Lock()
	defer b.mu.Unlock()
	if err != nil || resp.StatusCode != http.StatusOK {
		if b.isHealthy {
			b.isHealthy = false
			lb.Log.Warn("Backend is down", "backend_url", b.URL.String(), "error", err)
		}
	} else {
		if b.isHealthy {
			b.isHealthy = true
			lb.Log.Info("Backend is up", "backend_url", b.URL.String())
		}
	}
}

func (lb *LoadBalancer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	backend, err := lb.NextBackend()
	if errors.Is(err, errNoAvailableBackends) {
		apperrors.Error(w, r, errNoAvailableBackends)
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(backend.URL)
	lb.Log.Info("Forwarding request", "backend_url", backend.URL.Path)
	proxy.ServeHTTP(w, r)
}
