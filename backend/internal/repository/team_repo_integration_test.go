//go:build integration

package repository

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	appmigrations "github.com/bozhouDev/DragonCode-sub2api/migrations"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestTeamAPIKeyGetByIDHydratesCurrentOwnerForAsyncSettlement(t *testing.T) {
	ensureTeamMigrations(t)
	ctx := context.Background()
	teamRepo := NewTeamRepository(integrationDB)
	owner := mustCreateUser(t, integrationEntClient, &service.User{Email: uniqueTeamTestEmail("async-owner"), Balance: 10})
	member := mustCreateUser(t, integrationEntClient, &service.User{Email: uniqueTeamTestEmail("async-member")})
	teamCtx, err := teamRepo.Create(ctx, "异步结算团队", owner.ID, 5)
	require.NoError(t, err)
	token := uuid.NewString()
	_, err = teamRepo.CreateInvitation(ctx, teamCtx.Team.ID, owner.ID, member.Email, token, time.Now().Add(time.Hour))
	require.NoError(t, err)
	_, err = teamRepo.ResolveInvitation(ctx, token, member.ID, member.Email, "accepted", time.Now())
	require.NoError(t, err)
	teamID := teamCtx.Team.ID
	key := mustCreateApiKey(t, integrationEntClient, &service.APIKey{UserID: member.ID, TeamID: &teamID, Key: "sk-team-async-" + uuid.NewString()})

	apiKeyService := service.NewAPIKeyService(
		NewAPIKeyRepository(integrationEntClient, integrationDB),
		NewUserRepository(integrationEntClient, integrationDB),
		nil, nil, nil, nil,
		&config.Config{Team: config.TeamConfig{Enabled: true}},
	)
	apiKeyService.SetTeamRepository(teamRepo)
	hydrated, err := apiKeyService.GetByID(ctx, key.ID)
	require.NoError(t, err)
	require.Equal(t, owner.ID, hydrated.User.ID)
	require.Equal(t, member.ID, hydrated.ActorUser.ID)

	transferToken := uuid.NewString()
	_, err = teamRepo.CreateOwnershipTransfer(ctx, teamID, owner.ID, member.ID, transferToken, time.Now().Add(time.Hour))
	require.NoError(t, err)
	_, err = teamRepo.ResolveOwnershipTransfer(ctx, transferToken, member.ID, "accepted", time.Now())
	require.NoError(t, err)
	require.NoError(t, teamRepo.Dissolve(ctx, teamID, time.Now()))

	historical, err := apiKeyService.GetByIDForHistoricalBilling(ctx, key.ID, owner.ID)
	require.NoError(t, err)
	require.Equal(t, owner.ID, historical.User.ID, "captured payer survives transfer and dissolution")
	require.Equal(t, member.ID, historical.ActorUser.ID)
	account := mustCreateAccount(t, integrationEntClient, &service.Account{Name: "team-async-account-" + uuid.NewString(), Type: service.AccountTypeAPIKey})
	usageRepo := newUsageLogRepositoryWithSQL(integrationEntClient, integrationDB)
	_, err = usageRepo.Create(ctx, &service.UsageLog{
		UserID: owner.ID, APIKeyID: key.ID, AccountID: account.ID,
		RequestID: uuid.NewString(), Model: "gpt-image-2", ImageCount: 1,
		TotalCost: 0.25, ActualCost: 0.25, CreatedAt: time.Now(),
	})
	require.NoError(t, err)
	var billingUserID, actorUserID, persistedTeamID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT billing_user_id, actor_user_id, team_id FROM usage_logs
		WHERE api_key_id=$1 ORDER BY id DESC LIMIT 1`, key.ID).
		Scan(&billingUserID, &actorUserID, &persistedTeamID))
	require.Equal(t, owner.ID, billingUserID)
	require.Equal(t, member.ID, actorUserID)
	require.Equal(t, teamID, persistedTeamID)
}

var (
	teamMigrationsOnce sync.Once
	teamMigrationsErr  error
)

func ensureTeamMigrations(t *testing.T) {
	t.Helper()
	teamMigrationsOnce.Do(func() {
		for _, name := range []string{"219_add_teams.sql", "220_add_team_default_member_limits.sql", "221_harden_team_lifecycle_and_allowance.sql", "223_freeze_team_async_billing_user.sql"} {
			var sqlBytes []byte
			sqlBytes, teamMigrationsErr = appmigrations.FS.ReadFile(name)
			if teamMigrationsErr != nil {
				return
			}
			_, teamMigrationsErr = integrationDB.Exec(string(sqlBytes))
			if teamMigrationsErr != nil {
				return
			}
		}
	})
	require.NoError(t, teamMigrationsErr)
}

func TestTeamInvitationConcurrentAcceptanceEnforcesMemberLimit(t *testing.T) {
	ensureTeamMigrations(t)
	ctx := context.Background()
	repo := NewTeamRepository(integrationDB)
	owner := mustCreateUser(t, integrationEntClient, &service.User{Email: uniqueTeamTestEmail("owner"), Balance: 10})
	first := mustCreateUser(t, integrationEntClient, &service.User{Email: uniqueTeamTestEmail("first")})
	second := mustCreateUser(t, integrationEntClient, &service.User{Email: uniqueTeamTestEmail("second")})
	teamCtx, err := repo.Create(ctx, "并发邀请团队", owner.ID, 1)
	require.NoError(t, err)

	firstToken := uuid.NewString()
	secondToken := uuid.NewString()
	_, err = repo.CreateInvitation(ctx, teamCtx.Team.ID, owner.ID, first.Email, firstToken, time.Now().Add(time.Hour))
	require.NoError(t, err)
	_, err = repo.CreateInvitation(ctx, teamCtx.Team.ID, owner.ID, second.Email, secondToken, time.Now().Add(time.Hour))
	require.NoError(t, err)

	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	accept := func(token string, user *service.User) {
		defer wg.Done()
		<-start
		_, acceptErr := repo.ResolveInvitation(ctx, token, user.ID, user.Email, "accepted", time.Now())
		results <- acceptErr
	}
	wg.Add(2)
	go accept(firstToken, first)
	go accept(secondToken, second)
	close(start)
	wg.Wait()
	close(results)

	var accepted, limited int
	for result := range results {
		switch {
		case result == nil:
			accepted++
		case errors.Is(result, service.ErrTeamMemberLimitReached):
			limited++
		default:
			require.NoError(t, result)
		}
	}
	require.Equal(t, 1, accepted)
	require.Equal(t, 1, limited)

	var memberCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM team_memberships WHERE team_id = $1 AND left_at IS NULL AND role = 'member'`, teamCtx.Team.ID).Scan(&memberCount))
	require.Equal(t, 1, memberCount)
}

