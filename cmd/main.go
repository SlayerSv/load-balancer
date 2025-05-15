package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/SlayerSv/load-balancer/internal/config"
	"github.com/SlayerSv/load-balancer/internal/loadbalancer"
)

func main() {
	cfg, err := config.NewConfig("config.json")
	if err != nil {
		slog.Error("Reading config file", "error", err)
		os.Exit(1)
	}
	lb, err := loadbalancer.NewLoadBalancer(cfg.BackendURLs)
	if err != nil {
		slog.Error("Creating load balancer", "error", err)
		os.Exit(1)
	}
	lb.StartHealthChecks()
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: lb,
	}
	slog.Info("Server starting", "port", cfg.Port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}
}
