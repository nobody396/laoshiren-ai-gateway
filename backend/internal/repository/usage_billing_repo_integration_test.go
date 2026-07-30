//go:build integration

package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

func TestUsageBillingRepositoryApply_DeduplicatesBalanceBilling(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-user-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Balance:      100,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-usage-billing-" + uuid.NewString(),
		Name:   "billing",
		Quota:  1,
	})
	account := mustCreateAccount(t, client, &service.Account{
		Name: "usage-billing-account-" + uuid.NewString(),
		Type: service.AccountTypeAPIKey,
	})

	requestID := uuid.NewString()
	cmd := &service.UsageBillingCommand{
		RequestID:           requestID,
		APIKeyID:            apiKey.ID,
		UserID:              user.ID,
		AccountID:           account.ID,
		AccountType:         service.AccountTypeAPIKey,
		BalanceCost:         1.25,
		APIKeyQuotaCost:     1.25,
		APIKeyRateLimitCost: 1.25,
	}

	result1, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.NotNil(t, result1)
	require.True(t, result1.Applied)
	require.True(t, result1.APIKeyQuotaExhausted)

	result2, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.NotNil(t, result2)
	require.False(t, result2.Applied)

	var balance float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT balance FROM users WHERE id = $1", user.ID).Scan(&balance))
	require.InDelta(t, 98.75, balance, 0.000001)

	var quotaUsed float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT quota_used FROM api_keys WHERE id = $1", apiKey.ID).Scan(&quotaUsed))
	require.InDelta(t, 1.25, quotaUsed, 0.000001)

	var usage5h float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT usage_5h FROM api_keys WHERE id = $1", apiKey.ID).Scan(&usage5h))
	require.InDelta(t, 1.25, usage5h, 0.000001)

	var status string
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT status FROM api_keys WHERE id = $1", apiKey.ID).Scan(&status))
	require.Equal(t, service.StatusAPIKeyQuotaExhausted, status)

	var dedupCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_billing_dedup WHERE request_id = $1 AND api_key_id = $2", requestID, apiKey.ID).Scan(&dedupCount))
	require.Equal(t, 1, dedupCount)
}

func TestUsageBillingRepositoryApply_AttributesOnlyPaidBalanceLots(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-affiliate-balance-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Balance:      20,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-usage-billing-affiliate-balance-" + uuid.NewString(),
		Name:   "affiliate-balance",
	})
	usageLogID := time.Now().UnixNano()

	_, err := integrationDB.ExecContext(ctx, `
		INSERT INTO balance_lots (
			user_id, source_type, source_key,
			original_amount_micros, remaining_amount_micros,
			affiliate_eligible, affiliate_policy
		)
		VALUES
			($1, 'gift', $2, 3000000, 3000000, FALSE, 'NONE'),
			($1, 'paid_redeem', $3, 17000000, 17000000, FALSE, 'NONE')
	`, user.ID, "test-gift:"+uuid.NewString(), "test-paid:"+uuid.NewString())
	require.NoError(t, err)
	setAffiliateProgramLiveForIntegrationTest(t, ctx)

	result, err := repo.Apply(ctx, &service.UsageBillingCommand{
		RequestID:   uuid.NewString(),
		APIKeyID:    apiKey.ID,
		UsageLogID:  usageLogID,
		UserID:      user.ID,
		BalanceCost: 5,
	})
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Equal(t, int64(2_000_000), result.BalanceConfirmedMicros)
	require.Equal(t, int64(2_000_000), result.ConfirmedConsumptionMicros)

	var eventAmount int64
	var eventPolicy string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(amount_micros), 0), MAX(affiliate_policy)
		FROM affiliate_performance_events
		WHERE event_key LIKE $1
	`, fmt.Sprintf("confirmed:usage:%d:balance:lot:%%", usageLogID)).Scan(
		&eventAmount,
		&eventPolicy,
	))
	require.Equal(t, int64(2_000_000), eventAmount)
	require.Equal(t, service.AffiliateSourcePolicyNone, eventPolicy)
}

func TestUsageBillingRepositoryApply_AttributesMonthlyConsumptionProRata(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-affiliate-monthly-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	limit := 100.0
	group := mustCreateGroup(t, client, &service.Group{
		Name:             "usage-billing-affiliate-monthly-" + uuid.NewString(),
		Platform:         service.PlatformOpenAI,
		SubscriptionType: service.SubscriptionTypeCredit,
		DailyLimitUSD:    &limit,
		WeeklyLimitUSD:   &limit,
		MonthlyLimitUSD:  &limit,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID:  user.ID,
		GroupID: &group.ID,
		Key:     "sk-usage-billing-affiliate-monthly-" + uuid.NewString(),
		Name:    "affiliate-monthly",
	})
	subscription := mustCreateSubscription(t, client, &service.UserSubscription{
		UserID:  user.ID,
		GroupID: group.ID,
	})
	var cycleID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		INSERT INTO monthly_entitlement_cycles (
			user_id, source_type, source_key, product_code,
			sale_price_micros, credit_limit_micros,
			affiliate_eligible, affiliate_policy, starts_at, ends_at
		)
		VALUES ($1, 'paid_topup', $2, 'test-monthly', 50000000, 100000000, FALSE, 'NONE', NOW() - INTERVAL '1 minute', NOW() + INTERVAL '31 days')
		RETURNING id
	`, user.ID, "test-monthly:"+uuid.NewString()).Scan(&cycleID))
	_, err := integrationDB.ExecContext(ctx, `
		INSERT INTO monthly_entitlement_cycle_subscriptions (
			cycle_id, user_subscription_id, group_id
		)
		VALUES ($1, $2, $3)
	`, cycleID, subscription.ID, group.ID)
	require.NoError(t, err)
	setAffiliateProgramLiveForIntegrationTest(t, ctx)
	usageLogID := time.Now().UnixNano()

	result, err := repo.Apply(ctx, &service.UsageBillingCommand{
		RequestID:        uuid.NewString(),
		APIKeyID:         apiKey.ID,
		UsageLogID:       usageLogID,
		UserID:           user.ID,
		SubscriptionID:   &subscription.ID,
		SubscriptionCost: 10,
	})
	require.NoError(t, err)
	require.Equal(t, int64(5_000_000), result.MonthlyConfirmedMicros)
	require.Equal(t, int64(5_000_000), result.ConfirmedConsumptionMicros)

	var usedCredit, confirmed int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT used_credit_micros, confirmed_consumption_micros
		FROM monthly_entitlement_cycles
		WHERE id = $1
	`, cycleID).Scan(&usedCredit, &confirmed))
	require.Equal(t, int64(10_000_000), usedCredit)
	require.Equal(t, int64(5_000_000), confirmed)

	var eventAmount int64
	var eventPolicy string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT amount_micros, affiliate_policy
		FROM affiliate_performance_events
		WHERE event_key = $1
	`, fmt.Sprintf("confirmed:usage:%d:monthly", usageLogID)).Scan(
		&eventAmount,
		&eventPolicy,
	))
	require.Equal(t, int64(5_000_000), eventAmount)
	require.Equal(t, service.AffiliateSourcePolicyNone, eventPolicy)
}

