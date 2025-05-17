package clientcache

import (
	"github.com/SlayerSv/load-balancer/internal/database"
	"github.com/SlayerSv/load-balancer/internal/models"
)

type ClientCache interface {
	GetClient(APIKey string) (models.Client, error)
	AddClient(client models.Client) error
	// UpdateClient(client models.Client)
	DeleteClient(APIKey string) error
	AllowRequest(APIKey string) (bool, error)
	AddTokensToAll()
	SaveState(DB database.DataBase)
	RemoveStale()
}