func TestTeamInvitationMemberLimitZeroRejectsMembers(t *testing.T) {
	ensureTeamMigrations(t)
	ctx := context.Background()
	repo := NewTeamRepository(integrationDB)
	owner := mustCreateUser(t, integrationEntClient, &service.User{Email: uniqueTeamTestEmail("zero-owner")})
	member := mustCreateUser(t, integrationEntClient, &service.User{Email: uniqueTeamTestEmail("zero-member")})
	teamCtx, err := repo.Create(ctx, "零成员容量团队", owner.ID, 0)
	require.NoError(t, err)
	token := uuid.NewString()
	_, err = repo.CreateInvitation(ctx, teamCtx.Team.ID, owner.ID, member.Email, token, time.Now().Add(time.Hour))
	require.NoError(t, err)

	_, err = repo.ResolveInvitation(ctx, token, member.ID, member.Email, "accepted", time.Now())
	require.ErrorIs(t, err, service.ErrTeamMemberLimitReached)
}

func TestActiveTeamOwnerCannotBeDeleted(t *testing.T) {
	ensureTeamMigrations(t)
	ctx := context.Background()
	repo := NewTeamRepository(integrationDB)
	userRepo := NewUserRepository(integrationEntClient, integrationDB)
	owner := mustCreateUser(t, integrationEntClient, &service.User{Email: uniqueTeamTestEmail("protected-owner")})
	teamCtx, err := repo.Create(ctx, "删除保护团队", owner.ID, 10)
	require.NoError(t, err)

	err = userRepo.Delete(ctx, owner.ID)
	require.ErrorIs(t, err, service.ErrTeamOwnerTransferRequired)

	_, err = integrationDB.ExecContext(ctx, `UPDATE users SET deleted_at = NOW() WHERE id = $1`, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "TEAM_OWNER_TRANSFER_REQUIRED")

	require.NoError(t, repo.Dissolve(ctx, teamCtx.Team.ID, time.Now()))
	_, err = integrationDB.ExecContext(ctx, `UPDATE users SET deleted_at = NOW() WHERE id = $1`, owner.ID)
	require.NoError(t, err)
}

