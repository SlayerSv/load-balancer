package config

import (
	"encoding/json"
	"os"
)

// Config is the main config of application.
type Config struct {
	// Port is the port which this application will use for connections
	Port int `json:"port"`
	// LogLevel is the minimum level of logger messages that will be printed (debug, info, warn, error).
	LogLevel string `json:"log_level"`
	// LogFile is the path to file for logging (will be created if missing). If LogFile is an empty string then stdout is used.
	LogFile      string             `json:"log_file"`
	LoadBalancer ConfigLoadBalancer `json:"load_balancer"`
	RateLimiter  ConfigRateLimiter  `json:"rate_limiter"`
}

// ConfigLoadBalancer is the config for load balancer service
type ConfigLoadBalancer struct {
	// BackendURLs is the list of backends to which load balancer will redirect requests
	BackendURLs []string `json:"backend_urls"`
	// HealthCheckWorkerCount is the number of workers that will do health checks on backends
	HealthCheckWorkerCount int `json:"health_check_worker_count"`
	// HealthCheckInterval is the interval between health checks on backends
	HealthCheckInterval float64 `json:"health_check_interval"`
	// MaxRetries is the maximum retries for forwarding requests to backends
	MaxRetries int `json:"max_retries"`
	// RequestTimeout is the duration in seconds for waiting a response
	RequestTimeout float64 `json:"request_timeout"`
}

// ConfigRateLimiter is the config for rate limiter service
type ConfigRateLimiter struct {
	// AddTokensInterval is the interval in seconds between adding tokens to clients
	AddTokensInterval float64 `json:"add_tokens_interval"`
	// SaveStateInterval is the interval in seconds between saving state of clients in cache to database
	SaveStateInterval float64 `json:"save_state_interval"`
	// RemoveStaleInterval is the interval in seconds between running function to remove stale objects from cache
	RemoveStaleInterval float64 `json:"remove_stale_interval"`
	// Cache is the rata limiter in memory cache for clients
	Cache ConfigRateLimiterCache `json:"cache"`
	// DefaultRatePerSec is the default amount of tokens added to a client per second
	DefaultRatePerSec int `json:"default_rate_per_sec"`
	// DefaultCapacity is the default capacity of tokens for clients
	DefaultCapacity int `json:"default_capacity"`
}

// ConfigRateLimiterCache is the config for rate limiter cache
type ConfigRateLimiterCache struct {
	// TimeToLive determines duration in seconds of an object not being used in cache
	// after which the objects get removed from the cache.
	TimeToLive float64 `json:"time_to_live"`
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
