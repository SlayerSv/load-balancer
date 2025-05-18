package ratelimiter

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/SlayerSv/load-balancer/internal/apperrors"
	"github.com/SlayerSv/load-balancer/internal/models"
)

func (rl *RateLimiterBucket) NewClientHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /clients/{client_id}", rl.GetClient)
	mux.HandleFunc("POST /clients", rl.AddClient)
	mux.HandleFunc("PATCH /clients", rl.UpdateClient)
	mux.HandleFunc("DELETE /clients/{client_id}", rl.DeleteClient)
	return mux
}

// rateLimitMiddleware wraps a handler with rate limiting service
// rerouts /clients requests to its own handler, for others checks
// clients for api keys and tokens to allow access
func (rl *RateLimiterBucket) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/clients") {
			rl.clientHandler.ServeHTTP(w, r)
			return
		}
		apiKey := r.Header.Get("Authorization")
		if apiKey == "" {
			apperrors.Error(w, r, fmt.Errorf("%w: missing API key", apperrors.ErrUnauthorized))
			return
		}
		err := rl.AllowRequest(apiKey)
		if err != nil {
			apperrors.Error(w, r, err)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// GetClient return a client from database (client state may lag behind cache state)
func (rl *RateLimiterBucket) GetClient(w http.ResponseWriter, r *http.Request) {
	clientID := r.PathValue("client_id")
	if clientID == "" {
		apperrors.Error(w, r, fmt.Errorf("%w: missing client id value in path", apperrors.ErrBadRequest))
	}
	DBClient, err := rl.DB.GetClient(r.Context(), clientID)
	if err != nil {
		apperrors.Error(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(DBClient)
	if err != nil {
		apperrors.Error(w, r, err)
	}
}

// AddClient add a client to database, generates a new api key and returns
// created client as json.
func (rl *RateLimiterBucket) AddClient(w http.ResponseWriter, r *http.Request) {
	var client models.Client
	err := json.NewDecoder(r.Body).Decode(&client)
	if err != nil {
		apperrors.Error(w, r, fmt.Errorf("%w: invalid request body", apperrors.ErrBadRequest))
		return
	}
	b := make([]byte, 32)
	_, err = rand.Read(b)
	if err != nil {
		apperrors.Error(w, r, err)
		return
	}
	APIKey := base64.URLEncoding.EncodeToString(b)
	client.APIKey = APIKey
	if client.RatePerSec < 1 {
		client.RatePerSec = rl.Cfg.DefaultRatePerSec
	}
	if client.Capacity < 1 {
		client.Capacity = rl.Cfg.DefaultCapacity
	}
	newClient, err := rl.DB.AddClient(r.Context(), client)
	if err != nil {
		apperrors.Error(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	err = json.NewEncoder(w).Encode(newClient)
	if err != nil {
		apperrors.Error(w, r, err)
	}
}

// UpdateClient updates client rate per sec and capacity both in database and in cache.
// returns updated client as a json response.
func (rl *RateLimiterBucket) UpdateClient(w http.ResponseWriter, r *http.Request) {
	var client models.Client
	err := json.NewDecoder(r.Body).Decode(&client)
	if err != nil {
		apperrors.Error(w, r, fmt.Errorf("%w: invalid request body", apperrors.ErrBadRequest))
		return
	}
	// update database first
	DBClient, errDB := rl.DB.UpdateClient(r.Context(), client)
	if errDB != nil {
		apperrors.Error(w, r, errDB)
		return
	}
	// update cache
	cacheClient, errCache := rl.cache.UpdateClient(client)
	if errCache == nil {
		// return fresh state from cache
		err = json.NewEncoder(w).Encode(cacheClient)
		if err != nil {
			apperrors.Error(w, r, err)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(DBClient)
	if err != nil {
		apperrors.Error(w, r, err)
	}
}

// DeleteClient Deletes client from database and cache.
func (rl *RateLimiterBucket) DeleteClient(w http.ResponseWriter, r *http.Request) {
	clientID := r.PathValue("client_id")
	if clientID == "" {
		apperrors.Error(w, r, fmt.Errorf("%w: missing client id value in path", apperrors.ErrBadRequest))
	}
	// delete from database first
	deletedClient, err := rl.DB.DeleteClient(r.Context(), clientID)
	if err != nil {
		apperrors.Error(w, r, err)
		return
	}
	rl.cache.DeleteClient(deletedClient.APIKey)
	w.WriteHeader(http.StatusNoContent)
}
