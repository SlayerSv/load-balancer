package loadbalancer

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
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
	isHealthy atomic.Bool
}

type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func NewLoadBalancer(backendURLs []string) (*LoadBalancer, error) {
	lb := &LoadBalancer{}
	lb.backends = make([]*Backend, 0, len(backendURLs))
	for _, u := range backendURLs {
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
			slog.Warn("Backend is down", "backend_url", b.URL.String(), "error", err)
		}
	} else {
		wasHealthy := b.isHealthy.Swap(true)
		if !wasHealthy { // Only log if state changes
			slog.Info("Backend is up", "backend_url", b.URL.String())
		}
	}
}

func (lb *LoadBalancer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	backend, err := lb.NextBackend()
	if errors.Is(err, errNoAvailableBackends) {
		slog.Error("No available backends", "request_url", r.URL.Path)
		lb.Error(w, r, errNoAvailableBackends.Error(), http.StatusServiceUnavailable)
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(backend.URL)
	proxy.ServeHTTP(w, r)
	slog.Info("Request forwarded", "backend_url", backend.URL.String(), "request_url", r.URL.Path)
}

func (lb *LoadBalancer) Error(w http.ResponseWriter, r *http.Request, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	errorResponse := ErrorResponse{
		Code:    code,
		Message: message,
	}
	w.WriteHeader(code)
	err := json.NewEncoder(w).Encode(errorResponse)
	if err != nil {
		slog.Error("Encoding error response", "error", err)
	}
}
