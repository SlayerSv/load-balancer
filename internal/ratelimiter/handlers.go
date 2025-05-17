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
		allowed, err := rl.AllowRequest(apiKey)
		if err != nil {
			apperrors.Error(w, r, err)
			return
		}
		if !allowed {
			apperrors.Error(w, r, apperrors.ErrRateLimitExceeded)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiterBucket) GetClient(w http.ResponseWriter, r *http.Request) {
	var client models.Client
	err := json.NewDecoder(r.Body).Decode(&client)
	if err != nil {
		apperrors.Error(w, r, err)
		return
	}
	exxistingClient, err := rl.DB.GetClient(r.Context(), client.APIKey)
	if err != nil {
		apperrors.Error(w, r, err)
		return
	}
	err = json.NewEncoder(w).Encode(exxistingClient)
	if err != nil {
		apperrors.Error(w, r, err)
	}
}

func (rl *RateLimiterBucket) AddClient(w http.ResponseWriter, r *http.Request) {
	var client models.Client
	err := json.NewDecoder(r.Body).Decode(&client)
	if err != nil {
		apperrors.Error(w, r, err)
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
		apperrors.Error(w, r, err)
		return
	}
	updatedClient, err := rl.DB.UpdateClient(r.Context(), client)
	if err != nil {
		apperrors.Error(w, r, err)
		return
	}
	err = json.NewEncoder(w).Encode(updatedClient)
	if err != nil {
		apperrors.Error(w, r, err)
	}
}

func (rl *RateLimiterBucket) DeleteClient(w http.ResponseWriter, r *http.Request) {
	var client models.Client
	err := json.NewDecoder(r.Body).Decode(&client)
	if err != nil {
		apperrors.Error(w, r, err)
		return
	}
	deletedClient, err := rl.DB.DeleteClient(r.Context(), client.ClientID)
	if err != nil {
		apperrors.Error(w, r, err)
		return
	}
	err = json.NewEncoder(w).Encode(deletedClient)
	if err != nil {
		apperrors.Error(w, r, err)
	}
}
