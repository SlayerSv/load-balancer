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

	"github.com/SlayerSv/load-balancer/internal/app"
	"github.com/SlayerSv/load-balancer/internal/config"
	"github.com/SlayerSv/load-balancer/internal/database/postgresql"
	"github.com/SlayerSv/load-balancer/internal/loadbalancer"
	"github.com/SlayerSv/load-balancer/internal/logger"
	"github.com/SlayerSv/load-balancer/internal/ratelimiter"
	"github.com/SlayerSv/load-balancer/internal/ratelimiter/clientcache/mapcache"
)

func main() {
	cfg, err := config.NewConfig("config.json")
	if err != nil {
		slog.Error("Reading config file", "error", err)
		os.Exit(1)
	}
	log := logger.NewSlog(os.Stdout, nil)
	lb, err := loadbalancer.NewLoadBalancer(log, cfg)
	if err != nil {
		log.Error("Creating load balancer", "error", err)
		os.Exit(1)
	}
	lb.StartHealthChecks()
	db, err := postgresql.NewPostgresDB()
	if err != nil {
		log.Error("Creating postgres database", "error", err)
		os.Exit(1)
	}
	sm := mapcache.NewMapCache(log)
	rl := ratelimiter.NewRateLimiterBucket(db, sm, log)
	app := app.NewApp(cfg, db, lb, rl, log)
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: app.Middleware(app.LB),
	}
	log.Info("Server starting", "port", cfg.Port)
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