func TestUsageBillingRepositoryApply_SettlesFixedAgentPoolOnConfirmedConsumption(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)
	linkRepo := NewAffiliateLinkRepository(integrationDB)
	linkService := service.NewAffiliateLinkService(linkRepo)

	agent := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-agent-pool-agent-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	customer := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-agent-pool-customer-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Balance:      100,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: customer.ID,
		Key:    "sk-usage-billing-agent-pool-" + uuid.NewString(),
		Name:   "agent-pool",
	})
	_, err := integrationDB.ExecContext(ctx, `
		INSERT INTO agent_principals (
			agent_id, status, risk_status, qualified_at, activated_at
		)
		VALUES ($1, 'active', 'clear', NOW(), NOW())
	`, agent.ID)
	require.NoError(t, err)
	link, err := linkService.Create(ctx, agent.ID, "三七分成", "integration", 300)
	require.NoError(t, err)
	referral, err := linkRepo.ResolveActiveLink(ctx, link.Code)
	require.NoError(t, err)
	require.NoError(t, linkRepo.BindAgentReferral(ctx, customer.ID, *referral))
	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO balance_lots (
			user_id, source_type, source_key,
			original_amount_micros, remaining_amount_micros,
			affiliate_eligible, affiliate_policy, direct_partner_id,
			customer_rebate_rate_bps, partner_commission_rate_bps
		)
		VALUES ($1, 'paid_topup', $2, 100000000, 100000000, TRUE, 'PARTNER_USAGE', $3, 300, 700)
	`, customer.ID, "agent-pool-paid:"+uuid.NewString(), agent.ID)
	require.NoError(t, err)
	setAffiliateProgramLiveForIntegrationTest(t, ctx)
	usageLogID := time.Now().UnixNano()
	cmd := &service.UsageBillingCommand{
		RequestID:   uuid.NewString(),
		APIKeyID:    apiKey.ID,
		UsageLogID:  usageLogID,
		UserID:      customer.ID,
		BalanceCost: 10,
	}

	result, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.Equal(t, service.AffiliateProgramModeLive, result.AffiliateProgramMode)
	require.Equal(t, int64(300_000), result.AffiliateCustomerRebateMicros)
	require.Equal(t, int64(700_000), result.AffiliateAgentCommissionMicros)
	require.NotNil(t, result.NewBalance)
	require.InDelta(t, 90.3, *result.NewBalance, 0.000001)

	var cashMicros int64
	var postingStatus string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT amount_micros, posting_status
		FROM agent_cash_commission_entries
		WHERE agent_id = $1
			AND consumer_user_id = $2
			AND source_type = 'confirmed_consumption'
	`, agent.ID, customer.ID).Scan(&cashMicros, &postingStatus))
	require.Equal(t, int64(700_000), cashMicros)
	require.Equal(t, "posted", postingStatus)

	replay, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.False(t, replay.Applied)
	var cashCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM agent_cash_commission_entries
		WHERE agent_id = $1
			AND consumer_user_id = $2
	`, agent.ID, customer.ID).Scan(&cashCount))
	require.Equal(t, 1, cashCount)
}

func TestUsageBillingAffiliateSettlement_ShadowObservesWithoutMoney(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	agent := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-shadow-agent-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	customer := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-shadow-customer-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	_, err := integrationDB.ExecContext(ctx, `
		INSERT INTO agent_principals (
			agent_id, status, risk_status, qualified_at, activated_at
		)
		VALUES ($1, 'active', 'clear', NOW(), NOW())
	`, agent.ID)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO affiliate_links (
			agent_id, code, name, is_default, status, current_rate_version
		)
		VALUES ($1, $2, 'shadow', TRUE, 'active', 1)
	`, agent.ID, "shadow-link-"+uuid.NewString())
	require.NoError(t, err)
	var linkID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT id FROM affiliate_links WHERE agent_id=$1 AND is_default=TRUE
	`, agent.ID).Scan(&linkID))
	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO affiliate_link_rate_versions (
			link_id, version, customer_rebate_rate_bps,
			agent_commission_rate_bps, effective_at
		)
		VALUES ($1, 1, 300, 700, NOW())
	`, linkID)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO affiliate_bindings (
			customer_user_id, inviter_user_id, binding_kind,
			agent_id, affiliate_link_id, link_rate_version,
			customer_rebate_rate_snapshot_bps,
			agent_commission_rate_snapshot_bps
		)
		VALUES ($2, $3, 'agent', $3, $1, 1, 300, 700)
	`, linkID, customer.ID, agent.ID)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `
		UPDATE affiliate_program_settings
		SET mode='shadow', started_at=NULL
		WHERE id=1
	`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `
			UPDATE affiliate_program_settings SET mode='off', started_at=NULL WHERE id=1
		`)
	})

	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	settlement, err := recordUsageBillingPerformanceEvent(
		ctx,
		tx,
		customer.ID,
		time.Now().UnixNano(),
		"balance_usage",
		"shadow-event:"+uuid.NewString(),
		10_000_000,
		service.AffiliateSourcePolicyPartnerUsage,
		agent.ID,
		300,
		700,
		time.Now(),
	)
	require.NoError(t, err)
	require.Zero(t, settlement.CustomerRebateMicros)
	require.Zero(t, settlement.AgentCommissionMicros)
	require.NoError(t, tx.Commit())

	var rewards, cash int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM affiliate_reward_entries WHERE consumer_user_id=$1
	`, customer.ID).Scan(&rewards))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM agent_cash_commission_entries WHERE consumer_user_id=$1
	`, customer.ID).Scan(&cash))
	require.Zero(t, rewards)
	require.Zero(t, cash)
}

func TestUsageBillingAffiliateSettlement_RealRedeemShadowThenLiveCutover(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	usageRepo := NewUsageBillingRepository(client, integrationDB)
	userRepo := newUserRepositoryWithSQL(client, integrationDB)
	redeemRepo := NewRedeemCodeRepository(client)
	consumptionRepo := NewAffiliateConsumptionRepository(client)
	rewardService := service.NewAffiliateRewardService(NewAffiliateRewardRepository(client, integrationDB))
	redeemService := service.NewRedeemService(
		redeemRepo,
		nil,
		userRepo,
		nil,
		nil,
		nil,
		client,
		nil,
		nil,
		nil,
		consumptionRepo,
		rewardService,
	)
	linkRepo := NewAffiliateLinkRepository(integrationDB)
	linkService := service.NewAffiliateLinkService(linkRepo)

	agent := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-real-shadow-agent-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	customer := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-real-shadow-customer-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: customer.ID,
		Key:    "sk-usage-billing-real-shadow-" + uuid.NewString(),
		Name:   "real-shadow-cutover",
	})
	_, err := integrationDB.ExecContext(ctx, `
		INSERT INTO agent_principals (
			agent_id, status, risk_status, qualified_at, activated_at
		)
		VALUES ($1, 'active', 'clear', NOW(), NOW())
	`, agent.ID)
	require.NoError(t, err)
	link, err := linkService.Create(ctx, agent.ID, "真实观察三七", "integration", 300)
	require.NoError(t, err)
	referral, err := linkRepo.ResolveActiveLink(ctx, link.Code)
	require.NoError(t, err)
	require.NoError(t, linkRepo.BindAgentReferral(ctx, customer.ID, *referral))
	setAffiliateProgramShadowForIntegrationTest(t, ctx)

	shadowCode := &service.RedeemCode{
		Code:        fmt.Sprintf("SHADOW-%d", time.Now().UnixNano()),
		Type:        service.RedeemTypeBalance,
		Value:       10,
		Status:      service.StatusUnused,
		Purpose:     service.RedeemCodePurposeSaleRecharge,
		SalesStatus: service.RedeemCodeSalesStatusSold,
	}
	require.NoError(t, redeemRepo.Create(ctx, shadowCode))
	_, err = redeemService.Redeem(ctx, customer.ID, shadowCode.Code)
	require.NoError(t, err)

	var (
		shadowLotPolicy        string
		shadowLotPartnerID     int64
		shadowLotCustomerRate  int32
		shadowLotPartnerRate   int32
		shadowLotAcquiredAt    time.Time
		shadowFirstPaidRecords int
	)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT
			affiliate_policy,
			direct_partner_id,
			customer_rebate_rate_bps,
			partner_commission_rate_bps,
			occurred_at
		FROM balance_lots
		WHERE source_key=$1
	`, fmt.Sprintf("redeem:balance:%d", shadowCode.ID)).Scan(
		&shadowLotPolicy,
		&shadowLotPartnerID,
		&shadowLotCustomerRate,
		&shadowLotPartnerRate,
		&shadowLotAcquiredAt,
	))
	require.Equal(t, service.AffiliateSourcePolicyPartnerUsage, shadowLotPolicy)
	require.Equal(t, agent.ID, shadowLotPartnerID)
	require.Equal(t, int32(300), shadowLotCustomerRate)
	require.Equal(t, int32(700), shadowLotPartnerRate)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM affiliate_first_paid_purchases WHERE user_id=$1
	`, customer.ID).Scan(&shadowFirstPaidRecords))
	require.Zero(t, shadowFirstPaidRecords)

	shadowUsageLogID := time.Now().UnixNano()
	shadowCommand := &service.UsageBillingCommand{
		RequestID:   uuid.NewString(),
		APIKeyID:    apiKey.ID,
		UsageLogID:  shadowUsageLogID,
		UserID:      customer.ID,
		BalanceCost: 3,
	}
	shadowResult, err := usageRepo.Apply(ctx, shadowCommand)
	require.NoError(t, err)
	require.True(t, shadowResult.Applied)
	require.Equal(t, service.AffiliateProgramModeShadow, shadowResult.AffiliateProgramMode)
	require.Zero(t, shadowResult.AffiliateCustomerRebateMicros)
	require.Zero(t, shadowResult.AffiliateAgentCommissionMicros)

	var (
		shadowEventCount  int
		shadowEventAmount int64
		shadowEventMode   string
		rewardCount       int
		cashCount         int
	)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COALESCE(SUM(amount_micros), 0),
			COALESCE(MAX(metadata ->> 'program_mode'), '')
		FROM affiliate_performance_events
		WHERE user_id=$1
			AND source_type='balance_usage'
			AND source_id=$2
	`, customer.ID, shadowUsageLogID).Scan(&shadowEventCount, &shadowEventAmount, &shadowEventMode))
	require.Equal(t, 1, shadowEventCount)
	require.Equal(t, int64(3_000_000), shadowEventAmount)
	require.Equal(t, service.AffiliateProgramModeShadow, shadowEventMode)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM affiliate_reward_entries WHERE consumer_user_id=$1
	`, customer.ID).Scan(&rewardCount))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM agent_cash_commission_entries WHERE consumer_user_id=$1
	`, customer.ID).Scan(&cashCount))
	require.Zero(t, rewardCount)
	require.Zero(t, cashCount)

	replay, err := usageRepo.Apply(ctx, shadowCommand)
	require.NoError(t, err)
	require.False(t, replay.Applied)

	var liveStartedAt time.Time
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		UPDATE affiliate_program_settings
		SET mode='live', started_at=NOW(), updated_at=NOW()
		WHERE id=1
		RETURNING started_at
	`).Scan(&liveStartedAt))
	require.True(t, shadowLotAcquiredAt.Before(liveStartedAt))

	blockedShadowUsageLogID := time.Now().UnixNano()
	blockedResult, err := usageRepo.Apply(ctx, &service.UsageBillingCommand{
		RequestID:   uuid.NewString(),
		APIKeyID:    apiKey.ID,
		UsageLogID:  blockedShadowUsageLogID,
		UserID:      customer.ID,
		BalanceCost: 7,
	})
	require.NoError(t, err)
	require.True(t, blockedResult.Applied)
	require.Equal(t, service.AffiliateProgramModeLive, blockedResult.AffiliateProgramMode)
	require.Zero(t, blockedResult.AffiliateCustomerRebateMicros)
	require.Zero(t, blockedResult.AffiliateAgentCommissionMicros)
	var blockedEventCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM affiliate_performance_events
		WHERE user_id=$1 AND source_id=$2
	`, customer.ID, blockedShadowUsageLogID).Scan(&blockedEventCount))
	require.Zero(t, blockedEventCount)

	liveCode := &service.RedeemCode{
		Code:        fmt.Sprintf("LIVE-%d", time.Now().UnixNano()),
		Type:        service.RedeemTypeBalance,
		Value:       10,
		Status:      service.StatusUnused,
		Purpose:     service.RedeemCodePurposeSaleRecharge,
		SalesStatus: service.RedeemCodeSalesStatusSold,
	}
	require.NoError(t, redeemRepo.Create(ctx, liveCode))
	_, err = redeemService.Redeem(ctx, customer.ID, liveCode.Code)
	require.NoError(t, err)
	liveUsageLogID := time.Now().UnixNano()
	liveResult, err := usageRepo.Apply(ctx, &service.UsageBillingCommand{
		RequestID:   uuid.NewString(),
		APIKeyID:    apiKey.ID,
		UsageLogID:  liveUsageLogID,
		UserID:      customer.ID,
		BalanceCost: 5,
	})
	require.NoError(t, err)
	require.True(t, liveResult.Applied)
	require.Equal(t, int64(150_000), liveResult.AffiliateCustomerRebateMicros)
	require.Equal(t, int64(350_000), liveResult.AffiliateAgentCommissionMicros)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM affiliate_reward_entries
		WHERE consumer_user_id=$1 AND reward_type='customer_rebate'
	`, customer.ID).Scan(&rewardCount))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM agent_cash_commission_entries
		WHERE consumer_user_id=$1 AND entry_type='earned'
	`, customer.ID).Scan(&cashCount))
	require.Equal(t, 1, rewardCount)
	require.Equal(t, 1, cashCount)
}

