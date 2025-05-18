package config

import (
	"encoding/json"
	"os"
)

// Config is the main config of application
type Config struct {
	// Port is the port which this application will use for connections
	Port int `json:"port"`
	// LogLevel is the minimum level of logger messages that will be printed.
	LogLevel     string             `json:"log_level"`
	LoadBalancer ConfigLoadBalancer `json:"load_balancer"`
	RateLimiter  ConfigRateLimiter  `json:"rate_limiter"`
}

// ConfigLoadBalancer is the config for load balancer service
type ConfigLoadBalancer struct {
	// BackendURLs is the list of backends to which load balancer will redirect requests
	BackendURLs []string `json:"backend_urls"`
	// HealthCheckWorkerCount is the number of workers that will do health checks on backends
	HealthCheckWorkerCount int `json:"worker_count"`
	// HealthCheckInterval is the interval between health checks on backends
	HealthCheckInterval int `json:"health_check_interval"`
	// HealthCheckTimeout is the timeout for health checks
	HealthCheckTimeout int `json:"health_check_timeout"`
}

// ConfigRateLimiter is the config for rate limiter service
type ConfigRateLimiter struct {
	// AddTokensInterval is the interval between adding tokens to clients
	AddTokensInterval int `json:"add_tokens_interval"`
	// SaveStateInterval is the interval between saving state of clients in cache to database
	SaveStateInterval int `json:"save_state_interval"`
	// RemoveStaleInterval is the interval between running function to remove stale objects from cache
	RemoveStaleInterval int                    `json:"remove_stale_interval"`
	Cache               ConfigRateLimiterCache `json:"cache"`
}

// ConfigRateLimiterCache is the config for rate limiter cache
type ConfigRateLimiterCache struct {
	// TimeToLive determines duration of an object not being used in cache
	// after which the objects get removed from the cache.
	TimeToLive int `json:"time_to_live"`
}

// NewConfig reads config file and returns the pointer to config object
func NewConfig(path string) (*Config, error) {
	file, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	cfg := &Config{}
	err = decoder.Decode(cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}
