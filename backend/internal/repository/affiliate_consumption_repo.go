package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

type affiliateConsumptionRepository struct {
	client *dbent.Client
}

func NewAffiliateConsumptionRepository(client *dbent.Client) service.AffiliateConsumptionRepository {
	return &affiliateConsumptionRepository{client: client}
}

func (r *affiliateConsumptionRepository) RecordBalanceLot(ctx context.Context, input service.AffiliateBalanceLotInput) error {
	if input.UserID <= 0 || input.AmountMicros <= 0 || strings.TrimSpace(input.SourceKey) == "" {
		return errors.New("invalid affiliate balance lot input")
	}
	if err := validateAffiliateSourcePolicy(
		input.UserID,
		input.AffiliatePolicy,
		input.DirectPartnerID,
		input.CustomerRebateRateBPS,
		input.PartnerCommissionRateBPS,
	); err != nil {
		return err
	}
	client, err := r.clientForContext(ctx)
	if err != nil {
		return err
	}
	occurredAt := input.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now()
	}
	var result sql.Result
	return client.Driver().Exec(ctx, `
		INSERT INTO balance_lots (
			user_id, source_type, source_id, source_key,
			original_amount_micros, remaining_amount_micros,
			affiliate_eligible, affiliate_policy, direct_partner_id,
			customer_rebate_rate_bps, partner_commission_rate_bps,
			occurred_at
		)
			VALUES (
				$1, $2, NULLIF($3, 0), $4, $5, $5,
				$6, $7, NULLIF($8, 0), $9, $10, LEAST($11, CURRENT_TIMESTAMP)
			)
		ON CONFLICT (source_key) DO NOTHING
	`, []any{
		input.UserID,
		input.SourceType,
		input.SourceID,
		input.SourceKey,
		input.AmountMicros,
		service.AffiliatePolicyTracksConsumption(input.AffiliatePolicy),
		input.AffiliatePolicy,
		input.DirectPartnerID,
		input.CustomerRebateRateBPS,
		input.PartnerCommissionRateBPS,
		occurredAt,
	}, &result)
}

func (r *affiliateConsumptionRepository) RecordMonthlyEntitlement(ctx context.Context, input service.AffiliateMonthlyEntitlementInput) error {
	if input.UserID <= 0 ||
		input.CreditLimitMicros <= 0 ||
		strings.TrimSpace(input.SourceKey) == "" ||
		!input.EndsAt.After(input.StartsAt) ||
		len(input.Subscriptions) == 0 {
		return errors.New("invalid affiliate monthly entitlement input")
	}
	if err := validateAffiliateSourcePolicy(
		input.UserID,
		input.AffiliatePolicy,
		input.DirectPartnerID,
		input.CustomerRebateRateBPS,
		input.PartnerCommissionRateBPS,
	); err != nil {
		return err
	}
	if strings.TrimSpace(input.PricingTableVersion) == "" {
		input.PricingTableVersion = "legacy"
	}
	client, err := r.clientForContext(ctx)
	if err != nil {
		return err
	}

	var cycleID int64
	rows := &entsql.Rows{}
	if err := client.Driver().Query(ctx, `
		INSERT INTO monthly_entitlement_cycles (
			user_id, source_type, source_id, source_key, product_code,
			sale_price_micros, credit_limit_micros,
			affiliate_eligible, affiliate_policy, direct_partner_id,
			customer_rebate_rate_bps, partner_commission_rate_bps,
			pricing_table_version, starts_at, ends_at
		)
		VALUES (
			$1, $2, NULLIF($3, 0), $4, $5, $6, $7,
				$8, $9, NULLIF($10, 0), $11, $12, $13,
				CASE
					WHEN $14 <= CURRENT_TIMESTAMP + INTERVAL '1 second' THEN CURRENT_TIMESTAMP
					ELSE $14
				END,
				$15
			)
		ON CONFLICT (source_key) DO UPDATE
		SET source_key = EXCLUDED.source_key
		RETURNING id
	`, []any{
		input.UserID,
		input.SourceType,
		input.SourceID,
		input.SourceKey,
		input.ProductCode,
		input.SalePriceMicros,
		input.CreditLimitMicros,
		service.AffiliatePolicyTracksConsumption(input.AffiliatePolicy),
		input.AffiliatePolicy,
		input.DirectPartnerID,
		input.CustomerRebateRateBPS,
		input.PartnerCommissionRateBPS,
		input.PricingTableVersion,
		input.StartsAt,
		input.EndsAt,
	}, rows); err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return errors.New("affiliate monthly entitlement insert returned no row")
	}
	if err := rows.Scan(&cycleID); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}

	for _, subscription := range input.Subscriptions {
		if subscription.UserSubscriptionID <= 0 || subscription.GroupID <= 0 {
			return errors.New("invalid affiliate monthly entitlement subscription")
		}
		var result sql.Result
		if err := client.Driver().Exec(ctx, `
			INSERT INTO monthly_entitlement_cycle_subscriptions (
				cycle_id, user_subscription_id, group_id
			)
			VALUES ($1, $2, $3)
			ON CONFLICT (cycle_id, user_subscription_id) DO NOTHING
		`, []any{
			cycleID,
			subscription.UserSubscriptionID,
			subscription.GroupID,
		}, &result); err != nil {
			return err
		}
	}
	return nil
}

func validateAffiliateSourcePolicy(
	userID int64,
	policy string,
	directPartnerID int64,
	customerRateBPS int32,
	partnerRateBPS int32,
) error {
	switch policy {
	case service.AffiliateSourcePolicyNone:
		if customerRateBPS != 0 || partnerRateBPS != 0 {
			return errors.New("non-affiliate source cannot carry reward rates")
		}
	case service.AffiliateSourcePolicyOrdinaryFirstPaid:
		if directPartnerID <= 0 || customerRateBPS != 0 || partnerRateBPS != 0 {
			return errors.New("ordinary affiliate source requires one inviter and no usage rates")
		}
	case service.AffiliateSourcePolicyPartnerUsage:
		if directPartnerID <= 0 ||
			directPartnerID == userID ||
			customerRateBPS < 0 ||
			partnerRateBPS < 0 ||
			customerRateBPS+partnerRateBPS != service.AffiliateAgentPoolRateBPS {
			return errors.New("partner affiliate source requires one fixed 10 percent pool")
		}
	case service.AffiliateSourcePolicyPartnerSelfUsage:
		if directPartnerID <= 0 ||
			directPartnerID != userID ||
			customerRateBPS != 0 ||
			partnerRateBPS != service.AffiliateAgentPoolRateBPS {
			return errors.New("partner self affiliate source requires the consumer's fixed 10 percent cash pool")
		}
	default:
		return errors.New("invalid affiliate source policy")
	}
	return nil
}

func (r *affiliateConsumptionRepository) clientForContext(ctx context.Context) (*dbent.Client, error) {
	if client, ok := transactionClientFromContext(ctx); ok {
		return client, nil
	}
	if r == nil || r.client == nil {
		return nil, errors.New("affiliate consumption repository client is nil")
	}
	return r.client, nil
}
