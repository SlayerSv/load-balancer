package clientcache

import (
	"time"

	"github.com/SlayerSv/load-balancer/internal/database"
	"github.com/SlayerSv/load-balancer/internal/models"
)

type ClientCache interface {
	GetClient(APIKey string) (*models.Client, error)
	AddClient(client models.Client) error
	// UpdateClient(client models.Client)
	DeleteClient(APIKey string) error
	AddTokensToAll()
	SaveState(DB database.DataBase) error
	RemoveStale(period time.Duration)
}
