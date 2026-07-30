package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type affiliateSelfCommissionPolicyRepoStub struct {
	agentID          int64
	enabled          bool
	expectedRevision int64
	reason           string
	operatorID       int64
	result           *AffiliateSelfCommissionPolicy
	err              error
}

func (r *affiliateSelfCommissionPolicyRepoStub) SetAffiliateSelfCommissionPolicy(
	_ context.Context,
	agentID int64,
	enabled bool,
	expectedRevision int64,
	reason string,
	operatorID int64,
) (*AffiliateSelfCommissionPolicy, error) {
	r.agentID = agentID
	r.enabled = enabled
	r.expectedRevision = expectedRevision
	r.reason = reason
	r.operatorID = operatorID
	return r.result, r.err
}

func TestAffiliateSelfCommissionPolicyServiceValidatesAndNormalizes(t *testing.T) {
	repo := &affiliateSelfCommissionPolicyRepoStub{
		result: &AffiliateSelfCommissionPolicy{
			AgentID:  7,
			Enabled:  true,
			RateBPS:  AffiliateSelfCommissionRateBPS,
			Revision: 1,
		},
	}
	svc := NewAffiliateSelfCommissionPolicyService(repo)

	result, err := svc.Update(
		context.Background(),
		7,
		true,
		0,
		"  运营审核通过  ",
		99,
	)
	require.NoError(t, err)
	require.True(t, result.Enabled)
	require.Equal(t, int64(7), repo.agentID)
	require.True(t, repo.enabled)
	require.Zero(t, repo.expectedRevision)
	require.Equal(t, "运营审核通过", repo.reason)
	require.Equal(t, int64(99), repo.operatorID)

	_, err = svc.Update(context.Background(), 0, true, 0, "x", 99)
	require.ErrorIs(t, err, ErrInvalidInput)
	_, err = svc.Update(context.Background(), 7, true, -1, "x", 99)
	require.ErrorIs(t, err, ErrInvalidInput)
	_, err = svc.Update(context.Background(), 7, true, 0, " ", 99)
	require.ErrorIs(t, err, ErrInvalidInput)
	_, err = svc.Update(context.Background(), 7, true, 0, string(make([]rune, 501)), 99)
	require.ErrorIs(t, err, ErrInvalidInput)
}
