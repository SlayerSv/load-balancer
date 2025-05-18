package loadbalancer

import (
	"context"
	"io"
	"log"
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
	lb.Log.Debug("Starting health check service")
	// tasks is the channel to distribute health check tasks
	tasks := make(chan *Backend, len(lb.backends))
	// client is the http client for doing health checks by goroutines
	client := http.Client{
		Timeout: time.Second * time.Duration(lb.cfg.RequestTimeout),
	}
	for range lb.cfg.HealthCheckWorkerCount {
		wg.Add(1)
		go func() {
			lb.Log.Debug("Starting health check worker")
			defer wg.Done()
			for {
				select {
				case b := <-tasks:
					lb.healthCheck(client, b)
				case <-ctx.Done():
					lb.Log.Debug("Shutting down health check worker")
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
				lb.Log.Debug("Starting health checks for all backends")
				for _, b := range lb.backends {
					tasks <- b
				}
			case <-ctx.Done():
				close(tasks)
				lb.Log.Debug("Shutting down health check service")
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
	lb.Log.Debug("Finished health check on backend", "backend", b.URL.Path, "is_healthy", b.isHealthy)
}

func (lb *LoadBalancer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for range lb.cfg.MaxRetries {
		backend, err := lb.GetBackend()
		if err != nil {
			lb.Log.Warn("No healthy backends available", "request_id", r.Context().Value(pkg.RequestID))
			apperrors.Error(w, r, err)
			return
		}
		// Create a new proxy for the backend
		proxy := httputil.NewSingleHostReverseProxy(backend.URL)
		proxy.Transport = &http.Transport{
			ResponseHeaderTimeout: time.Second * time.Duration(lb.cfg.RequestTimeout),
		}
		proxy.ErrorLog = log.New(io.Discard, "", 0)
		// Capture the response to check for errors
		recorder := &responseRecorder{ResponseWriter: w}
		proxy.ServeHTTP(recorder, r)
		// Check if the response indicates a backend failure
		if recorder.statusCode >= 500 || recorder.statusCode == http.StatusBadGateway ||
			recorder.statusCode == http.StatusServiceUnavailable ||
			recorder.statusCode == http.StatusGatewayTimeout {
			backend.mu.Lock()
			backend.isHealthy = false
			backend.mu.Unlock()
			lb.Log.Warn("Request to backend failed", "request_id", r.Context().Value(pkg.RequestID),
				"backend_url", backend.URL.String(), "status_code", recorder.statusCode)
			continue
		}
		recorder.writeResponse(w)
		lb.Log.Info("Request forwarded sucessfully", "request_id", r.Context().Value(pkg.RequestID),
			"backend_url", backend.URL.String())
		return
	}
	apperrors.Error(w, r, apperrors.ErrServiceUnavailable)
}

// responseRecorder captures the response from the proxy to inspect status codes
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       []byte
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body = append(r.body, b...)
	return len(b), nil
}

func (r *responseRecorder) writeResponse(w http.ResponseWriter) {
	if r.statusCode != 0 {
		w.WriteHeader(r.statusCode)
	}
	w.Write(r.body)
}
