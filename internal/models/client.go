package models

import (
	"sync"
	"time"
)

// Client represents a client for rate limiting service
type Client struct {
	ClientID string `json:"client_id"`
	APIKey   string `json:"api_key"`
	// Capacity is the maximum number of tokens client can have
	Capacity int `json:"capacity"`
	// RatePerSec is the amount of tokens added to client each second
	RatePerSec int `json:"rate_per_sec"`
	// Tokens is the current number of tokens client has
	Tokens int `json:"tokens"`
	// last time client tokens were replenished
	LastRefill time.Time `json:"last_refill"`
}

// ClientCache is the object for rate limiter cache
type ClientCache struct {
	Client
	// HasChanged reports whether state of this client has changed from last database synchronization
	HasChanged bool
	// Expires is the time at which this client will be removed from cache if it will not be used
	Expires time.Time
	Mu      sync.RWMutex
}

// Copy copies all fields of Client to ClientCache, sets HasChanged to false.
func (c *ClientCache) Copy(cl Client) {
	c.ClientID = cl.ClientID
	c.APIKey = cl.APIKey
	c.Capacity = cl.Capacity
	c.RatePerSec = cl.RatePerSec
	c.Tokens = cl.Tokens
	c.LastRefill = cl.LastRefill
	c.HasChanged = false
}