func TestUsageBillingAffiliateSettlement_MonthlyRedeemShadowNeverSettlesAfterCutover(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	usageRepo := NewUsageBillingRepository(client, integrationDB)
	userRepo := newUserRepositoryWithSQL(client, integrationDB)
	redeemRepo := NewRedeemCodeRepository(client)
	groupRepo := NewGroupRepository(client, integrationDB)
	subscriptionRepo := NewUserSubscriptionRepository(client)
	subscriptionService := service.NewSubscriptionService(groupRepo, subscriptionRepo, nil, client, nil)
	consumptionRepo := NewAffiliateConsumptionRepository(client)
	rewardService := service.NewAffiliateRewardService(NewAffiliateRewardRepository(client, integrationDB))
	redeemService := service.NewRedeemService(
		redeemRepo,
		nil,
		userRepo,
		subscriptionService,
		nil,
		nil,
		client,
		nil,
		nil,
		nil,
		consumptionRepo,
		rewardService,
	)
	linkRepo := NewAffiliateLinkRepository(integrationDB)
	linkService := service.NewAffiliateLinkService(linkRepo)

	agent := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-monthly-shadow-agent-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	customer := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-monthly-shadow-customer-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	limit := 100.0
	group := mustCreateGroup(t, client, &service.Group{
		Name:             "usage-monthly-shadow-" + uuid.NewString(),
		Platform:         service.PlatformOpenAI,
		SubscriptionType: service.SubscriptionTypeCredit,
		MonthlyLimitUSD:  &limit,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID:  customer.ID,
		GroupID: &group.ID,
		Key:     "sk-usage-monthly-shadow-" + uuid.NewString(),
		Name:    "monthly-shadow-cutover",
	})
	_, err := integrationDB.ExecContext(ctx, `
		INSERT INTO agent_principals (
			agent_id, status, risk_status, qualified_at, activated_at
		)
		VALUES ($1, 'active', 'clear', NOW(), NOW())
	`, agent.ID)
	require.NoError(t, err)
	link, err := linkService.Create(ctx, agent.ID, "月卡观察三七", "integration", 300)
	require.NoError(t, err)
	referral, err := linkRepo.ResolveActiveLink(ctx, link.Code)
	require.NoError(t, err)
	require.NoError(t, linkRepo.BindAgentReferral(ctx, customer.ID, *referral))
	setAffiliateProgramShadowForIntegrationTest(t, ctx)

	monthlyCode := &service.RedeemCode{
		Code:         fmt.Sprintf("MONTHLY-%d", time.Now().UnixNano()),
		Type:         service.RedeemTypeSubscription,
		Value:        10,
		Status:       service.StatusUnused,
		GroupID:      &group.ID,
		GroupIDs:     []int64{group.ID},
		ValidityDays: 31,
		Purpose:      service.RedeemCodePurposeSaleRecharge,
		SalesStatus:  service.RedeemCodeSalesStatusSold,
	}
	require.NoError(t, redeemRepo.Create(ctx, monthlyCode))
	_, err = redeemService.Redeem(ctx, customer.ID, monthlyCode.Code)
	require.NoError(t, err)
	subscription, err := subscriptionRepo.GetByUserIDAndGroupID(ctx, customer.ID, group.ID)
	require.NoError(t, err)

	var (
		cyclePolicy     string
		cyclePartnerID  int64
		cycleCreatedAt  time.Time
		cycleSaleMicros int64
		cycleLimit      int64
	)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT
			affiliate_policy,
			direct_partner_id,
			created_at,
			sale_price_micros,
			credit_limit_micros
		FROM monthly_entitlement_cycles
		WHERE source_key=$1
	`, fmt.Sprintf("redeem:subscription:%d", monthlyCode.ID)).Scan(
		&cyclePolicy,
		&cyclePartnerID,
		&cycleCreatedAt,
		&cycleSaleMicros,
		&cycleLimit,
	))
	require.Equal(t, service.AffiliateSourcePolicyPartnerUsage, cyclePolicy)
	require.Equal(t, agent.ID, cyclePartnerID)
	require.Equal(t, int64(10_000_000), cycleSaleMicros)
	require.Equal(t, int64(100_000_000), cycleLimit)

	shadowUsageLogID := time.Now().UnixNano()
	shadowResult, err := usageRepo.Apply(ctx, &service.UsageBillingCommand{
		RequestID:        uuid.NewString(),
		APIKeyID:         apiKey.ID,
		UsageLogID:       shadowUsageLogID,
		UserID:           customer.ID,
		SubscriptionID:   &subscription.ID,
		SubscriptionCost: 30,
	})
	require.NoError(t, err)
	require.True(t, shadowResult.Applied)
	require.Equal(t, service.AffiliateProgramModeShadow, shadowResult.AffiliateProgramMode)
	require.Equal(t, int64(3_000_000), shadowResult.MonthlyConfirmedMicros)
	require.Zero(t, shadowResult.AffiliateCustomerRebateMicros)
	require.Zero(t, shadowResult.AffiliateAgentCommissionMicros)

	var shadowEventCount, rewardCount, cashCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM affiliate_performance_events
		WHERE user_id=$1
			AND source_id=$2
			AND metadata ->> 'program_mode'='shadow'
	`, customer.ID, shadowUsageLogID).Scan(&shadowEventCount))
	require.Equal(t, 1, shadowEventCount)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM affiliate_reward_entries WHERE consumer_user_id=$1
	`, customer.ID).Scan(&rewardCount))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM agent_cash_commission_entries WHERE consumer_user_id=$1
	`, customer.ID).Scan(&cashCount))
	require.Zero(t, rewardCount)
	require.Zero(t, cashCount)

	var liveStartedAt time.Time
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		UPDATE affiliate_program_settings
		SET mode='live', started_at=NOW(), updated_at=NOW()
		WHERE id=1
		RETURNING started_at
	`).Scan(&liveStartedAt))
	require.True(t, cycleCreatedAt.Before(liveStartedAt))
	blockedUsageLogID := time.Now().UnixNano()
	blockedResult, err := usageRepo.Apply(ctx, &service.UsageBillingCommand{
		RequestID:        uuid.NewString(),
		APIKeyID:         apiKey.ID,
		UsageLogID:       blockedUsageLogID,
		UserID:           customer.ID,
		SubscriptionID:   &subscription.ID,
		SubscriptionCost: 70,
	})
	require.NoError(t, err)
	require.True(t, blockedResult.Applied)
	require.Equal(t, int64(7_000_000), blockedResult.MonthlyConfirmedMicros)
	require.Zero(t, blockedResult.AffiliateCustomerRebateMicros)
	require.Zero(t, blockedResult.AffiliateAgentCommissionMicros)
	var blockedEventCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM affiliate_performance_events
		WHERE user_id=$1 AND source_id=$2
	`, customer.ID, blockedUsageLogID).Scan(&blockedEventCount))
	require.Zero(t, blockedEventCount)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM affiliate_reward_entries WHERE consumer_user_id=$1
	`, customer.ID).Scan(&rewardCount))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM agent_cash_commission_entries WHERE consumer_user_id=$1
	`, customer.ID).Scan(&cashCount))
	require.Zero(t, rewardCount)
	require.Zero(t, cashCount)
}

