package models

import (
	"time"
)

// ClientConfig holds the rate limiting config for a client
type Client struct {
	ClientID   string    `json:"client_id"`
	APIKey     string    `json:"api_key"`
	Capacity   int       `json:"capacity"`
	RatePerSec int       `json:"rate_per_sec"`
	Tokens     int       `json:"tokens"`
	LastRefill time.Time `json:"last_refill"`
	HasChanged bool      `json:"-"`
}

func (c *Client) Copy(cl Client) {
	c.ClientID = cl.ClientID
	c.APIKey = cl.APIKey
	c.Capacity = cl.Capacity
	c.RatePerSec = cl.RatePerSec
	c.Tokens = cl.Tokens
	c.LastRefill = cl.LastRefill
	c.HasChanged = cl.HasChanged
}
