package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	BackendURLs         []string `json:"backend_urls"`
	Port                int      `json:"port"`
	WorkerCount         int      `json:"worker_count"`
	HealthCheckInterval int      `json:"health_check_interval"`
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
