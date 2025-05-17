package mapcache

import (
	"context"
	"sync"
	"time"

	"github.com/SlayerSv/load-balancer/internal/apperrors"
	"github.com/SlayerSv/load-balancer/internal/database"
	"github.com/SlayerSv/load-balancer/internal/logger"
	"github.com/SlayerSv/load-balancer/internal/models"
)

type MapCache struct {
	Log   logger.Logger
	cache map[string]*models.Client
	pool  *sync.Pool
}

func NewMapCache(log logger.Logger) *MapCache {
	sm := &MapCache{
		cache: make(map[string]*models.Client),
		pool: &sync.Pool{
			New: func() any {
				return &models.Client{}
			},
		},
		Log: log,
	}
	return sm
}

func (sm *MapCache) AddClient(client models.Client) error {
	_, has := sm.cache[client.APIKey]
	if has {
		sm.Log.Debug("Client already in cache", "client_id", client.ClientID, "api_key", client.APIKey)
		return apperrors.ErrAlreadyExists
	}
	cl := sm.pool.New().(*models.Client)
	cl.Copy(client)
	sm.cache[client.APIKey] = cl
	sm.Log.Debug("Added client to cache", "client_id", client.ClientID, "api_key", client.APIKey)
	return nil
}

func (sm *MapCache) GetClient(APIKey string) (*models.Client, error) {
	client, ok := sm.cache[APIKey]
	if !ok {
		sm.Log.Debug("Client not found in cache", "api_key", APIKey)
		return nil, apperrors.ErrNotFound
	}
	sm.Log.Debug("Found client in cache", "client_id", client.ClientID, "api_key", APIKey)
	return client, nil
}

func (sm *MapCache) DeleteClient(APIKey string) error {
	client, has := sm.cache[APIKey]
	if !has {
		return apperrors.ErrNotFound
	}
	sm.pool.Put(client)
	delete(sm.cache, APIKey)
	return nil
}

func (sm *MapCache) AddTokensToAll() {
	now := time.Now()
	for _, client := range sm.cache {
		if client.Tokens == client.Capacity {
			continue
		}
		// Refill tokens based on elapsed time
		elapsed := now.Sub(client.LastRefill).Seconds()
		tokensToAdd := int(elapsed * float64(client.RatePerSec))
		if tokensToAdd > 0 && client.Tokens != client.Capacity {
			client.Tokens = min(client.Capacity, client.Tokens+tokensToAdd)
			client.LastRefill = now
			client.HasChanged = true
		}
	}
}

func (sm *MapCache) SaveState(DB database.DataBase) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	err := DB.UpdateManyTokens(ctx, sm.cache)
	if err != nil {
		return err
	}
	for _, client := range sm.cache {
		client.HasChanged = false
	}
	return nil
}

func (sm *MapCache) RemoveStale(period time.Duration) {
	now := time.Now()
	for _, client := range sm.cache {
		if now.Sub(client.LastRefill) > period && !client.HasChanged {
			sm.DeleteClient(client.APIKey)
		}
	}
}
