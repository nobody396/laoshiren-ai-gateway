package service

import (
	"context"
	"sync"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/logger"
)

// affiliateActivationSweepHour is the daily sweep hour in Asia/Shanghai.
// Qualified users who never open the partner page still get activated within
// 24 hours, complete with the in-app notice and activation email.
const affiliateActivationSweepHour = 12

// AffiliateAgentActivationScheduler activates qualified-but-unactivated
// partners once a day at 12:00 Asia/Shanghai. Beijing has no DST, so a fixed
// local wall-clock time is stable; the location keeps the intent explicit.
type AffiliateAgentActivationScheduler struct {
	agents *AffiliateAgentService
	loc    *time.Location
	now    func() time.Time // injectable clock for tests

	startOnce sync.Once
	stopOnce  sync.Once
	stopCh    chan struct{}
	doneCh    chan struct{}

	mu      sync.Mutex
	lastRun time.Time
}

func NewAffiliateAgentActivationScheduler(agents *AffiliateAgentService) *AffiliateAgentActivationScheduler {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	return &AffiliateAgentActivationScheduler{
		agents: agents,
		loc:    loc,
		now:    time.Now,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
}

func (s *AffiliateAgentActivationScheduler) Start() {
	if s == nil || s.agents == nil {
		return
	}
	s.startOnce.Do(func() {
		go s.runLoop()
		logger.LegacyPrintf("service.affiliate_activation", "[AffiliateActivation] Daily sweep scheduled at %02d:00 %s", affiliateActivationSweepHour, s.loc)
	})
}

func (s *AffiliateAgentActivationScheduler) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
		<-s.doneCh
	})
}

func (s *AffiliateAgentActivationScheduler) runLoop() {
	defer close(s.doneCh)
	for {
		delay, catchUp := s.nextDelay()
		if catchUp {
			s.RunOnce(context.Background())
			continue
		}
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
			s.RunOnce(context.Background())
		case <-s.stopCh:
			timer.Stop()
			return
		}
	}
}

// nextDelay computes the wait until the next daily run. When the scheduled
// time already passed today and no run has happened yet (e.g. the process was
// down at noon), it requests an immediate catch-up run instead.
func (s *AffiliateAgentActivationScheduler) nextDelay() (time.Duration, bool) {
	now := s.now().In(s.loc)
	todayRun := time.Date(now.Year(), now.Month(), now.Day(), affiliateActivationSweepHour, 0, 0, 0, s.loc)
	if now.Before(todayRun) {
		return todayRun.Sub(now), false
	}
	s.mu.Lock()
	lastRun := s.lastRun
	s.mu.Unlock()
	if lastRun.IsZero() || !sameAffiliateSweepDay(lastRun.In(s.loc), now) {
		return 0, true
	}
	return todayRun.AddDate(0, 0, 1).Sub(now), false
}

func sameAffiliateSweepDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// RunOnce executes one sweep: list qualified candidates and activate each in
// its own transaction. One failing user never blocks the rest.
func (s *AffiliateAgentActivationScheduler) RunOnce(ctx context.Context) {
	scanned, activated, failed, err := s.agents.ActivateQualifiedCandidates(ctx, 500)
	if err != nil {
		logger.LegacyPrintf("service.affiliate_activation", "[AffiliateActivation] Daily sweep failed to list candidates: %v", err)
		return
	}
	s.mu.Lock()
	s.lastRun = s.now()
	s.mu.Unlock()
	logger.LegacyPrintf(
		"service.affiliate_activation",
		"[AffiliateActivation] Daily sweep done: scanned=%d activated=%d failures=%d",
		scanned, activated, failed,
	)
}
