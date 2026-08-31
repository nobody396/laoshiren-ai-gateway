//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAffiliateWalletRepository_OnDemandWithdrawalFailureAndConversion(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewAffiliateWalletRepository(integrationDB)
	walletService := service.NewAffiliateWalletService(repo)
	admin := mustCreateUser(t, client, &service.User{
		Email: fmt.Sprintf("wallet-admin-%d@example.com", time.Now().UnixNano()),
		Role:  service.RoleAdmin,
	})
	agent := createActiveAffiliatePaymentAgent(t, ctx, client, "wallet-agent")
	_, err := integrationDB.ExecContext(ctx, `
		INSERT INTO agent_payment_profiles (
			agent_id, alipay_real_name, alipay_account,
			alipay_qr_object_key, alipay_qr_content_type,
				alipay_qr_original_filename, alipay_qr_size,
				identity_fingerprint_hash, verification_status,
				verified_at, verified_by,
				privacy_consent_version, privacy_consented_at
		)
		VALUES (
			$1, '钱包测试', 'wallet@example.com',
			'agent-payment-qrcodes/test/wallet.png', 'image/png',
			'wallet.png', 128,
				$2, 'verified', NOW(), $3,
				$4, NOW()
			)
		`, agent.ID, fmt.Sprintf("%064d", agent.ID), admin.ID, service.AgentPaymentPrivacyNoticeVersion)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO agent_cash_commission_entries (
			agent_id, entry_type, amount_micros,
			posting_status, source_type, source_id,
			idempotency_key, occurred_at
		)
		VALUES (
			$1, 'earned', 200000000,
			'posted', 'integration', 1,
			$2, NOW()
		)
	`, agent.ID, fmt.Sprintf("wallet-earned:%d", agent.ID))
	require.NoError(t, err)

	wallet, err := walletService.GetWallet(ctx, agent.ID)
	require.NoError(t, err)
	require.Equal(t, int64(200_000_000), wallet.AvailableCashMicros)
	require.True(t, wallet.PaymentProfileVerified)
	require.True(t, wallet.CanWithdraw)
	require.Equal(t, "¥", wallet.CashAssetSymbol)
	require.Equal(t, "⚡", wallet.CreditAssetSymbol)

	failedRequest, err := walletService.RequestWithdrawal(
		ctx,
		agent.ID,
		100_000_000,
		"wallet-withdraw-failure",
	)
	require.NoError(t, err)
	require.Equal(t, "processing", failedRequest.Status)
	require.WithinDuration(t, failedRequest.RequestedAt.Add(24*time.Hour), failedRequest.DueAt, time.Second)
	idempotent, err := walletService.RequestWithdrawal(
		ctx,
		agent.ID,
		100_000_000,
		"wallet-withdraw-failure",
	)
	require.NoError(t, err)
	require.Equal(t, failedRequest.ID, idempotent.ID)
	_, err = walletService.RequestWithdrawal(
		ctx,
		agent.ID,
		100_000_000,
		"wallet-withdraw-second-processing",
	)
	require.ErrorIs(t, err, service.ErrAffiliateWithdrawalAlreadyProcessing)
	_, err = walletService.GetWithdrawalQRCodeFile(ctx, failedRequest.ID, admin.ID)
	require.NoError(t, err)
	var qrAccessCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM agent_payment_qr_access_events
		WHERE withdrawal_id = $1
			AND accessor_user_id = $2
			AND access_context = 'withdrawal_snapshot'
	`, failedRequest.ID, admin.ID).Scan(&qrAccessCount))
	require.Equal(t, 1, qrAccessCount)

	paymentRepo := NewCommissionRepository(client, integrationDB).(service.AgentPaymentRepository)
	err = paymentRepo.UpsertAgentPaymentProfile(ctx, &service.AgentPaymentProfile{
		AgentID:               agent.ID,
		AlipayRealName:        "处理中不可修改",
		AlipayAccount:         "locked@example.com",
		PrivacyConsentVersion: service.AgentPaymentPrivacyNoticeVersion,
		PrivacyConsentedAt:    func() *time.Time { value := time.Now(); return &value }(),
	})
	require.ErrorIs(t, err, service.ErrAgentPaymentProfileLocked)

	wallet, err = walletService.GetWallet(ctx, agent.ID)
	require.NoError(t, err)
	require.Equal(t, int64(100_000_000), wallet.AvailableCashMicros)
	require.Equal(t, int64(100_000_000), wallet.ProcessingWithdrawalMicros)

	failedRequest, err = walletService.FailWithdrawal(
		ctx,
		failedRequest.ID,
		admin.ID,
		"收款码无法识别",
	)
	require.NoError(t, err)
	require.Equal(t, "failed", failedRequest.Status)
	wallet, err = walletService.GetWallet(ctx, agent.ID)
	require.NoError(t, err)
	require.Equal(t, int64(200_000_000), wallet.AvailableCashMicros)
	require.Zero(t, wallet.ProcessingWithdrawalMicros)
	visible, err := walletService.ListWithdrawals(ctx, agent.ID)
	require.NoError(t, err)
	require.Empty(t, visible, "failed requests are communicated as notices, not a third user-facing status")
	notices, err := walletService.ListNotices(ctx, agent.ID)
	require.NoError(t, err)
	require.NotEmpty(t, notices)
	require.Equal(t, "withdrawal_failed", notices[0].NoticeType)
	require.NoError(t, walletService.MarkNoticeRead(ctx, agent.ID, notices[0].ID))

	paidRequest, err := walletService.RequestWithdrawal(
		ctx,
		agent.ID,
		150_000_000,
		"wallet-withdraw-paid",
	)
	require.NoError(t, err)
	paidRequest, err = walletService.CompleteWithdrawal(
		ctx,
		paidRequest.ID,
		admin.ID,
		"alipay-trade-001",
	)
	require.NoError(t, err)
	require.Equal(t, "paid", paidRequest.Status)
	require.NotNil(t, paidRequest.PaidAt)

	var requestedEvents, paidEvents, failedEvents int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE event_type = 'requested'),
			COUNT(*) FILTER (WHERE event_type = 'paid'),
			COUNT(*) FILTER (WHERE event_type = 'failed')
		FROM agent_withdrawal_events
		WHERE agent_id = $1
	`, agent.ID).Scan(&requestedEvents, &paidEvents, &failedEvents))
	require.Equal(t, 2, requestedEvents)
	require.Equal(t, 1, paidEvents)
	require.Equal(t, 1, failedEvents)

	conversion, err := walletService.Convert(
		ctx,
		agent.ID,
		50_000_000,
		"wallet-conversion-001",
	)
	require.NoError(t, err)
	require.Equal(t, int64(50_000_000), conversion.CashAmountMicros)
	require.Equal(t, int64(60_000_000), conversion.CreditAmountMicros)
	require.Equal(t, int32(1200), conversion.MultiplierMillis)
	secondConversion, err := walletService.Convert(
		ctx,
		agent.ID,
		50_000_000,
		"wallet-conversion-001",
	)
	require.NoError(t, err)
	require.Equal(t, conversion.ID, secondConversion.ID)

	wallet, err = walletService.GetWallet(ctx, agent.ID)
	require.NoError(t, err)
	require.Zero(t, wallet.AvailableCashMicros)
	var balance float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT balance FROM users WHERE id = $1
	`, agent.ID).Scan(&balance))
	require.InDelta(t, 60, balance, 0.000001)
	var conversionLotEligible bool
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT affiliate_eligible
		FROM balance_lots
		WHERE source_type = 'commission_conversion'
			AND source_id = $1
	`, conversion.ID).Scan(&conversionLotEligible))
	require.False(t, conversionLotEligible)
}

func TestAffiliateWalletRepository_PlatformPurchaseDebitsCashWithoutCreatingCredit(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewAffiliateWalletRepository(integrationDB)
	walletService := service.NewAffiliateWalletService(repo)
	agent := createActiveAffiliatePaymentAgent(t, ctx, client, "wallet-purchase-agent")
	_, err := integrationDB.ExecContext(ctx, `
		INSERT INTO agent_cash_commission_entries (
			agent_id, entry_type, amount_micros,
			posting_status, source_type, source_id,
			idempotency_key, occurred_at
		)
		VALUES (
			$1, 'earned', 300000000,
			'posted', 'integration', 1,
			$2, NOW()
		)
	`, agent.ID, fmt.Sprintf("wallet-purchase-earned:%d", agent.ID))
	require.NoError(t, err)

	var balanceBefore float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT balance FROM users WHERE id = $1
	`, agent.ID).Scan(&balanceBefore))

	purchase, err := walletService.Purchase(
		ctx,
		agent.ID,
		255_000_000,
		service.AffiliateCommissionMachineOperatorID,
		service.AffiliateCommissionPurchaseKindMonthlyCard,
		"plus",
		"monthly-new-cycle-integration-plus",
		"Plus monthly card paid from affiliate commission",
		"affiliate-purchase-integration-plus",
	)
	require.NoError(t, err)
	require.Equal(t, int64(255_000_000), purchase.AmountMicros)
	require.Equal(t, int64(45_000_000), purchase.RemainingCashMicros)
	require.Equal(t, service.AffiliateCommissionPurchaseKindMonthlyCard, purchase.PurchaseKind)
	require.Equal(t, "plus", purchase.ProductCode)
	require.Equal(t, "monthly-new-cycle-integration-plus", purchase.ExternalReference)
	require.Equal(t, service.AffiliateCommissionMachineOperatorID, purchase.OperatorID)

	idempotent, err := walletService.Purchase(
		ctx,
		agent.ID,
		255_000_000,
		service.AffiliateCommissionMachineOperatorID,
		service.AffiliateCommissionPurchaseKindMonthlyCard,
		"plus",
		"monthly-new-cycle-integration-plus",
		"Plus monthly card paid from affiliate commission",
		"affiliate-purchase-integration-plus",
	)
	require.NoError(t, err)
	require.Equal(t, purchase.ID, idempotent.ID)

	sameReference, err := walletService.Purchase(
		ctx,
		agent.ID,
		255_000_000,
		service.AffiliateCommissionMachineOperatorID,
		service.AffiliateCommissionPurchaseKindMonthlyCard,
		"plus",
		"monthly-new-cycle-integration-plus",
		"Plus monthly card paid from affiliate commission",
		"affiliate-purchase-integration-plus-retry",
	)
	require.NoError(t, err)
	require.Equal(t, purchase.ID, sameReference.ID)

	_, err = walletService.Purchase(
		ctx,
		agent.ID,
		254_000_000,
		service.AffiliateCommissionMachineOperatorID,
		service.AffiliateCommissionPurchaseKindMonthlyCard,
		"plus",
		"monthly-new-cycle-integration-plus",
		"Plus monthly card paid from affiliate commission",
		"affiliate-purchase-integration-plus",
	)
	require.ErrorIs(t, err, service.ErrAffiliatePurchaseIdempotencyConflict)

	_, err = walletService.Purchase(
		ctx,
		agent.ID,
		100_000_000,
		service.AffiliateCommissionMachineOperatorID,
		service.AffiliateCommissionPurchaseKindMonthlyCard,
		"plus",
		"monthly-new-cycle-integration-plus-second",
		"second purchase",
		"affiliate-purchase-integration-plus-second",
	)
	require.ErrorIs(t, err, service.ErrAffiliateInsufficientCash)

	wallet, err := walletService.GetWallet(ctx, agent.ID)
	require.NoError(t, err)
	require.Equal(t, int64(45_000_000), wallet.AvailableCashMicros)
	require.Equal(t, int64(300_000_000), wallet.LifetimeEarnedMicros)

	var balanceAfter float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT balance FROM users WHERE id = $1
	`, agent.ID).Scan(&balanceAfter))
	require.InDelta(t, balanceBefore, balanceAfter, 0.000001)

	var entryType, sourceType, productCode, externalReference string
	var ledgerAmount, ledgerOperatorID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT
			entry_type,
			source_type,
			amount_micros,
			metadata ->> 'product_code',
			metadata ->> 'external_reference',
			(metadata ->> 'operator_id')::bigint
		FROM agent_cash_commission_entries
		WHERE id = $1
	`, purchase.ID).Scan(
		&entryType,
		&sourceType,
		&ledgerAmount,
		&productCode,
		&externalReference,
		&ledgerOperatorID,
	))
	require.Equal(t, "platform_purchase", entryType)
	require.Equal(t, "platform_purchase", sourceType)
	require.Equal(t, int64(-255_000_000), ledgerAmount)
	require.Equal(t, "plus", productCode)
	require.Equal(t, "monthly-new-cycle-integration-plus", externalReference)
	require.Equal(t, service.AffiliateCommissionMachineOperatorID, ledgerOperatorID)

	notices, err := walletService.ListNotices(ctx, agent.ID)
	require.NoError(t, err)
	require.NotEmpty(t, notices)
	require.Equal(t, "commission_purchase", notices[0].NoticeType)
	require.Equal(t, purchase.ID, *notices[0].SourceID)
	require.Contains(t, notices[0].Message, "¥255.00")
	require.Contains(t, notices[0].Message, "¥45.00")
}
