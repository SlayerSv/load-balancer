package ratelimiter

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/SlayerSv/load-balancer/internal/database"
	"github.com/SlayerSv/load-balancer/internal/logger"
	"github.com/SlayerSv/load-balancer/internal/ratelimiter/clientcache"
)

type RateLimiter interface {
	AllowRequest(APIKey string) (bool, error)
}

// RateLimiter manages rate limiting for clients
type RateLimiterBucket struct {
	DB            database.DataBase
	cache         clientcache.ClientCache
	Log           logger.Logger
	clientHandler http.Handler
	mu            sync.Mutex
}

// NewRateLimiter creates a new RateLimiter with a database connection
func NewRateLimiterBucket(DB database.DataBase, cache clientcache.ClientCache, log logger.Logger) *RateLimiterBucket {
	rl := &RateLimiterBucket{DB: DB, cache: cache, Log: log}
	rl.clientHandler = rl.NewClientHandler()
	return rl
}

// AllowRequest checks if a request is allowed based on the token bucket
func (rl *RateLimiterBucket) AllowRequest(APIKey string) (bool, error) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	client, err := rl.cache.GetClient(APIKey)
	if err != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		clientV, err := rl.DB.GetClient(ctx, APIKey)
		if err != nil {
			return false, err
		}
		defer func() {
			rl.cache.AddClient(clientV)
		}()
		if clientV.Tokens > 0 {
			clientV.Tokens--
			clientV.HasChanged = true
			return true, nil
		}
		return false, nil

	}
	if client.Tokens > 0 {
		client.Tokens--
		client.HasChanged = true
		return true, nil
	}
	return false, nil
}

func (rl *RateLimiterBucket) AddTokensInterval(ctx context.Context, wg *sync.WaitGroup, interval time.Duration) {
	wg.Add(1)
	defer wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			rl.cache.AddTokensToAll()
			rl.mu.Unlock()
		case <-ctx.Done():
			return
		}
	}
}

func (rl *RateLimiterBucket) SaveStateInterval(ctx context.Context, wg *sync.WaitGroup, interval time.Duration) {
	wg.Add(1)
	defer wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			rl.cache.SaveState(rl.DB)
			rl.mu.Unlock()
		case <-ctx.Done():
			return
		}
	}
}

func (rl *RateLimiterBucket) RemoveStaleInterval(ctx context.Context, wg *sync.WaitGroup, interval time.Duration) {
	wg.Add(1)
	defer wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			rl.cache.RemoveStale(interval)
			rl.mu.Unlock()
		case <-ctx.Done():
			return
		}
	}
}
