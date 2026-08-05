package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func newTestActivationScheduler(repo *affiliateAgentRepoStub, queue *activationEmailQueueStub) *AffiliateAgentActivationScheduler {
	agents := newActivationTestService(repo, queue)
	return NewAffiliateAgentActivationScheduler(agents)
}

func TestAffiliateActivationScheduler_NextDelayBeforeNoon(t *testing.T) {
	s := newTestActivationScheduler(&affiliateAgentRepoStub{}, &activationEmailQueueStub{})
	s.now = func() time.Time {
		return time.Date(2026, 8, 5, 9, 30, 0, 0, s.loc)
	}
	delay, catchUp := s.nextDelay()
	require.False(t, catchUp)
	require.Equal(t, 2*time.Hour+30*time.Minute, delay)
}

func TestAffiliateActivationScheduler_CatchUpAfterMissedNoon(t *testing.T) {
	s := newTestActivationScheduler(&affiliateAgentRepoStub{}, &activationEmailQueueStub{})
	s.now = func() time.Time {
		return time.Date(2026, 8, 5, 15, 0, 0, 0, s.loc)
	}
	delay, catchUp := s.nextDelay()
	require.True(t, catchUp, "a process that was down at noon must catch up immediately")
	require.Zero(t, delay)

	s.RunOnce(context.Background())

	delay, catchUp = s.nextDelay()
	require.False(t, catchUp, "after the catch-up run the same day must not run twice")
	require.Equal(t, 21*time.Hour, delay)
}

func TestAffiliateActivationScheduler_NextDelayUsesShanghaiWallClock(t *testing.T) {
	s := newTestActivationScheduler(&affiliateAgentRepoStub{}, &activationEmailQueueStub{})
	// 2026-08-05 03:30 UTC = 11:30 Asia/Shanghai: 30 minutes before the sweep.
	s.now = func() time.Time {
		return time.Date(2026, 8, 5, 3, 30, 0, 0, time.UTC)
	}
	delay, catchUp := s.nextDelay()
	require.False(t, catchUp)
	require.Equal(t, 30*time.Minute, delay)
}

func TestAffiliateAgentService_ActivateQualifiedCandidatesIsolatesFailures(t *testing.T) {
	repo := &affiliateAgentRepoStub{
		candidates: []AffiliateQualifiedCandidate{
			{UserID: 101},
			{UserID: 102},
			{UserID: 103},
			{UserID: 104},
		},
		autoResults: map[int64]*AffiliateAgentReviewResult{
			101: {NewlyActivated: true, Application: AffiliateAgentApplication{ID: 1, UserID: 101, Status: "approved"}},
			102: {NewlyActivated: false, Application: AffiliateAgentApplication{ID: 2, UserID: 102, Status: "approved"}},
		},
		autoErrors: map[int64]error{
			103: ErrAffiliateQualificationNotMet, // state flipped after listing: harmless skip
			104: errors.New("db exploded"),       // real failure: counted, must not block others
		},
	}
	queue := &activationEmailQueueStub{}
	svc := newActivationTestService(repo, queue)

	scanned, activated, failed, err := svc.ActivateQualifiedCandidates(context.Background(), 500)
	require.NoError(t, err)
	require.Equal(t, 4, scanned)
	require.Equal(t, 1, activated)
	require.Equal(t, 1, failed)
	require.Equal(t, []string{"partner@example.com"}, queue.emails, "only the freshly activated user gets the email")
}