func setAffiliateProgramLiveForIntegrationTest(t *testing.T, ctx context.Context) {
	t.Helper()
	_, err := integrationDB.ExecContext(ctx, `
		UPDATE affiliate_program_settings
		SET mode = 'live',
			started_at = NOW() - INTERVAL '1 minute',
			updated_at = NOW()
		WHERE id = 1
	`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `
			UPDATE affiliate_program_settings
			SET mode = 'off',
				started_at = NULL,
				updated_at = NOW()
			WHERE id = 1
		`)
	})
}

func TestUsageBillingRepositoryApply_DeduplicatesSubscriptionBilling(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-sub-user-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	group := mustCreateGroup(t, client, &service.Group{
		Name:             "usage-billing-group-" + uuid.NewString(),
		Platform:         service.PlatformAnthropic,
		SubscriptionType: service.SubscriptionTypeSubscription,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID:  user.ID,
		GroupID: &group.ID,
		Key:     "sk-usage-billing-sub-" + uuid.NewString(),
		Name:    "billing-sub",
	})
	subscription := mustCreateSubscription(t, client, &service.UserSubscription{
		UserID:  user.ID,
		GroupID: group.ID,
	})

	requestID := uuid.NewString()
	cmd := &service.UsageBillingCommand{
		RequestID:        requestID,
		APIKeyID:         apiKey.ID,
		UserID:           user.ID,
		AccountID:        0,
		SubscriptionID:   &subscription.ID,
		SubscriptionCost: 2.5,
	}

	result1, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.True(t, result1.Applied)

	result2, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.False(t, result2.Applied)

	var dailyUsage float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT daily_usage_usd FROM user_subscriptions WHERE id = $1", subscription.ID).Scan(&dailyUsage))
	require.InDelta(t, 2.5, dailyUsage, 0.000001)
}

