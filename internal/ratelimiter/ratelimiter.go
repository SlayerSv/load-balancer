package ratelimiter

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/SlayerSv/load-balancer/internal/apperrors"
	"github.com/SlayerSv/load-balancer/internal/config"
	"github.com/SlayerSv/load-balancer/internal/database"
	"github.com/SlayerSv/load-balancer/internal/logger"
	"github.com/SlayerSv/load-balancer/internal/ratelimiter/clientcache"
)

type RateLimiter interface {
	AllowRequest(APIKey string) error
}

// RateLimiter manages rate limiting for clients
type RateLimiterBucket struct {
	Cfg           *config.ConfigRateLimiter
	DB            database.DataBase
	cache         clientcache.ClientCache
	Log           logger.Logger
	clientHandler http.Handler
}

// NewRateLimiter creates a new RateLimiter with a database connection
func NewRateLimiterBucket(cfg *config.ConfigRateLimiter, DB database.DataBase, cache clientcache.ClientCache, log logger.Logger) *RateLimiterBucket {
	rl := &RateLimiterBucket{Cfg: cfg, DB: DB, cache: cache, Log: log}
	rl.clientHandler = rl.NewClientHandler()
	return rl
}

// AllowRequest checks if a request is allowed based on the token bucket
func (rl *RateLimiterBucket) AllowRequest(APIKey string) error {
	err := rl.cache.AllowRequest(APIKey)
	if errors.Is(err, apperrors.ErrNotFound) {
		rl.Log.Debug("Api key not found in cache", "api_key", APIKey)
		client, err := rl.DB.GetClientByAPIKey(context.Background(), APIKey)
		if err != nil {
			if errors.Is(err, apperrors.ErrNotFound) {
				rl.Log.Debug("Api key not found in database", "api_key", APIKey)
				return fmt.Errorf("%w: invalid api key", apperrors.ErrUnauthorized)
			}
			return err
		}
		err = rl.cache.AddClient(client)
		if err != nil && !errors.Is(err, apperrors.ErrAlreadyExists) {
			return err
		}
		return rl.cache.AllowRequest(APIKey)
	}
	return err
}

func (rl *RateLimiterBucket) AddTokensInterval(ctx context.Context, wg *sync.WaitGroup) {
	wg.Add(1)
	defer wg.Done()
	ticker := time.NewTicker(time.Duration(rl.Cfg.AddTokensInterval) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.cache.AddTokensToAll()
		case <-ctx.Done():
			return
		}
	}
}

func (rl *RateLimiterBucket) SaveStateInterval(ctx context.Context, wg *sync.WaitGroup) {
	wg.Add(1)
	defer wg.Done()
	ticker := time.NewTicker(time.Duration(rl.Cfg.SaveStateInterval) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.cache.SaveState(rl.DB)
		case <-ctx.Done():
			return
		}
	}
}

func (rl *RateLimiterBucket) RemoveStaleInterval(ctx context.Context, wg *sync.WaitGroup) {
	wg.Add(1)
	defer wg.Done()
	ticker := time.NewTicker(time.Duration(rl.Cfg.RemoveStaleInterval) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.cache.RemoveStale()
		case <-ctx.Done():
			return
		}
	}
}
