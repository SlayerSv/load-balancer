package ratelimiter

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/SlayerSv/load-balancer/internal/apperrors"
	"github.com/SlayerSv/load-balancer/internal/models"
)

func (rl *RateLimiterBucket) NewClientHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /clients", rl.GetClient)
	mux.HandleFunc("POST /clients", rl.AddClient)
	mux.HandleFunc("PATCH /clients", rl.UpdateClient)
	mux.HandleFunc("DELETE /clients", rl.DeleteClient)
	return mux
}

// rateLimitMiddleware wraps a handler with rate limiting
func (rl *RateLimiterBucket) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/clients" {
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

func (rl *RateLimiterBucket) GetClient(w http.ResponseWriter, r *http.Request) {
	var client models.Client
	err := json.NewDecoder(r.Body).Decode(&client)
	if err != nil {
		apperrors.Error(w, r, fmt.Errorf("%w: invalid request body", apperrors.ErrBadRequest))
		return
	}
	cacheClient, err := rl.cache.GetClient(client.APIKey)
	if err == nil {
		err = json.NewEncoder(w).Encode(cacheClient)
		if err != nil {
			apperrors.Error(w, r, err)
		}
		return
	}
	DBClient, err := rl.DB.GetClient(r.Context(), client.APIKey)
	if err != nil {
		apperrors.Error(w, r, err)
		return
	}
	err = json.NewEncoder(w).Encode(DBClient)
	if err != nil {
		apperrors.Error(w, r, err)
	}
}

func (rl *RateLimiterBucket) AddClient(w http.ResponseWriter, r *http.Request) {
	var client models.Client
	err := json.NewDecoder(r.Body).Decode(&client)
	if err != nil {
		apperrors.Error(w, r, fmt.Errorf("%w: invalid request body", apperrors.ErrBadRequest))
		return
	}
	newClient, err := rl.DB.AddClient(r.Context(), client)
	if err != nil {
		apperrors.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
	err = json.NewEncoder(w).Encode(newClient)
	if err != nil {
		apperrors.Error(w, r, err)
	}
}

func (rl *RateLimiterBucket) UpdateClient(w http.ResponseWriter, r *http.Request) {
	var client models.Client
	err := json.NewDecoder(r.Body).Decode(&client)
	if err != nil {
		apperrors.Error(w, r, fmt.Errorf("%w: invalid request body", apperrors.ErrBadRequest))
		return
	}
	updClient, err := rl.cache.UpdateClient(client)
	if err != nil {
		apperrors.Error(w, r, err)
		return
	}
	err = json.NewEncoder(w).Encode(updClient)
	if err != nil {
		apperrors.Error(w, r, err)
	}
}

func (rl *RateLimiterBucket) DeleteClient(w http.ResponseWriter, r *http.Request) {
	var client models.Client
	err := json.NewDecoder(r.Body).Decode(&client)
	if err != nil {
		apperrors.Error(w, r, fmt.Errorf("%w: invalid request body", apperrors.ErrBadRequest))
		return
	}
	err = rl.cache.DeleteClient(client.APIKey)
	if err != nil {
		apperrors.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