func TestUsageBillingRepositoryApply_SharedSubscriptionBillingUsesOneQuotaPool(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-shared-sub-user-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	limit := 10.00
	gptGroup := mustCreateGroup(t, client, &service.Group{
		Name:             "usage-billing-shared-gpt-" + uuid.NewString(),
		Platform:         service.PlatformOpenAI,
		SubscriptionType: service.SubscriptionTypeSubscription,
		DailyLimitUSD:    &limit,
		WeeklyLimitUSD:   &limit,
		MonthlyLimitUSD:  &limit,
	})
	claudeGroup := mustCreateGroup(t, client, &service.Group{
		Name:             "usage-billing-shared-claude-" + uuid.NewString(),
		Platform:         service.PlatformAnthropic,
		SubscriptionType: service.SubscriptionTypeSubscription,
		DailyLimitUSD:    &limit,
		WeeklyLimitUSD:   &limit,
		MonthlyLimitUSD:  &limit,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID:  user.ID,
		GroupID: &gptGroup.ID,
		Key:     "sk-usage-billing-shared-sub-" + uuid.NewString(),
		Name:    "billing-shared-sub",
	})
	notes := "通过兑换码 BUNDLE-GPT-CLAUDE 兑换"
	gptSub := mustCreateSubscription(t, client, &service.UserSubscription{
		UserID:          user.ID,
		GroupID:         gptGroup.ID,
		DailyUsageUSD:   4,
		WeeklyUsageUSD:  4,
		MonthlyUsageUSD: 4,
		Notes:           notes,
	})
	claudeSub := mustCreateSubscription(t, client, &service.UserSubscription{
		UserID:          user.ID,
		GroupID:         claudeGroup.ID,
		DailyUsageUSD:   4,
		WeeklyUsageUSD:  4,
		MonthlyUsageUSD: 4,
		Notes:           notes,
	})

	result, err := repo.Apply(ctx, &service.UsageBillingCommand{
		RequestID:        uuid.NewString(),
		APIKeyID:         apiKey.ID,
		UserID:           user.ID,
		SubscriptionID:   &gptSub.ID,
		SubscriptionCost: 2,
	})
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.ElementsMatch(t, []service.SubscriptionUsageUpdate{
		{UserID: user.ID, GroupID: gptGroup.ID, CostUSD: 2},
		{UserID: user.ID, GroupID: claudeGroup.ID, CostUSD: 2},
	}, result.SubscriptionUsageUpdates)

	var gptDaily, claudeDaily float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT daily_usage_usd FROM user_subscriptions WHERE id = $1", gptSub.ID).Scan(&gptDaily))
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT daily_usage_usd FROM user_subscriptions WHERE id = $1", claudeSub.ID).Scan(&claudeDaily))
	require.InDelta(t, 6, gptDaily, 0.000001)
	require.InDelta(t, 6, claudeDaily, 0.000001)

	capRequestID := uuid.NewString()
	result, err = repo.Apply(ctx, &service.UsageBillingCommand{
		RequestID:        capRequestID,
		APIKeyID:         apiKey.ID,
		UserID:           user.ID,
		SubscriptionID:   &claudeSub.ID,
		SubscriptionCost: 5,
	})
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.ElementsMatch(t, []service.SubscriptionUsageUpdate{
		{UserID: user.ID, GroupID: gptGroup.ID, CostUSD: 5},
		{UserID: user.ID, GroupID: claudeGroup.ID, CostUSD: 5},
	}, result.SubscriptionUsageUpdates)

	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT daily_usage_usd FROM user_subscriptions WHERE id = $1", gptSub.ID).Scan(&gptDaily))
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT daily_usage_usd FROM user_subscriptions WHERE id = $1", claudeSub.ID).Scan(&claudeDaily))
	require.InDelta(t, 10, gptDaily, 0.000001)
	require.InDelta(t, 10, claudeDaily, 0.000001)

	var dedupCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_billing_dedup WHERE request_id = $1 AND api_key_id = $2", capRequestID, apiKey.ID).Scan(&dedupCount))
	require.Equal(t, 1, dedupCount)
}

func TestUsageBillingRepositoryApply_BalanceFinalLimitRejectsInsufficientFunds(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-low-balance-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Balance:      0.01,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-usage-billing-low-balance-" + uuid.NewString(),
		Name:   "billing-low-balance",
	})

	requestID := uuid.NewString()
	_, err := repo.Apply(ctx, &service.UsageBillingCommand{
		RequestID:   requestID,
		APIKeyID:    apiKey.ID,
		UserID:      user.ID,
		BalanceCost: 1.00,
	})
	require.ErrorIs(t, err, service.ErrInsufficientBalance)

	var balance float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT balance FROM users WHERE id = $1", user.ID).Scan(&balance))
	require.InDelta(t, 0.01, balance, 0.000001)

	var dedupCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_billing_dedup WHERE request_id = $1 AND api_key_id = $2", requestID, apiKey.ID).Scan(&dedupCount))
	require.Equal(t, 0, dedupCount)
}

