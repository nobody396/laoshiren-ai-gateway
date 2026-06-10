package service

import (
	"context"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionRedeemGroupIDsNormalizesGroups(t *testing.T) {
	legacyGroupID := int64(9)

	require.Equal(t, []int64{8, 11}, subscriptionRedeemGroupIDs(&RedeemCode{
		GroupID:  &legacyGroupID,
		GroupIDs: []int64{8, 11, 8, 0, -1},
	}))
	require.Equal(t, []int64{9}, subscriptionRedeemGroupIDs(&RedeemCode{
		GroupID: &legacyGroupID,
	}))
	require.Empty(t, subscriptionRedeemGroupIDs(&RedeemCode{
		GroupIDs: []int64{0, -1},
	}))
}

func TestSubscriptionRedeemNotesMarksSharedQuotaForMultiGroupCodes(t *testing.T) {
	require.Equal(t, "通过兑换码 SINGLE 兑换", subscriptionRedeemNotes("SINGLE", false))

	notes := subscriptionRedeemNotes("BUNDLE-GPT-CLAUDE", true)
	require.Contains(t, notes, "通过兑换码 BUNDLE-GPT-CLAUDE 兑换")
	require.Contains(t, notes, "shared_quota=redeem:BUNDLE-GPT-CLAUDE")
	require.Equal(t, "redeem:BUNDLE-GPT-CLAUDE", SubscriptionSharedQuotaMarkerFromNotes(notes))
	require.Equal(t, "redeem:OLD-BUNDLE", SubscriptionSharedQuotaMarkerFromNotes("通过兑换码 OLD-BUNDLE 兑换"))
}

func TestRedeemServiceCreateCodePersistsSubscriptionGroupIDs(t *testing.T) {
	repo := &redeemCreateRepoCapture{}
	svc := NewRedeemService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	code := &RedeemCode{
		Code:         "BUNDLE-GPT-CLAUDE",
		Type:         RedeemTypeSubscription,
		Value:        499,
		GroupIDs:     []int64{8, 11, 8},
		ValidityDays: 30,
	}
	err := svc.CreateCode(context.Background(), code)

	require.NoError(t, err)
	require.NotNil(t, repo.created)
	require.NotNil(t, repo.created.GroupID)
	require.Equal(t, int64(8), *repo.created.GroupID)
	require.Equal(t, []int64{8, 11}, repo.created.GroupIDs)
}

type redeemCreateRepoCapture struct {
	created *RedeemCode
}

func (r *redeemCreateRepoCapture) Create(_ context.Context, code *RedeemCode) error {
	if code == nil {
		r.created = nil
		return nil
	}
	cp := *code
	cp.GroupIDs = append([]int64(nil), code.GroupIDs...)
	r.created = &cp
	return nil
}

func (r *redeemCreateRepoCapture) CreateBatch(context.Context, []RedeemCode) error {
	panic("unexpected CreateBatch call")
}

func (r *redeemCreateRepoCapture) GetByID(context.Context, int64) (*RedeemCode, error) {
	panic("unexpected GetByID call")
}

func (r *redeemCreateRepoCapture) GetByCode(context.Context, string) (*RedeemCode, error) {
	panic("unexpected GetByCode call")
}

func (r *redeemCreateRepoCapture) Update(context.Context, *RedeemCode) error {
	panic("unexpected Update call")
}

func (r *redeemCreateRepoCapture) Delete(context.Context, int64) error {
	panic("unexpected Delete call")
}

func (r *redeemCreateRepoCapture) Use(context.Context, int64, int64) error {
	panic("unexpected Use call")
}

func (r *redeemCreateRepoCapture) List(context.Context, pagination.PaginationParams) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}

func (r *redeemCreateRepoCapture) ListWithFilters(context.Context, pagination.PaginationParams, string, string, string) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}

func (r *redeemCreateRepoCapture) ListByUser(context.Context, int64, int) ([]RedeemCode, error) {
	panic("unexpected ListByUser call")
}

func (r *redeemCreateRepoCapture) ListByUserPaginated(context.Context, int64, pagination.PaginationParams, string) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected ListByUserPaginated call")
}

func (r *redeemCreateRepoCapture) SumPositiveBalanceByUser(context.Context, int64) (float64, error) {
	panic("unexpected SumPositiveBalanceByUser call")
}
