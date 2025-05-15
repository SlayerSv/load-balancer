package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}()
	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
	slog.Info("Received shutdown signal, shutting down...")

	// Shutdown server with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("Server shutdown failed", "error", err)
	} else {
		slog.Info("Server shutdown successfully")
	}
}