func TestSoftDeletedTeamMemberIsRemovedAndKeysDisabled(t *testing.T) {
	ensureTeamMigrations(t)
	ctx := context.Background()
	repo := NewTeamRepository(integrationDB)
	owner := mustCreateUser(t, integrationEntClient, &service.User{Email: uniqueTeamTestEmail("delete-owner")})
	member := mustCreateUser(t, integrationEntClient, &service.User{Email: uniqueTeamTestEmail("delete-member")})
	teamCtx, err := repo.Create(ctx, "成员删除清理团队", owner.ID, 5)
	require.NoError(t, err)
	token := uuid.NewString()
	_, err = repo.CreateInvitation(ctx, teamCtx.Team.ID, owner.ID, member.Email, token, time.Now().Add(time.Hour))
	require.NoError(t, err)
	_, err = repo.ResolveInvitation(ctx, token, member.ID, member.Email, "accepted", time.Now())
	require.NoError(t, err)
	teamID := teamCtx.Team.ID
	apiKey := mustCreateApiKey(t, integrationEntClient, &service.APIKey{
		UserID: member.ID,
		TeamID: &teamID,
		Key:    "sk-team-delete-" + uuid.NewString(),
		Name:   "delete-member-key",
	})

	_, err = integrationDB.ExecContext(ctx, `UPDATE users SET deleted_at = NOW() WHERE id = $1`, member.ID)
	require.NoError(t, err)

	var leftAt time.Time
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT left_at FROM team_memberships WHERE team_id = $1 AND user_id = $2`, teamID, member.ID).Scan(&leftAt))
	require.False(t, leftAt.IsZero())
	var status string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT status FROM api_keys WHERE id = $1`, apiKey.ID).Scan(&status))
	require.Equal(t, service.StatusAPIKeyDisabled, status)
}

