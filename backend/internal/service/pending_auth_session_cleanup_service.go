package service

import (
	"context"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/logger"
)

type PendingAuthSessionCleanupService struct {
	identityService *IdentityService
	interval        time.Duration
	stopCh          chan struct{}
}

func NewPendingAuthSessionCleanupService(identityService *IdentityService, interval time.Duration) *PendingAuthSessionCleanupService {
	if interval <= 0 {
		interval = time.Hour
	}
	return &PendingAuthSessionCleanupService{
		identityService: identityService,
		interval:        interval,
		stopCh:          make(chan struct{}),
	}
}

func (s *PendingAuthSessionCleanupService) Start() {
	if s == nil || s.identityService == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if _, err := s.identityService.CleanupExpiredPendingSessions(context.Background(), time.Now().UTC()); err != nil {
					logger.LegacyPrintf("service.pending_auth_cleanup", "cleanup expired pending auth sessions failed: %v", err)
				}
			case <-s.stopCh:
				return
			}
		}
	}()
}

func (s *PendingAuthSessionCleanupService) Stop() {
	if s == nil {
		return
	}
	select {
	case <-s.stopCh:
	default:
		close(s.stopCh)
	}
}
