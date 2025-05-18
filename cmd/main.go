package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/SlayerSv/load-balancer/internal/app"
	"github.com/SlayerSv/load-balancer/internal/config"
	"github.com/SlayerSv/load-balancer/internal/database/postgresql"
	"github.com/SlayerSv/load-balancer/internal/loadbalancer"
	"github.com/SlayerSv/load-balancer/internal/logger"
	"github.com/SlayerSv/load-balancer/internal/ratelimiter"
	"github.com/SlayerSv/load-balancer/internal/ratelimiter/clientcache/mapcache"

	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wg := &sync.WaitGroup{}
	cfg, err := config.NewConfig("config.json")
	if err != nil {
		slog.Error("Reading config file", "error", err)
		os.Exit(1)
	}
	level := logger.GetSlogLevel(cfg.LogLevel)
	log := logger.NewSlog(os.Stdout, &slog.HandlerOptions{Level: level})
	lb, err := loadbalancer.NewLoadBalancer(log, &cfg.LoadBalancer)
	if err != nil {
		log.Error("Creating load balancer", "error", err)
		os.Exit(1)
	}
	db, err := postgresql.NewPostgresDB()
	if err != nil {
		log.Error("Creating postgres database", "error", err)
		os.Exit(1)
	}
	sm := mapcache.NewMapCache(&cfg.RateLimiter.Cache, log)
	rl := ratelimiter.NewRateLimiterBucket(&cfg.RateLimiter, db, sm, log)
	app := app.NewApp(cfg, db, lb, rl, log)
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: app.Middleware(rl.Middleware(app.LB)),
	}
	log.Info("Server starting", "port", cfg.Port)
	go func() {
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}()

	// start services
	lb.StartHealthChecks(ctx, wg)
	rl.AddTokensInterval(ctx, wg)
	rl.SaveStateInterval(ctx, wg)
	rl.RemoveStaleInterval(ctx, wg)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
	log.Info("Received shutdown signal, shutting down...")
	cancel()
	// Shutdown server with a timeout
	ctx1, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = server.Shutdown(ctx1)
	wg.Wait()
	if err != nil {
		slog.Error("Server shutdown failed", "error", err)
	} else {
		slog.Info("Server shutdown successfully")
	}
}
