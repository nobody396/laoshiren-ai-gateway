package service

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/robfig/cron/v3"
)

// AgentLevelEvaluatorService runs the monthly agent level evaluation job.
type AgentLevelEvaluatorService struct {
	commission *CommissionService
	cfg        *config.Config

	mu   sync.Mutex
	cron *cron.Cron
}

func NewAgentLevelEvaluatorService(commission *CommissionService, cfg *config.Config) *AgentLevelEvaluatorService {
	return &AgentLevelEvaluatorService{commission: commission, cfg: cfg}
}

func (s *AgentLevelEvaluatorService) Start() {
	if s == nil || s.commission == nil || s.commission.levelRepo == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cron != nil {
		return
	}
	loc := time.Local
	if s.cfg != nil && strings.TrimSpace(s.cfg.Timezone) != "" {
		if parsed, err := time.LoadLocation(strings.TrimSpace(s.cfg.Timezone)); err == nil && parsed != nil {
			loc = parsed
		}
	}
	c := cron.New(cron.WithLocation(loc))
	if _, err := c.AddFunc(AgentLevelMonthlyEvaluationCron, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		result, err := s.commission.RunAllAgentLevelEvaluations(ctx, time.Now().In(loc))
		if err != nil {
			slog.Error("agent level monthly evaluation failed", "error", err)
			return
		}
		slog.Info("agent level monthly evaluation completed",
			"period", result.Period,
			"evaluated_count", result.EvaluatedCount,
			"failed_count", result.FailedCount,
		)
	}); err != nil {
		slog.Error("agent level monthly evaluator not started", "error", err)
		return
	}
	s.cron = c
	s.cron.Start()
	slog.Info("agent level monthly evaluator started")
}

func (s *AgentLevelEvaluatorService) Stop() {
	if s == nil {
		return
	}
	s.mu.Lock()
	c := s.cron
	s.cron = nil
	s.mu.Unlock()
	if c == nil {
		return
	}
	ctx := c.Stop()
	select {
	case <-ctx.Done():
	case <-time.After(10 * time.Second):
		slog.Warn("agent level monthly evaluator stop timed out")
	}
}