func TestTeamOwnerKeyLockRequiresExplicitOwnerEnable(t *testing.T) {
	ensureTeamMigrations(t)
	ctx := context.Background()
	repo := NewTeamRepository(integrationDB)
	owner := mustCreateUser(t, integrationEntClient, &service.User{Email: uniqueTeamTestEmail("key-owner")})
	member := mustCreateUser(t, integrationEntClient, &service.User{Email: uniqueTeamTestEmail("key-member")})
	teamCtx, err := repo.Create(ctx, "团队密钥锁定测试", owner.ID, 5)
	require.NoError(t, err)
	token := uuid.NewString()
	_, err = repo.CreateInvitation(ctx, teamCtx.Team.ID, owner.ID, member.Email, token, time.Now().Add(time.Hour))
	require.NoError(t, err)
	_, err = repo.ResolveInvitation(ctx, token, member.ID, member.Email, "accepted", time.Now())
	require.NoError(t, err)
	teamID := teamCtx.Team.ID
	apiKey := mustCreateApiKey(t, integrationEntClient, &service.APIKey{
		UserID: member.ID,
		TeamID: &teamID,
		Key:    "sk-team-owner-lock-" + uuid.NewString(),
		Name:   "owner-lock",
	})

	_, err = repo.DisableTeamKey(ctx, teamID, apiKey.ID, nil)
	require.NoError(t, err)
	var status string
	var ownerDisabled bool
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT status, team_owner_disabled FROM api_keys WHERE id = $1`, apiKey.ID).Scan(&status, &ownerDisabled))
	require.Equal(t, service.StatusAPIKeyDisabled, status)
	require.True(t, ownerDisabled)

	// 普通更新只能改状态，无法清除 Owner 的独立锁定标记。
	_, err = integrationDB.ExecContext(ctx, `UPDATE api_keys SET status = 'active' WHERE id = $1`, apiKey.ID)
	require.NoError(t, err)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT status, team_owner_disabled FROM api_keys WHERE id = $1`, apiKey.ID).Scan(&status, &ownerDisabled))
	require.Equal(t, service.StatusAPIKeyActive, status)
	require.True(t, ownerDisabled)

	_, err = repo.EnableTeamKey(ctx, teamID, apiKey.ID, nil)
	require.NoError(t, err)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT status, team_owner_disabled FROM api_keys WHERE id = $1`, apiKey.ID).Scan(&status, &ownerDisabled))
	require.Equal(t, service.StatusAPIKeyActive, status)
	require.False(t, ownerDisabled)
}

func TestTeamInvitationCopiesCurrentDefaultMemberLimits(t *testing.T) {
	ensureTeamMigrations(t)
	ctx := context.Background()
	repo := NewTeamRepository(integrationDB)
	owner := mustCreateUser(t, integrationEntClient, &service.User{Email: uniqueTeamTestEmail("default-limit-owner")})
	member := mustCreateUser(t, integrationEntClient, &service.User{Email: uniqueTeamTestEmail("default-limit-member")})
	teamCtx, err := repo.Create(ctx, "默认限额团队", owner.ID, 10)
	require.NoError(t, err)
	require.NoError(t, repo.SetDefaultMemberLimits(ctx, teamCtx.Team.ID, 1.5, 8, 30))

	token := uuid.NewString()
	_, err = repo.CreateInvitation(ctx, teamCtx.Team.ID, owner.ID, member.Email, token, time.Now().Add(time.Hour))
	require.NoError(t, err)
	memberCtx, err := repo.ResolveInvitation(ctx, token, member.ID, member.Email, "accepted", time.Now())
	require.NoError(t, err)
	require.InDelta(t, 1.5, memberCtx.Membership.DailyLimitUSD, 0.000001)
	require.InDelta(t, 8, memberCtx.Membership.WeeklyLimitUSD, 0.000001)
	require.InDelta(t, 30, memberCtx.Membership.MonthlyLimitUSD, 0.000001)

	// 后续修改默认值只影响新成员，不追溯覆盖已经加入的成员。
	require.NoError(t, repo.SetDefaultMemberLimits(ctx, teamCtx.Team.ID, 2, 10, 40))
	memberCtx, err = repo.GetContextByUserID(ctx, member.ID)
	require.NoError(t, err)
	require.InDelta(t, 1.5, memberCtx.Membership.DailyLimitUSD, 0.000001)
}

func TestTeamMemberUsageSeriesKeepsDepartedMemberHistory(t *testing.T) {
	ensureTeamMigrations(t)
	ctx := context.Background()
	teamRepo := NewTeamRepository(integrationDB)
	usageRepo := newUsageLogRepositoryWithSQL(integrationEntClient, integrationDB)
	owner := mustCreateUser(t, integrationEntClient, &service.User{Email: uniqueTeamTestEmail("usage-owner")})
	member := mustCreateUser(t, integrationEntClient, &service.User{Email: uniqueTeamTestEmail("usage-member")})
	teamCtx, err := teamRepo.Create(ctx, "历史成员用量团队", owner.ID, 5)
	require.NoError(t, err)
	token := uuid.NewString()
	_, err = teamRepo.CreateInvitation(ctx, teamCtx.Team.ID, owner.ID, member.Email, token, time.Now().Add(time.Hour))
	require.NoError(t, err)
	_, err = teamRepo.ResolveInvitation(ctx, token, member.ID, member.Email, "accepted", time.Now())
	require.NoError(t, err)
	teamID := teamCtx.Team.ID
	ownerKey := mustCreateApiKey(t, integrationEntClient, &service.APIKey{UserID: owner.ID, TeamID: &teamID, Key: "sk-team-usage-owner-" + uuid.NewString()})
	memberKey := mustCreateApiKey(t, integrationEntClient, &service.APIKey{UserID: member.ID, TeamID: &teamID, Key: "sk-team-usage-member-" + uuid.NewString()})
	account := mustCreateAccount(t, integrationEntClient, &service.Account{Name: "team-usage-" + uuid.NewString(), Type: service.AccountTypeAPIKey})
	createdAt := time.Now().UTC()
	for _, item := range []struct {
		userID   int64
		apiKeyID int64
		cost     float64
	}{
		{userID: owner.ID, apiKeyID: ownerKey.ID, cost: 1.2},
		{userID: member.ID, apiKeyID: memberKey.ID, cost: 0.8},
	} {
		_, err = usageRepo.Create(ctx, &service.UsageLog{
			// 本地 user_id 维持付款账号；actor_user_id/team_id 由迁移触发器从 Key 补齐。
			UserID:       owner.ID,
			APIKeyID:     item.apiKeyID,
			AccountID:    account.ID,
			RequestID:    uuid.NewString(),
			Model:        "team-usage-test",
			InputTokens:  10,
			OutputTokens: 5,
			TotalCost:    item.cost,
			ActualCost:   item.cost,
			CreatedAt:    createdAt,
		})
		require.NoError(t, err)
	}
	var persistedUserID, persistedBillingUserID, persistedActorUserID, persistedTeamID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT user_id, billing_user_id, actor_user_id, team_id
		FROM usage_logs WHERE api_key_id = $1 ORDER BY id DESC LIMIT 1`, memberKey.ID).
		Scan(&persistedUserID, &persistedBillingUserID, &persistedActorUserID, &persistedTeamID))
	require.Equal(t, owner.ID, persistedUserID, "local Reliability customer identity remains the payer account")
	require.Equal(t, owner.ID, persistedBillingUserID)
	require.Equal(t, member.ID, persistedActorUserID)
	require.Equal(t, teamID, persistedTeamID)
	var memberDailyUsage float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT daily_usage_usd FROM team_memberships
		WHERE team_id=$1 AND user_id=$2 AND left_at IS NULL`, teamID, member.ID).Scan(&memberDailyUsage))
	require.InDelta(t, 0.8, memberDailyUsage, 0.000001)
	require.NoError(t, teamRepo.RemoveMember(ctx, teamID, member.ID, time.Now()))

	query := service.TeamUsageQuery{From: createdAt.Add(-time.Hour), To: createdAt.Add(time.Hour)}
	total, err := teamRepo.GetUsageSummary(ctx, teamID, query)
	require.NoError(t, err)
	series, err := teamRepo.ListMemberUsageSeries(ctx, teamID, query)
	require.NoError(t, err)

	var seriesTotal float64
	statuses := make(map[int64]string, len(series))
	for _, item := range series {
		seriesTotal += item.Summary.ActualCost
		statuses[item.ActorUserID] = item.Status
	}
	require.InDelta(t, total.ActualCost, seriesTotal, 0.000001)
	require.InDelta(t, 2, seriesTotal, 0.000001)
	require.Equal(t, "active", statuses[owner.ID])
	require.Equal(t, "left", statuses[member.ID])
}

func uniqueTeamTestEmail(prefix string) string {
	return fmt.Sprintf("team-%s-%s@example.com", prefix, uuid.NewString())
}