func TestUsageBillingRepositoryApply_SubscriptionFinalLimitCapsOverage(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-sub-limit-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	limit := 10.00
	group := mustCreateGroup(t, client, &service.Group{
		Name:             "usage-billing-sub-limit-" + uuid.NewString(),
		Platform:         service.PlatformAnthropic,
		SubscriptionType: service.SubscriptionTypeSubscription,
		DailyLimitUSD:    &limit,
		WeeklyLimitUSD:   &limit,
		MonthlyLimitUSD:  &limit,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID:  user.ID,
		GroupID: &group.ID,
		Key:     "sk-usage-billing-sub-limit-" + uuid.NewString(),
		Name:    "billing-sub-limit",
	})
	subscription := mustCreateSubscription(t, client, &service.UserSubscription{
		UserID:          user.ID,
		GroupID:         group.ID,
		DailyUsageUSD:   9.99,
		WeeklyUsageUSD:  9.99,
		MonthlyUsageUSD: 9.99,
	})

	requestID := uuid.NewString()
	result, err := repo.Apply(ctx, &service.UsageBillingCommand{
		RequestID:        requestID,
		APIKeyID:         apiKey.ID,
		UserID:           user.ID,
		SubscriptionID:   &subscription.ID,
		SubscriptionCost: 1.00,
	})
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Equal(t, []service.SubscriptionUsageUpdate{{
		UserID:  user.ID,
		GroupID: group.ID,
		CostUSD: 1.00,
	}}, result.SubscriptionUsageUpdates)

	var dailyUsage, weeklyUsage, monthlyUsage float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT daily_usage_usd, weekly_usage_usd, monthly_usage_usd
		FROM user_subscriptions
		WHERE id = $1
	`, subscription.ID).Scan(&dailyUsage, &weeklyUsage, &monthlyUsage))
	require.InDelta(t, 10, dailyUsage, 0.000001)
	require.InDelta(t, 10, weeklyUsage, 0.000001)
	require.InDelta(t, 10, monthlyUsage, 0.000001)

	var dedupCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_billing_dedup WHERE request_id = $1 AND api_key_id = $2", requestID, apiKey.ID).Scan(&dedupCount))
	require.Equal(t, 1, dedupCount)
}

func TestUsageBillingRepositoryApply_ConcurrentBalanceFinalLimitPreventsOverspend(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-concurrent-balance-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Balance:      1.00,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-usage-billing-concurrent-balance-" + uuid.NewString(),
		Name:   "billing-concurrent-balance",
	})

	const workers = 2
	var start sync.WaitGroup
	start.Add(1)
	var done sync.WaitGroup
	done.Add(workers)
	errCh := make(chan error, workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer done.Done()
			start.Wait()
			_, err := repo.Apply(ctx, &service.UsageBillingCommand{
				RequestID:   uuid.NewString(),
				APIKeyID:    apiKey.ID,
				UserID:      user.ID,
				BalanceCost: 0.75,
			})
			errCh <- err
		}()
	}
	start.Done()
	done.Wait()
	close(errCh)

	var successes int32
	var insufficient int32
	for err := range errCh {
		switch {
		case err == nil:
			atomic.AddInt32(&successes, 1)
		case errors.Is(err, service.ErrInsufficientBalance):
			atomic.AddInt32(&insufficient, 1)
		default:
			require.NoError(t, err)
		}
	}
	require.Equal(t, int32(1), successes)
	require.Equal(t, int32(1), insufficient)

	var balance float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT balance FROM users WHERE id = $1", user.ID).Scan(&balance))
	require.InDelta(t, 0.25, balance, 0.000001)

	var dedupCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_billing_dedup WHERE api_key_id = $1", apiKey.ID).Scan(&dedupCount))
	require.Equal(t, 1, dedupCount)
}

func TestUsageBillingRepositoryApply_RequestFingerprintConflict(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-conflict-user-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Balance:      100,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-usage-billing-conflict-" + uuid.NewString(),
		Name:   "billing-conflict",
	})

	requestID := uuid.NewString()
	_, err := repo.Apply(ctx, &service.UsageBillingCommand{
		RequestID:   requestID,
		APIKeyID:    apiKey.ID,
		UserID:      user.ID,
		BalanceCost: 1.25,
	})
	require.NoError(t, err)

	_, err = repo.Apply(ctx, &service.UsageBillingCommand{
		RequestID:   requestID,
		APIKeyID:    apiKey.ID,
		UserID:      user.ID,
		BalanceCost: 2.50,
	})
	require.ErrorIs(t, err, service.ErrUsageBillingRequestConflict)
}

func TestUsageBillingRepositoryApply_UpdatesAccountQuota(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-account-user-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-usage-billing-account-" + uuid.NewString(),
		Name:   "billing-account",
	})
	account := mustCreateAccount(t, client, &service.Account{
		Name: "usage-billing-account-quota-" + uuid.NewString(),
		Type: service.AccountTypeAPIKey,
		Extra: map[string]any{
			"quota_limit": 100.0,
		},
	})

	_, err := repo.Apply(ctx, &service.UsageBillingCommand{
		RequestID:        uuid.NewString(),
		APIKeyID:         apiKey.ID,
		UserID:           user.ID,
		AccountID:        account.ID,
		AccountType:      service.AccountTypeAPIKey,
		AccountQuotaCost: 3.5,
	})
	require.NoError(t, err)

	var quotaUsed float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COALESCE((extra->>'quota_used')::numeric, 0) FROM accounts WHERE id = $1", account.ID).Scan(&quotaUsed))
	require.InDelta(t, 3.5, quotaUsed, 0.000001)
}

func TestDashboardAggregationRepositoryCleanupUsageBillingDedup_BatchDeletesOldRows(t *testing.T) {
	ctx := context.Background()
	repo := newDashboardAggregationRepositoryWithSQL(integrationDB)

	oldRequestID := "dedup-old-" + uuid.NewString()
	newRequestID := "dedup-new-" + uuid.NewString()
	oldCreatedAt := time.Now().UTC().AddDate(0, 0, -400)
	newCreatedAt := time.Now().UTC().Add(-time.Hour)

	_, err := integrationDB.ExecContext(ctx, `
		INSERT INTO usage_billing_dedup (request_id, api_key_id, request_fingerprint, created_at)
		VALUES ($1, 1, $2, $3), ($4, 1, $5, $6)
	`,
		oldRequestID, strings.Repeat("a", 64), oldCreatedAt,
		newRequestID, strings.Repeat("b", 64), newCreatedAt,
	)
	require.NoError(t, err)

	require.NoError(t, repo.CleanupUsageBillingDedup(ctx, time.Now().UTC().AddDate(0, 0, -365)))

	var oldCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_billing_dedup WHERE request_id = $1", oldRequestID).Scan(&oldCount))
	require.Equal(t, 0, oldCount)

	var newCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_billing_dedup WHERE request_id = $1", newRequestID).Scan(&newCount))
	require.Equal(t, 1, newCount)

	var archivedCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_billing_dedup_archive WHERE request_id = $1", oldRequestID).Scan(&archivedCount))
	require.Equal(t, 1, archivedCount)
}

func TestUsageBillingRepositoryApply_DeduplicatesAgainstArchivedKey(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)
	aggRepo := newDashboardAggregationRepositoryWithSQL(integrationDB)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-archive-user-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Balance:      100,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-usage-billing-archive-" + uuid.NewString(),
		Name:   "billing-archive",
	})

	requestID := uuid.NewString()
	cmd := &service.UsageBillingCommand{
		RequestID:   requestID,
		APIKeyID:    apiKey.ID,
		UserID:      user.ID,
		BalanceCost: 1.25,
	}

	result1, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.True(t, result1.Applied)

	_, err = integrationDB.ExecContext(ctx, `
		UPDATE usage_billing_dedup
		SET created_at = $1
		WHERE request_id = $2 AND api_key_id = $3
	`, time.Now().UTC().AddDate(0, 0, -400), requestID, apiKey.ID)
	require.NoError(t, err)
	require.NoError(t, aggRepo.CleanupUsageBillingDedup(ctx, time.Now().UTC().AddDate(0, 0, -365)))

	result2, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.False(t, result2.Applied)

	var balance float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT balance FROM users WHERE id = $1", user.ID).Scan(&balance))
	require.InDelta(t, 98.75, balance, 0.000001)
}

func TestUsageBillingPartnerSelfBalanceCommissionIsIdempotentReportableAndReversible(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-self-balance-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Balance:      25,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-usage-self-balance-" + uuid.NewString(),
		Name:   "self-balance",
	})
	admin := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-self-admin-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleAdmin,
	})
	createSelfCommissionIntegrationPartner(t, ctx, user.ID)
	setAffiliateProgramLiveForIntegrationTest(t, ctx)

	rewardResult, err := service.NewAffiliateRewardService(
		NewAffiliateRewardRepository(client, integrationDB),
	).ProcessFirstPaidPurchase(ctx, service.AffiliateFirstPaidPurchaseInput{
		UserID:       user.ID,
		PurchaseType: service.AffiliatePurchaseBalanceTopup,
		SourceID:     91001,
		PurchaseKey:  "self-balance:" + uuid.NewString(),
		AmountMicros: 20_000_000,
		OccurredAt:   time.Now(),
	})
	require.NoError(t, err)
	require.Equal(t, service.AffiliateSourcePolicyPartnerSelfUsage, rewardResult.SourcePolicy)
	require.Equal(t, user.ID, rewardResult.DirectPartnerID)
	require.Zero(t, rewardResult.CustomerRebateRateBPS)
	require.Equal(t, service.AffiliateAgentPoolRateBPS, rewardResult.PartnerCommissionRateBPS)
	var firstPaidClaims int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM affiliate_first_paid_purchases WHERE user_id=$1
	`, user.ID).Scan(&firstPaidClaims))
	require.Zero(t, firstPaidClaims, "self policy must not consume the ordinary first-paid claim")

	policy, partnerID, customerRate, partnerRate := service.AffiliatePolicyFromPurchaseResult(true, rewardResult)
	require.NoError(t, NewAffiliateConsumptionRepository(client).RecordBalanceLot(
		ctx,
		service.AffiliateBalanceLotInput{
			UserID:          user.ID,
			SourceType:      service.AffiliateSourcePaidTopup,
			SourceID:        91000,
			SourceKey:       "pre-self-balance-lot:" + uuid.NewString(),
			AmountMicros:    5_000_000,
			AffiliatePolicy: service.AffiliateSourcePolicyNone,
			OccurredAt:      time.Now().Add(-10 * time.Minute),
		},
	))
	require.NoError(t, NewAffiliateConsumptionRepository(client).RecordBalanceLot(
		ctx,
		service.AffiliateBalanceLotInput{
			UserID:                   user.ID,
			SourceType:               service.AffiliateSourcePaidTopup,
			SourceID:                 91001,
			SourceKey:                "self-balance-lot:" + uuid.NewString(),
			AmountMicros:             20_000_000,
			AffiliatePolicy:          policy,
			DirectPartnerID:          partnerID,
			CustomerRebateRateBPS:    customerRate,
			PartnerCommissionRateBPS: partnerRate,
			OccurredAt:               time.Now(),
		},
	))

	billingRepo := NewUsageBillingRepository(client, integrationDB)
	historicalResult, err := billingRepo.Apply(ctx, &service.UsageBillingCommand{
		RequestID:   uuid.NewString(),
		APIKeyID:    apiKey.ID,
		UsageLogID:  time.Now().UnixNano(),
		UserID:      user.ID,
		BalanceCost: 5,
	})
	require.NoError(t, err)
	require.True(t, historicalResult.Applied)
	require.Zero(t, historicalResult.AffiliateCustomerRebateMicros)
	require.Zero(t, historicalResult.AffiliateAgentCommissionMicros)

	usageLogID := time.Now().UnixNano()
	command := &service.UsageBillingCommand{
		RequestID:   uuid.NewString(),
		APIKeyID:    apiKey.ID,
		UsageLogID:  usageLogID,
		UserID:      user.ID,
		BalanceCost: 10,
	}
	result, err := billingRepo.Apply(ctx, command)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Zero(t, result.AffiliateCustomerRebateMicros)
	require.Equal(t, int64(1_000_000), result.AffiliateAgentCommissionMicros)
	require.NotNil(t, result.NewBalance)
	require.InDelta(t, 10, *result.NewBalance, 0.000001)

	var (
		eventID             int64
		eventPolicy         string
		eventMetadataPolicy string
	)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT id, affiliate_policy, metadata ->> 'attribution_policy'
		FROM affiliate_performance_events
		WHERE user_id=$1 AND source_id=$2 AND source_type='balance_usage'
	`, user.ID, usageLogID).Scan(&eventID, &eventPolicy, &eventMetadataPolicy))
	require.Equal(t, service.AffiliateSourcePolicyPartnerSelfUsage, eventPolicy)
	require.Equal(t, service.AffiliateSourcePolicyPartnerSelfUsage, eventMetadataPolicy)

	var (
		cashMicros       int64
		cashStatus       string
		cashKey          string
		cashPolicy       string
		cashDisplayType  string
		customerCashRate int32
		partnerCashRate  int32
	)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT
			amount_micros,
			posting_status,
			idempotency_key,
			metadata ->> 'attribution_policy',
			metadata ->> 'display_type',
			customer_rebate_rate_bps,
			agent_commission_rate_bps
		FROM agent_cash_commission_entries
		WHERE source_type='confirmed_consumption' AND source_id=$1
	`, eventID).Scan(
		&cashMicros,
		&cashStatus,
		&cashKey,
		&cashPolicy,
		&cashDisplayType,
		&customerCashRate,
		&partnerCashRate,
	))
	require.Equal(t, int64(1_000_000), cashMicros)
	require.Equal(t, "posted", cashStatus)
	require.Equal(t, fmt.Sprintf("confirmed:%d:self-cash", eventID), cashKey)
	require.Equal(t, service.AffiliateSourcePolicyPartnerSelfUsage, cashPolicy)
	require.Equal(t, "self_consumption_commission", cashDisplayType)
	require.Zero(t, customerCashRate)
	require.Equal(t, service.AffiliateAgentPoolRateBPS, partnerCashRate)

	commissionRepo := NewCommissionRepository(client, integrationDB)
	records, page, err := commissionRepo.ListByBeneficiary(
		ctx,
		user.ID,
		pagination.DefaultPagination(),
		"self_consumption_commission",
		nil,
		nil,
	)
	require.NoError(t, err)
	require.Equal(t, int64(1), page.Total)
	require.Len(t, records, 1)
	require.Equal(t, "self_consumption_commission", records[0].Type)
	require.Equal(t, user.ID, records[0].UserID)
	require.InDelta(t, 1, records[0].Amount, 0.000001)
	selfTotal, err := commissionRepo.SumByBeneficiaryTypeAndPeriod(
		ctx,
		user.ID,
		"self_consumption_commission",
		nil,
		nil,
	)
	require.NoError(t, err)
	require.InDelta(t, 1, selfTotal, 0.000001)
	regularTotal, err := commissionRepo.SumByBeneficiaryTypeAndPeriod(
		ctx,
		user.ID,
		"consumption_commission",
		nil,
		nil,
	)
	require.NoError(t, err)
	require.Zero(t, regularTotal)

	replay, err := billingRepo.Apply(ctx, command)
	require.NoError(t, err)
	require.False(t, replay.Applied)
	var earnedCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM agent_cash_commission_entries
		WHERE source_type='confirmed_consumption' AND source_id=$1
	`, eventID).Scan(&earnedCount))
	require.Equal(t, 1, earnedCount)

	riskService := service.NewAffiliateRiskService(NewAffiliateRiskRepository(integrationDB))
	reversal, err := riskService.ReversePerformanceEvent(ctx, eventID, "本人消费测试冲正", admin.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1_000_000), reversal.ReversedCashMicros)
	secondReversal, err := riskService.ReversePerformanceEvent(ctx, eventID, "重复冲正", admin.ID)
	require.NoError(t, err)
	require.Equal(t, reversal.ID, secondReversal.ID)

	var cashBalance int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(amount_micros), 0)::bigint
		FROM agent_cash_commission_entries
		WHERE agent_id=$1 AND posting_status='posted'
	`, user.ID).Scan(&cashBalance))
	require.Zero(t, cashBalance)
}

