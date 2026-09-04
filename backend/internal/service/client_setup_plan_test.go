package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type planKeys struct {
	clientSetupAPIKeysStub
	groups []Group
}

func (k *planKeys) GetAvailableGroups(context.Context, int64) ([]Group, error) { return k.groups, nil }

type planDiscovery struct {
	clientSetupModelsStub
	models map[int64][]string
}

func (d *planDiscovery) MultiGroupCatalog(_ context.Context, g *Group) (*GroupModelDeclaration, error) {
	return &GroupModelDeclaration{Models: d.models[g.ID]}, nil
}
func planFixture() (*ClientSetupService, *planKeys, *planDiscovery) {
	k := &planKeys{clientSetupAPIKeysStub: clientSetupAPIKeysStub{keys: map[int64]*APIKey{1: {ID: 1, UserID: 2, Key: "fixture-only", Status: StatusActive, GroupIDs: []int64{6, 5}}}}, groups: []Group{{ID: 6, Platform: PlatformOpenAI, Status: StatusActive}, {ID: 5, Platform: PlatformAnthropic, Status: StatusActive}}}
	d := &planDiscovery{models: map[int64][]string{6: {"gpt-5.4", "gpt-5.5", "not-verified", "*"}, 5: {"claude-opus-5"}}}
	return &ClientSetupService{apiKeys: k, models: d, tickets: newClientSetupTicketCacheStub()}, k, d
}
func findPlan(t *testing.T, plans []ClientSetupPlan, id string) ClientSetupPlan {
	t.Helper()
	for _, p := range plans {
		if p.ClientID == id {
			return p
		}
	}
	t.Fatal("missing plan", id)
	return ClientSetupPlan{}
}
func TestSetupPlansIntersectAllAuthorizedGroups(t *testing.T) {
	s, _, _ := planFixture()
	ctx := context.Background()
	plans, err := s.SetupPlans(ctx, 2, 1, "macos")
	require.NoError(t, err)
	require.Len(t, plans, len(generatedClientSetupContracts))
	codex := findPlan(t, plans, "codex")
	require.True(t, codex.Available)
	require.Equal(t, []ClientSetupPlanModel{{ID: "gpt-5.4", Protocol: "responses"}, {ID: "gpt-5.5", Protocol: "responses"}}, codex.Models)
	claude := findPlan(t, plans, "claude-code")
	require.True(t, claude.Available)
	require.Equal(t, "claude-opus-5", claude.DefaultModel)
	require.False(t, findPlan(t, plans, "qoder").Available)
	require.False(t, findPlan(t, plans, "kimi-code").Available, "documentation is not installer acceptance")
}
func TestSetupPlanTicketRechecksThenConsumesOnce(t *testing.T) {
	s, _, _ := planFixture()
	ctx := context.Background()
	plans, err := s.SetupPlans(ctx, 2, 1, "linux")
	require.NoError(t, err)
	p := findPlan(t, plans, "codex")
	ticket, err := s.IssueTicketForPlan(ctx, 2, 1, p.ClientID, p.OS, p.Fingerprint)
	require.NoError(t, err)
	result, err := s.ExchangeTicket(ctx, ticket.Ticket)
	require.NoError(t, err)
	require.Equal(t, p.Models, result.Plan.Models)
	require.Equal(t, "fixture-only", result.APIKey)
	_, err = s.ExchangeTicket(ctx, ticket.Ticket)
	require.Error(t, err)
}
func TestSetupPlanTicketRejectsDriftAndCrossSelection(t *testing.T) {
	for _, change := range []string{"catalog", "permission", "disabled", "expired", "reorder"} {
		t.Run(change, func(t *testing.T) {
			s, k, d := planFixture()
			ctx := context.Background()
			plans, _ := s.SetupPlans(ctx, 2, 1, "macos")
			p := findPlan(t, plans, "codex")
			_, err := s.IssueTicketForPlan(ctx, 2, 1, "claude-code", p.OS, p.Fingerprint)
			require.Error(t, err)
			_, err = s.IssueTicketForPlan(ctx, 2, 1, "codex", "windows", p.Fingerprint)
			require.Error(t, err)
			ticket, err := s.IssueTicketForPlan(ctx, 2, 1, p.ClientID, p.OS, p.Fingerprint)
			require.NoError(t, err)
			switch change {
			case "catalog":
				d.models[6] = []string{"gpt-5.4"}
			case "permission":
				k.groups = k.groups[:1]
			case "disabled":
				k.keys[1].Status = StatusAPIKeyDisabled
			case "expired":
				past := time.Now().Add(-time.Hour)
				k.keys[1].ExpiresAt = &past
			case "reorder":
				k.keys[1].GroupIDs = []int64{5, 6}
			}
			_, err = s.ExchangeTicket(ctx, ticket.Ticket)
			require.Error(t, err)
		})
	}
}
func TestSetupPlansDenyOtherOwnerAndInvalidOS(t *testing.T) {
	s, _, _ := planFixture()
	for _, row := range []struct {
		user int64
		os   string
	}{{3, "macos"}, {2, "plan9"}} {
		_, err := s.SetupPlans(context.Background(), row.user, 1, row.os)
		require.Error(t, err)
	}
}
func TestSetupPlansSingleGroupStillSupported(t *testing.T) {
	s, k, _ := planFixture()
	k.keys[1].GroupIDs = nil
	k.keys[1].Group = &k.groups[0]
	plans, err := s.SetupPlans(context.Background(), 2, 1, "windows")
	require.NoError(t, err)
	require.True(t, findPlan(t, plans, "codex").Available)
	require.False(t, findPlan(t, plans, "claude-code").Available)
}
