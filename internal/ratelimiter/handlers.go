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
	mux.HandleFunc("DELETE /clients", rl.DeleteClient)
	return mux
}

// rateLimitMiddleware wraps a handler with rate limiting
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

func (rl *RateLimiterBucket) GetClient(w http.ResponseWriter, r *http.Request) {
	clientID := r.PathValue("client_id")
	DBClient, err := rl.DB.GetClient(r.Context(), clientID)
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
	b := make([]byte, 32)
	_, err = rand.Read(b)
	if err != nil {
		apperrors.Error(w, r, err)
		return
	}
	APIKey := base64.URLEncoding.EncodeToString(b)
	client.APIKey = APIKey
	rl.Log.Debug("Created client", "client", client)
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
	// update database first
	DBClient, errDB := rl.DB.UpdateClient(r.Context(), client)
	if errDB != nil {
		apperrors.Error(w, r, errDB)
		return
	}
	cacheClient, errCache := rl.cache.UpdateClient(client)
	if errCache != nil {
		// return fresh state from cache
		err = json.NewEncoder(w).Encode(cacheClient)
		if err != nil {
			apperrors.Error(w, r, err)
		}
	}
	err = json.NewEncoder(w).Encode(DBClient)
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
	// delete from database first
	errDB := rl.DB.DeleteClient(r.Context(), client.ClientID)
	if errDB != nil {
		apperrors.Error(w, r, err)
		return
	}
	rl.cache.DeleteClient(client.APIKey)
	w.WriteHeader(http.StatusNoContent)
}