func TestAdminBalanceRoundTripDoesNotShadowPartnerSelfCommission(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("admin-roundtrip-self-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-admin-roundtrip-self-" + uuid.NewString(),
		Name:   "admin-roundtrip-self",
	})
	createSelfCommissionIntegrationPartner(t, ctx, user.ID)
	setAffiliateProgramLiveForIntegrationTest(t, ctx)

	userRepo := newUserRepositoryWithSQL(client, integrationDB)
	_, err := userRepo.ApplyAdminBalanceAdjustment(ctx, user.ID, 100, "add")
	require.NoError(t, err)
	_, err = userRepo.ApplyAdminBalanceAdjustment(ctx, user.ID, 100, "subtract")
	require.NoError(t, err)

	var adminLotRemaining int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT remaining_amount_micros
		FROM balance_lots
		WHERE user_id = $1
			AND source_type = 'admin_adjustment'
	`, user.ID).Scan(&adminLotRemaining))
	require.Zero(t, adminLotRemaining)

	const paidSourceID int64 = 92001
	rewardResult, err := service.NewAffiliateRewardService(
		NewAffiliateRewardRepository(client, integrationDB),
	).ProcessFirstPaidPurchase(ctx, service.AffiliateFirstPaidPurchaseInput{
		UserID:       user.ID,
		PurchaseType: service.AffiliatePurchaseBalanceTopup,
		SourceID:     paidSourceID,
		PurchaseKey:  "admin-roundtrip-self-paid:" + uuid.NewString(),
		AmountMicros: 100_000_000,
		OccurredAt:   time.Now(),
	})
	require.NoError(t, err)
	require.Equal(t, service.AffiliateSourcePolicyPartnerSelfUsage, rewardResult.SourcePolicy)
	policy, partnerID, customerRate, partnerRate := service.AffiliatePolicyFromPurchaseResult(true, rewardResult)

	require.NoError(t, userRepo.UpdateBalance(ctx, user.ID, 100))
	require.NoError(t, NewAffiliateConsumptionRepository(client).RecordBalanceLot(
		ctx,
		service.AffiliateBalanceLotInput{
			UserID:                   user.ID,
			SourceType:               service.AffiliateSourcePaidTopup,
			SourceID:                 paidSourceID,
			SourceKey:                "admin-roundtrip-self-lot:" + uuid.NewString(),
			AmountMicros:             100_000_000,
			AffiliatePolicy:          policy,
			DirectPartnerID:          partnerID,
			CustomerRebateRateBPS:    customerRate,
			PartnerCommissionRateBPS: partnerRate,
			OccurredAt:               time.Now(),
		},
	))

	usageLogID := time.Now().UnixNano()
	result, err := NewUsageBillingRepository(client, integrationDB).Apply(ctx, &service.UsageBillingCommand{
		RequestID:   uuid.NewString(),
		APIKeyID:    apiKey.ID,
		UsageLogID:  usageLogID,
		UserID:      user.ID,
		BalanceCost: 100,
	})
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Equal(t, int64(10_000_000), result.AffiliateAgentCommissionMicros)
	require.NotNil(t, result.NewBalance)
	require.InDelta(t, 0, *result.NewBalance, 0.000001)

	var cashMicros int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT amount_micros
		FROM agent_cash_commission_entries
		WHERE agent_id = $1
			AND consumer_user_id = $1
			AND source_type = 'confirmed_consumption'
	`, user.ID).Scan(&cashMicros))
	require.Equal(t, int64(10_000_000), cashMicros)
}

