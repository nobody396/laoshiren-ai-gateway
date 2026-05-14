package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"

	"github.com/redis/go-redis/v9"
)

const ssoTicketKeyPrefix = "dragoncode:sso:ticket:"

type ssoTicketCache struct {
	rdb *redis.Client
}

func NewSSOTicketCache(rdb *redis.Client) service.SSOTicketCache {
	return &ssoTicketCache{rdb: rdb}
}

func ssoTicketKey(ticket string) string {
	return ssoTicketKeyPrefix + ticket
}

func (c *ssoTicketCache) StoreSSOTicket(ctx context.Context, ticket string, data service.SSOTicketData, ttl time.Duration) error {
	val, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal sso ticket data: %w", err)
	}
	return c.rdb.Set(ctx, ssoTicketKey(ticket), val, ttl).Err()
}

func (c *ssoTicketCache) ConsumeSSOTicket(ctx context.Context, ticket string) (*service.SSOTicketData, error) {
	val, err := c.rdb.GetDel(ctx, ssoTicketKey(ticket)).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, service.ErrSSOTicketNotFound
		}
		return nil, err
	}

	var data service.SSOTicketData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, fmt.Errorf("unmarshal sso ticket data: %w", err)
	}
	return &data, nil
}
