//go:build integration

package repository

import (
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type SSOTicketCacheSuite struct {
	IntegrationRedisSuite
	cache service.SSOTicketCache
}

func (s *SSOTicketCacheSuite) SetupTest() {
	s.IntegrationRedisSuite.SetupTest()
	s.cache = NewSSOTicketCache(s.rdb)
}

func (s *SSOTicketCacheSuite) TestTicketIsTTLBoundAndConsumedExactlyOnce() {
	data := service.SSOTicketData{
		Purpose: "embed", UserID: 42, Audience: "https://consumer.example",
		TargetKind: "custom_menu", TargetID: "reports", Delivery: "iframe", CreatedAt: time.Now().UTC(),
	}
	require.NoError(s.T(), s.cache.StoreSSOTicket(s.ctx, "ticket", data, 60*time.Second))
	ttl, err := s.rdb.TTL(s.ctx, ssoTicketKey("ticket")).Result()
	require.NoError(s.T(), err)
	s.AssertTTLWithin(ttl, time.Second, 60*time.Second)

	consumed, err := s.cache.ConsumeSSOTicket(s.ctx, "ticket")
	require.NoError(s.T(), err)
	require.Equal(s.T(), data.Purpose, consumed.Purpose)
	require.Equal(s.T(), data.Audience, consumed.Audience)
	_, err = s.cache.ConsumeSSOTicket(s.ctx, "ticket")
	require.ErrorIs(s.T(), err, service.ErrSSOTicketNotFound)
}

func TestSSOTicketCacheSuite(t *testing.T) {
	suite.Run(t, new(SSOTicketCacheSuite))
}