func TestUsageBillingPartnerSelfMonthlyCommissionUsesConfirmedSaleValue(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-self-monthly-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	limit := 100.0
	group := mustCreateGroup(t, client, &service.Group{
		Name:             "usage-self-monthly-" + uuid.NewString(),
		Platform:         service.PlatformOpenAI,
		SubscriptionType: service.SubscriptionTypeCredit,
		MonthlyLimitUSD:  &limit,
	})
	subscription := mustCreateSubscription(t, client, &service.UserSubscription{
		UserID:  user.ID,
		GroupID: group.ID,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID:  user.ID,
		GroupID: &group.ID,
		Key:     "sk-usage-self-monthly-" + uuid.NewString(),
		Name:    "self-monthly",
	})
	createSelfCommissionIntegrationPartner(t, ctx, user.ID)
	setAffiliateProgramLiveForIntegrationTest(t, ctx)

	rewardResult, err := service.NewAffiliateRewardService(
		NewAffiliateRewardRepository(client, integrationDB),
	).ProcessFirstPaidPurchase(ctx, service.AffiliateFirstPaidPurchaseInput{
		UserID:       user.ID,
		PurchaseType: service.AffiliatePurchaseMonthlyPayment,
		SourceID:     92001,
		PurchaseKey:  "self-monthly:" + uuid.NewString(),
		AmountMicros: 100_000_000,
		OccurredAt:   time.Now(),
	})
	require.NoError(t, err)
	policy, partnerID, customerRate, partnerRate := service.AffiliatePolicyFromPurchaseResult(true, rewardResult)
	require.Equal(t, service.AffiliateSourcePolicyPartnerSelfUsage, policy)
	require.NoError(t, NewAffiliateConsumptionRepository(client).RecordMonthlyEntitlement(
		ctx,
		service.AffiliateMonthlyEntitlementInput{
			UserID:                   user.ID,
			SourceType:               service.AffiliateSourcePaidTopup,
			SourceID:                 92001,
			SourceKey:                "self-monthly-cycle:" + uuid.NewString(),
			ProductCode:              "integration-self-monthly",
			SalePriceMicros:          100_000_000,
			CreditLimitMicros:        100_000_000,
			AffiliatePolicy:          policy,
			DirectPartnerID:          partnerID,
			CustomerRebateRateBPS:    customerRate,
			PartnerCommissionRateBPS: partnerRate,
			PricingTableVersion:      service.AffiliateCommercialPricingTableVersionV3,
			StartsAt:                 time.Now().Add(-time.Minute),
			EndsAt:                   time.Now().AddDate(0, 1, 0),
			Subscriptions: []service.AffiliateMonthlySubscription{{
				UserSubscriptionID: subscription.ID,
				GroupID:            group.ID,
			}},
		},
	))

	usageLogID := time.Now().UnixNano()
	result, err := NewUsageBillingRepository(client, integrationDB).Apply(
		ctx,
		&service.UsageBillingCommand{
			RequestID:        uuid.NewString(),
			APIKeyID:         apiKey.ID,
			UsageLogID:       usageLogID,
			UserID:           user.ID,
			SubscriptionID:   &subscription.ID,
			SubscriptionCost: 25,
		},
	)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Equal(t, int64(25_000_000), result.MonthlyConfirmedMicros)
	require.Zero(t, result.AffiliateCustomerRebateMicros)
	require.Equal(t, int64(2_500_000), result.AffiliateAgentCommissionMicros)

	var (
		eventPolicy        string
		sourceAmountMicros int64
		cashMicros         int64
	)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT
			event.affiliate_policy,
			cash.source_amount_micros,
			cash.amount_micros
		FROM affiliate_performance_events event
		JOIN agent_cash_commission_entries cash
			ON cash.source_type='confirmed_consumption'
			AND cash.source_id=event.id
		WHERE event.user_id=$1
			AND event.source_id=$2
			AND event.source_type='monthly_usage'
	`, user.ID, usageLogID).Scan(
		&eventPolicy,
		&sourceAmountMicros,
		&cashMicros,
	))
	require.Equal(t, service.AffiliateSourcePolicyPartnerSelfUsage, eventPolicy)
	require.Equal(t, int64(25_000_000), sourceAmountMicros)
	require.Equal(t, int64(2_500_000), cashMicros)

	secondResult, err := NewUsageBillingRepository(client, integrationDB).Apply(
		ctx,
		&service.UsageBillingCommand{
			RequestID:        uuid.NewString(),
			APIKeyID:         apiKey.ID,
			UsageLogID:       time.Now().UnixNano(),
			UserID:           user.ID,
			SubscriptionID:   &subscription.ID,
			SubscriptionCost: 75,
		},
	)
	require.NoError(t, err)
	require.Equal(t, int64(75_000_000), secondResult.MonthlyConfirmedMicros)
	require.Equal(t, int64(7_500_000), secondResult.AffiliateAgentCommissionMicros)
	var totalCashMicros int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(amount_micros), 0)::bigint
		FROM agent_cash_commission_entries
		WHERE agent_id=$1
			AND consumer_user_id=$1
			AND entry_type='earned'
			AND metadata ->> 'attribution_policy'='PARTNER_SELF_USAGE'
	`, user.ID).Scan(&totalCashMicros))
	require.Equal(t, int64(10_000_000), totalCashMicros, "monthly cash must cap at 10% of sale price")
}

func TestUsageBillingPartnerSelfCommissionRespectsRiskHoldAndTermination(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-self-risk-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	createSelfCommissionIntegrationPartner(t, ctx, user.ID)
	setAffiliateProgramLiveForIntegrationTest(t, ctx)

	record := func(key string) usageBillingAffiliateSettlement {
		t.Helper()
		tx, err := integrationDB.BeginTx(ctx, nil)
		require.NoError(t, err)
		settlement, err := recordUsageBillingPerformanceEvent(
			ctx,
			tx,
			user.ID,
			time.Now().UnixNano(),
			"balance_usage",
			key,
			10_000_000,
			service.AffiliateSourcePolicyPartnerSelfUsage,
			user.ID,
			0,
			service.AffiliateAgentPoolRateBPS,
			time.Now(),
		)
		require.NoError(t, err)
		require.NoError(t, tx.Commit())
		return settlement
	}

	posted := record("self-risk-posted:" + uuid.NewString())
	require.Equal(t, int64(1_000_000), posted.AgentCommissionMicros)

	_, err := integrationDB.ExecContext(ctx, `
		UPDATE agent_principals
		SET risk_status='review', updated_at=NOW()
		WHERE agent_id=$1
	`, user.ID)
	require.NoError(t, err)
	held := record("self-risk-held:" + uuid.NewString())
	require.Zero(t, held.AgentCommissionMicros)

	var heldCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM agent_cash_commission_entries
		WHERE agent_id=$1
			AND consumer_user_id=$1
			AND posting_status='risk_hold'
			AND metadata ->> 'attribution_policy'='PARTNER_SELF_USAGE'
	`, user.ID).Scan(&heldCount))
	require.Equal(t, 1, heldCount)

	_, err = integrationDB.ExecContext(ctx, `
		UPDATE agent_principals
		SET status='terminated', terminated_at=NOW(), updated_at=NOW()
		WHERE agent_id=$1
	`, user.ID)
	require.NoError(t, err)
	terminated := record("self-risk-terminated:" + uuid.NewString())
	require.Zero(t, terminated.AgentCommissionMicros)

	var earnedCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM agent_cash_commission_entries
		WHERE agent_id=$1
			AND consumer_user_id=$1
			AND entry_type='earned'
			AND metadata ->> 'attribution_policy'='PARTNER_SELF_USAGE'
	`, user.ID).Scan(&earnedCount))
	require.Equal(t, 2, earnedCount, "terminated partner must not receive another cash entry")
}

func createSelfCommissionIntegrationPartner(
	t *testing.T,
	ctx context.Context,
	userID int64,
) {
	t.Helper()
	_, err := integrationDB.ExecContext(ctx, `
		INSERT INTO agent_principals (
			agent_id, status, risk_status, qualified_at, activated_at
		)
		VALUES ($1, 'active', 'clear', NOW(), NOW())
	`, userID)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO affiliate_agent_self_commission_policies (
			agent_id, enabled, rate_bps, effective_at, revision, reason
		)
		VALUES ($1, TRUE, 1000, NOW() - INTERVAL '5 minutes', 1, 'integration test')
	`, userID)
	require.NoError(t, err)
}
