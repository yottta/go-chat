package app

import (
	"context"
	"github.com/yottta/chat/directory/domain"
	"github.com/yottta/go-cache"
	"time"
)

type Clients interface {
	GetClients(ctx context.Context) ([]domain.Client, error)
	RegisterClient(ctx context.Context, client domain.Client) error
}

type clientsSvc struct {
	clients *cache.Cache[domain.Client]
}

func NewClientsSvc() Clients {
	return &clientsSvc{
		clients: cache.New[domain.Client](time.Second*30, time.Second*5, func() domain.Client { return domain.Client{} }),
	}
}

func (c *clientsSvc) GetClients(ctx context.Context) ([]domain.Client, error) {
	clients := c.clients.Items()
	res := make([]domain.Client, len(clients))
	var idx int
	for _, c := range clients {
		res[idx] = c.Object
		idx++
	}
	return res, nil
}

func (c *clientsSvc) RegisterClient(ctx context.Context, client domain.Client) error {
	c.clients.AddOrReplace(client.ID, client, cache.DefaultExpiration)
	return nil
}
