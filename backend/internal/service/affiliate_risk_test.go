package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type affiliateRiskRepoStub struct {
	setStatus string
	setReason string
	reversal  int64
}

func (r *affiliateRiskRepoStub) ListAffiliateRiskPrincipals(
	context.Context,
	int,
) ([]AffiliateRiskPrincipal, error) {
	return []AffiliateRiskPrincipal{{AgentID: 7}}, nil
}

func (r *affiliateRiskRepoStub) SetAffiliateAgentRisk(
	_ context.Context,
	agentID int64,
	nextStatus string,
	reason string,
	_ int64,
) (*AffiliateRiskActionResult, error) {
	r.setStatus = nextStatus
	r.setReason = reason
	return &AffiliateRiskActionResult{
		AgentID:             agentID,
		NextRiskStatus:      nextStatus,
		AffectedCreditUsers: []int64{9},
	}, nil
}

func (r *affiliateRiskRepoStub) ReverseAffiliatePerformanceEvent(
	_ context.Context,
	eventID int64,
	reason string,
	operatorID int64,
) (*AffiliatePerformanceReversal, error) {
	r.reversal = eventID
	return &AffiliatePerformanceReversal{
		OriginalEventID: eventID,
		Reason:          reason,
		OperatorID:      operatorID,
	}, nil
}

func TestAffiliateRiskServiceValidatesStatusAndReasons(t *testing.T) {
	repo := &affiliateRiskRepoStub{}
	svc := NewAffiliateRiskService(repo)

	result, err := svc.SetAgentRisk(context.Background(), 7, " REVIEW ", " 风控复核 ", 99)
	require.NoError(t, err)
	require.Equal(t, AffiliateRiskStatusReview, result.NextRiskStatus)
	require.Equal(t, AffiliateRiskStatusReview, repo.setStatus)
	require.Equal(t, "风控复核", repo.setReason)

	_, err = svc.SetAgentRisk(context.Background(), 7, "invalid", "x", 99)
	require.ErrorIs(t, err, ErrInvalidInput)
	_, err = svc.SetAgentRisk(context.Background(), 7, AffiliateRiskStatusBlocked, " ", 99)
	require.ErrorIs(t, err, ErrInvalidInput)
}

func TestAffiliateRiskServiceValidatesReversal(t *testing.T) {
	repo := &affiliateRiskRepoStub{}
	svc := NewAffiliateRiskService(repo)

	result, err := svc.ReversePerformanceEvent(context.Background(), 42, "退款冲正", 99)
	require.NoError(t, err)
	require.Equal(t, int64(42), result.OriginalEventID)
	require.Equal(t, int64(42), repo.reversal)

	_, err = svc.ReversePerformanceEvent(context.Background(), 0, "退款", 99)
	require.ErrorIs(t, err, ErrInvalidInput)
	_, err = svc.ReversePerformanceEvent(context.Background(), 42, "", 99)
	require.ErrorIs(t, err, ErrInvalidInput)
}
