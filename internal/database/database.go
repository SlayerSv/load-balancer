package database

import (
	"context"

	"github.com/SlayerSv/load-balancer/internal/models"
)

// DataBase is the interface for database implementations
type DataBase interface {
	GetClient(ctx context.Context, clientID string) (models.Client, error)
	GetClientByAPIKey(ctx context.Context, APIKey string) (models.Client, error)
	AddClient(ctx context.Context, client models.Client) (models.Client, error)
	// UpdateClient only updates rate_per_sec and capacity fields of a client
	UpdateClient(ctx context.Context, client models.Client) (models.Client, error)
	DeleteClient(ctx context.Context, clientID string) (models.Client, error)

	UpdateTokens(ctx context.Context, client models.Client) (models.Client, error)
}
