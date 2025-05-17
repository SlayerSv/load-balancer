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
	mu       sync.Mutex
	backends []*Backend
	current  int
	cfg      *config.Config
	Log      logger.Logger
}

type Backend struct {
	URL       *url.URL
	isHealthy atomic.Bool
}

func NewLoadBalancer(log logger.Logger, cfg *config.Config) (*LoadBalancer, error) {
	lb := &LoadBalancer{Log: log}
	lb.backends = make([]*Backend, 0, len(cfg.BackendURLs))
	for _, u := range cfg.BackendURLs {
		parsedURL, err := url.Parse(u)
		if err != nil {
			return nil, err
		}
		b := &Backend{URL: parsedURL, isHealthy: atomic.Bool{}}
		b.isHealthy.Store(true)
		lb.backends = append(lb.backends, b)
	}
	return lb, nil
}

func (lb *LoadBalancer) NextBackend() (*Backend, error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	for range lb.backends {
		lb.current = (lb.current + 1) % len(lb.backends)
		b := lb.backends[lb.current]
		isHealthy := b.isHealthy.Load()
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
	b.isHealthy.Load()

	if err != nil || resp.StatusCode != http.StatusOK {
		wasHealthy := b.isHealthy.Swap(false)
		if wasHealthy { // Only log if state changes
			lb.Log.Warn("Backend is down", "backend_url", b.URL.String(), "error", err)
		}
	} else {
		wasHealthy := b.isHealthy.Swap(true)
		if !wasHealthy { // Only log if state changes
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
