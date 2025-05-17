package app

import (
	"context"
	"net/http"
	"runtime/debug"
	"sync/atomic"

	"github.com/SlayerSv/load-balancer/internal/apperrors"
	"github.com/SlayerSv/load-balancer/internal/config"
	"github.com/SlayerSv/load-balancer/internal/database"
	"github.com/SlayerSv/load-balancer/internal/loadbalancer"
	"github.com/SlayerSv/load-balancer/internal/logger"
	"github.com/SlayerSv/load-balancer/internal/ratelimiter"
)

type App struct {
	Cfg           *config.Config
	DB            database.DataBase
	LB            *loadbalancer.LoadBalancer
	RL            ratelimiter.RateLimiter
	Log           logger.Logger
	nextRequestID atomic.Int64
}

type IDString string

const RequestID IDString = "request_id"

func NewApp(cfg *config.Config, DB database.DataBase, LB *loadbalancer.LoadBalancer, RL ratelimiter.RateLimiter, log logger.Logger) *App {
	return &App{
		Cfg: cfg,
		DB:  DB,
		LB:  LB,
		RL:  RL,
		Log: log,
	}
}

func (app *App) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			err := recover()
			if err != nil {
				apperrors.Error(w, r, apperrors.ErrInternal)
				app.Log.Error("Panic occured", "error", err, "debug_trace", debug.Stack())
			}
		}()
		ID := app.nextRequestID.Add(1)
		ctx := context.WithValue(r.Context(), RequestID, ID)
		r = r.Clone(ctx)
		app.Log.Info("Incoming request", "method", "request_id", ID, r.Method, "path", r.URL.Path, "remote_address", r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}
