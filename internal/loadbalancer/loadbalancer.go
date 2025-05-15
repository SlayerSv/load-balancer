package loadbalancer

import (
	"errors"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"github.com/SlayerSv/load-balancer/internal/config"
)

var errNoAvailableBackends = errors.New("no available backends")

type LoadBalancer struct {
	mu       sync.Mutex
	backends []*Backend
	current  int
	cfg      *config.Config
}

type Backend struct {
	URL       *url.URL
	isHealthy bool
	mu        sync.Mutex
}

func NewLoadBalancer(backendURLs []string) (*LoadBalancer, error) {
	lb := &LoadBalancer{}
	lb.backends = make([]*Backend, 0, len(backendURLs))
	for _, u := range backendURLs {
		parsedURL, err := url.Parse(u)
		if err != nil {
			return nil, err
		}
		lb.backends = append(lb.backends, &Backend{URL: parsedURL, isHealthy: true})
	}
	return lb, nil
}

func (lb *LoadBalancer) NextBackend() (*Backend, error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	for range lb.backends {
		lb.current = (lb.current + 1) % len(lb.backends)
		b := lb.backends[lb.current]
		b.mu.Lock()
		isHealthy := b.isHealthy
		b.mu.Unlock()
		if isHealthy {
			return b, nil
		}
	}
	return nil, errNoAvailableBackends
}

// StartHealthChecks launches a worker pool for periodic backend health checks
func (lb *LoadBalancer) StartHealthChecks() {
	// Channel to distribute health check tasks
	tasks := make(chan *Backend, len(lb.backends))

	// Start worker pool
	var wg sync.WaitGroup
	for i := 0; i < lb.cfg.WorkerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for b := range tasks {
				lb.healthCheck(b)
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
func (lb *LoadBalancer) healthCheck(b *Backend) {
	resp, err := http.Get(b.URL.String() + "/health")
	b.mu.Lock()
	defer b.mu.Unlock()
	if err != nil || resp != nil && resp.StatusCode != http.StatusOK {
		if b.isHealthy { // Only log if state changes
			slog.Warn("Backend is down", "backend", b.URL.String(), "error", err)
		}
		b.isHealthy = false
	} else {
		if !b.isHealthy { // Only log if state changes
			slog.Info("Backend is up", "backend", b.URL.String())
		}
		b.isHealthy = true
	}
}

func (lb *LoadBalancer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	backend, err := lb.NextBackend()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		slog.Error("No healthy backends available", "path", r.URL.Path)
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(backend.URL)
	proxy.ServeHTTP(w, r)
	slog.Info("Request forwarded", "backend", backend.URL.String(), "path", r.URL.Path)
}
