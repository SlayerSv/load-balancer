package mapcache

import (
	"context"
	"sync"
	"time"

	"github.com/SlayerSv/load-balancer/internal/apperrors"
	"github.com/SlayerSv/load-balancer/internal/config"
	"github.com/SlayerSv/load-balancer/internal/database"
	"github.com/SlayerSv/load-balancer/internal/logger"
	"github.com/SlayerSv/load-balancer/internal/models"
)

type MapCache struct {
	Cfg   *config.ConfigRateLimiterCache
	Log   logger.Logger
	cache map[string]*models.ClientCache
	pool  *sync.Pool
	mu    sync.RWMutex
}

func NewMapCache(cfg *config.ConfigRateLimiterCache, log logger.Logger) *MapCache {
	sm := &MapCache{
		Cfg:   cfg,
		cache: make(map[string]*models.ClientCache),
		pool: &sync.Pool{
			New: func() any {
				return &models.ClientCache{}
			},
		},
		Log: log,
	}
	return sm
}

func (sm *MapCache) AllowRequest(APIKey string) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	client, ok := sm.cache[APIKey]
	if !ok {
		return apperrors.ErrNotFound
	}
	client.Mu.Lock()
	defer client.Mu.Unlock()
	if client.Tokens > 0 {
		client.Tokens--
		client.HasChanged = true
		client.Expires = time.Now().Add(time.Duration(sm.Cfg.TimeToLive) * time.Second)
		return nil
	}
	return apperrors.ErrRateLimitExceeded
}

func (sm *MapCache) AddClient(client models.Client) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	_, has := sm.cache[client.APIKey]
	if has {
		sm.Log.Debug("Client already in cache", "client_id", client.ClientID, "api_key", client.APIKey)
		return apperrors.ErrAlreadyExists
	}
	cl := sm.pool.Get().(*models.ClientCache)
	cl.Copy(client)
	cl.Expires = time.Now().Add(time.Duration(sm.Cfg.TimeToLive) * time.Second)
	sm.AddTokens(cl)
	sm.cache[client.APIKey] = cl
	sm.Log.Debug("Added client to cache", "client_id", client.ClientID, "api_key", client.APIKey)
	return nil
}

func (sm *MapCache) GetClient(APIKey string) (models.Client, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	client, ok := sm.cache[APIKey]
	if !ok {
		sm.Log.Debug("Client not found in cache", "api_key", APIKey)
		return models.Client{}, apperrors.ErrNotFound
	}
	sm.Log.Debug("Found client in cache", "client_id", client.ClientID, "api_key", APIKey)
	return client.Client, nil
}

func (sm *MapCache) UpdateClient(client models.Client) (models.Client, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	cl, has := sm.cache[client.APIKey]
	if !has {
		sm.Log.Debug("Client not found in cache", "client_id", client.ClientID, "api_key", client.APIKey)
		return models.Client{}, apperrors.ErrNotFound
	}
	cl.Mu.Lock()
	defer cl.Mu.Unlock()
	cl.Capacity = client.Capacity
	cl.RatePerSec = client.RatePerSec
	cl.Tokens = min(cl.Tokens, cl.Capacity)
	return cl.Client, nil
}

func (sm *MapCache) DeleteClient(APIKey string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	client, ok := sm.cache[APIKey]
	if !ok {
		return apperrors.ErrNotFound
	}
	delete(sm.cache, APIKey)
	sm.pool.Put(client)
	return nil
}

func (sm *MapCache) AddTokensToAll() {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	for _, client := range sm.cache {
		sm.AddTokens(client)
	}
}

func (sm *MapCache) AddTokens(client *models.ClientCache) {
	client.Mu.Lock()
	defer client.Mu.Unlock()
	if client.Tokens == client.Capacity {
		return
	}
	now := time.Now()
	// Refill tokens based on elapsed time
	elapsed := now.Sub(client.LastRefill).Seconds()
	tokensToAdd := int(elapsed * float64(client.RatePerSec))
	if tokensToAdd > 0 && client.Tokens != client.Capacity {
		client.Tokens = min(client.Capacity, client.Tokens+tokensToAdd)
		client.LastRefill = now
		client.HasChanged = true
	}
	sm.Log.Debug("Added tokens", "client", &client, "tokens", tokensToAdd, "elapsed", elapsed, "now", now)
}

func (sm *MapCache) SaveState(DB database.DataBase) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	for _, client := range sm.cache {
		client.Mu.Lock()
		if !client.HasChanged {
			client.Mu.Unlock()
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_, err := DB.UpdateTokens(ctx, client.Client)
		cancel()
		if err != nil {
			sm.Log.Error("Saving client's state to database", "client_id", client.ClientID, "api_key", client.APIKey)
			client.Mu.Unlock()
			continue
		}
		client.HasChanged = false
		client.Mu.Unlock()
	}
}

func (sm *MapCache) RemoveStale() {
	sm.mu.RLock()
	forDeletion := make([]*models.ClientCache, 0)
	now := time.Now()
	for _, client := range sm.cache {
		client.Mu.RLock()
		if now.After(client.Expires) && !client.HasChanged {
			forDeletion = append(forDeletion, client)
		}
		client.Mu.RUnlock()
	}
	sm.mu.RUnlock()
	if len(forDeletion) == 0 {
		return
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()
	for i, c := range forDeletion {
		forDeletion[i] = nil // make life easier for garbage collector
		client, ok := sm.cache[c.APIKey]
		if !ok || client != c {
			continue
		}
		client.Mu.RLock()
		if now.After(client.Expires) && !client.HasChanged {
			delete(sm.cache, client.APIKey)
			sm.pool.Put(client)
		}
		client.Mu.RUnlock()
	}
	forDeletion = nil
}
