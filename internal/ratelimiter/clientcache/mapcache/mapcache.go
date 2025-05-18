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

// MapCache represents ClientCache implementation using token bucket algorithm.
// Uses a map as a cache for clients reducing database operations.
// Uses Read Write mutex for accessing cache. Most common operations
// require read lock, reducing lock times.
type MapCache struct {
	Cfg *config.ConfigRateLimiterCache
	Log logger.Logger
	// cache is the temporary in memory storage of recently used clients
	cache map[string]*models.ClientCache
	// pool is a sync pool for reusing client objects concurrently
	pool *sync.Pool
	// mu is RW mutex for accessing cache.
	mu sync.RWMutex
}

// NewMapCache returns pointer to instance of MapCache
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

// AllowRequest finds client in cache by api key,
// checks if client has enough tokens, subtracts those
// tokens to grant access. Returns error if client not found
// or does not have enough tokens. Increases expiration time
// and sets HasChanged to true.
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

// AddClient adds provided client to cache.
func (sm *MapCache) AddClient(client models.Client) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	_, has := sm.cache[client.APIKey]
	if has {
		sm.Log.Debug("Client already in cache", "client_id", client.ClientID, "api_key", client.APIKey)
		return apperrors.ErrAlreadyExists
	}
	// reuse ClientCache objects from sync pool
	cl := sm.pool.Get().(*models.ClientCache)
	// copy fields
	cl.Copy(client)
	// set expiration date
	cl.Expires = time.Now().Add(time.Duration(sm.Cfg.TimeToLive) * time.Second)
	// run add tokens to add tokens
	sm.AddTokens(cl)
	// save to cache
	sm.cache[client.APIKey] = cl
	sm.Log.Debug("Added client to cache", "client_id", client.ClientID, "api_key", client.APIKey)
	return nil
}

// GetClient returns client from a cache or error
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

// UpdateClient updates RatePerSec and Capacity fields of a client and returns the updated client.
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

// DeleteClient deletes client from cache
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

// AddTokensToAll runs AddToken func for all clients in cache
func (sm *MapCache) AddTokensToAll() {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	sm.Log.Debug("Starting adding tokens to clients")
	for _, client := range sm.cache {
		sm.AddTokens(client)
	}
	sm.Log.Debug("Finished adding tokens to clients")
}

// AddTokens adds tokens to a client based on elapsed time since
// last refill and clients tokens rate per sec.
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

// SaveState saves state of changed clients in cache to database.
func (sm *MapCache) SaveState(DB database.DataBase) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	sm.Log.Debug("Starting saving state of clients")
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
	sm.Log.Debug("Finished saving state of clients")
}

// RemoveStale removes clients from cache, whose Expires field has passed current time.
// Uses 2 passes for reducing mutex lock times. First pass is read only, during which
// clients for deletion are collected in a slice. Second is a write lock pass,
// which removes for deletion clients from cache, making sure that client's state
// has not changed and they are still should be removed
func (sm *MapCache) RemoveStale() {
	// first pass is read only for performance
	sm.mu.RLock()
	removed := 0
	sm.Log.Debug("Starting removing old clients from cache")
	defer sm.Log.Debug("Finished removing old clients from cache", "removed", removed)
	// slices for candidates for removal
	forDeletion := make([]*models.ClientCache, 0)
	now := time.Now()
	// collect candidates for deletion
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
	// write lock pass
	sm.mu.Lock()
	defer sm.mu.Unlock()
	for i, c := range forDeletion {
		forDeletion[i] = nil // make life easier for garbage collector
		// refetch client from cache by api key to make sure it is still same object
		client, ok := sm.cache[c.APIKey]
		if !ok || client != c {
			continue
		}
		client.Mu.RLock()
		// recheck condition for deletion, since it may have been changed
		if now.After(client.Expires) && !client.HasChanged {
			removed++
			delete(sm.cache, client.APIKey)
			sm.pool.Put(client)
		}
		client.Mu.RUnlock()
	}
	forDeletion = nil
}
