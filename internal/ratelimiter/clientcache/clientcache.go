package clientcache

import (
	"github.com/SlayerSv/load-balancer/internal/database"
	"github.com/SlayerSv/load-balancer/internal/models"
)

// ClientCache interface for rate limiter
type ClientCache interface {
	GetClient(APIKey string) (models.Client, error)
	AddClient(client models.Client) error
	UpdateClient(client models.Client) (models.Client, error)
	DeleteClient(APIKey string) error
	AllowRequest(APIKey string) error
	AddTokensToAll()
	SaveState(DB database.DataBase)
	RemoveStale()
}
