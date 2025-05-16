package database

import (
	"context"

	"github.com/SlayerSv/load-balancer/internal/models"
)

type DataBase interface {
	GetClient(ctx context.Context, clientAPIKey string) (models.Client, error)
	AddClient(ctx context.Context, client models.Client) (models.Client, error)
	UpdateClient(ctx context.Context, client models.Client) (models.Client, error)
	DeleteClient(ctx context.Context, clientID string) (models.Client, error)

	UpdateTokens(ctx context.Context, client models.Client) (models.Client, error)
	UpdateManyTokens(ctx context.Context, client []models.Client) error
}
