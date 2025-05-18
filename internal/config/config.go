package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	Port         int                `json:"port"`
	LoadBalancer ConfigLoadBalancer `json:"load_balancer"`
	RateLimiter  ConfigRateLimiter  `json:"rate_limiter"`
}

type ConfigLoadBalancer struct {
	BackendURLs         []string `json:"backend_urls"`
	WorkerCount         int      `json:"worker_count"`
	HealthCheckInterval int      `json:"health_check_interval"`
	HealthCheckTimeout  int      `json:"health_check_timeout"`
}

type ConfigRateLimiter struct {
	AddTokensInterval   int                    `json:"add_tokens_interval"`
	SaveStateInterval   int                    `json:"save_state_interval"`
	RemoveStaleInterval int                    `json:"remove_stale_interval"`
	Cache               ConfigRateLimiterCache `json:"cache"`
}

type ConfigRateLimiterCache struct {
	TimeToLive int `json:"time_to_live"`
}

func NewConfig(path string) (cfg *Config, err error) {
	file, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		return
	}
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	err = decoder.Decode(cfg)
	return
}
