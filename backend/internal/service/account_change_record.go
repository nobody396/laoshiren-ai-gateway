package service

import "time"

const (
	AccountChangeAssetBalance      = "balance"
	AccountChangeAssetConcurrency  = "concurrency"
	AccountChangeAssetSubscription = "subscription"

	AccountChangeReasonRedeemCode      = "redeem_code"
	AccountChangeReasonTopup           = "topup"
	AccountChangeReasonTopupPromotion  = "topup_promotion"
	AccountChangeReasonAdminAdjustment = "admin_adjustment"
	AccountChangeReasonFeedbackReward  = "feedback_reward"

	AccountChangeSourceRedeemCode       = "redeem_code"
	AccountChangeSourceTopupOrder       = "topup_order"
	AccountChangeSourceAdminManual      = "admin_manual"
	AccountChangeSourceLegacyRedeemCode = "legacy_redeem_code"
	AccountChangeSourceFeedback         = "feedback"

	AccountChangeDisplayTopup          = "topup"
	AccountChangeDisplayTopupPromotion = "topup_promotion"
)

type AccountChangeRecord struct {
	ID             int64
	UserID         int64
	AssetType      string
	Reason         string
	Delta          float64
	SourceType     string
	SourceID       *int64
	ReferenceNo    string
	OperatorUserID *int64
	Notes          string
	GroupID        *int64
	ValidityDays   int
	DedupeKey      *string
	CreatedAt      time.Time

	Group *Group
}

type AccountChangeRecordListFilters struct {
	DisplayType string
	Reasons     []string
}

func (r *AccountChangeRecord) DisplayType() string {
	if r == nil {
		return ""
	}

	switch r.Reason {
	case AccountChangeReasonTopup:
		return AccountChangeDisplayTopup
	case AccountChangeReasonTopupPromotion:
		return AccountChangeDisplayTopupPromotion
	case AccountChangeReasonAdminAdjustment:
		switch r.AssetType {
		case AccountChangeAssetBalance:
			return AdjustmentTypeAdminBalance
		case AccountChangeAssetConcurrency:
			return AdjustmentTypeAdminConcurrency
		}
	case AccountChangeReasonRedeemCode:
		switch r.AssetType {
		case AccountChangeAssetBalance:
			return RedeemTypeBalance
		case AccountChangeAssetConcurrency:
			return RedeemTypeConcurrency
		case AccountChangeAssetSubscription:
			return RedeemTypeSubscription
		}
	}

	return r.AssetType
}

func (r *AccountChangeRecord) ToRedeemCode() RedeemCode {
	usedAt := r.CreatedAt

	return RedeemCode{
		ID:           r.ID,
		Code:         r.ReferenceNo,
		Type:         r.DisplayType(),
		Value:        r.Delta,
		Status:       StatusUsed,
		UsedBy:       &r.UserID,
		UsedAt:       &usedAt,
		Notes:        r.Notes,
		CreatedAt:    r.CreatedAt,
		GroupID:      r.GroupID,
		ValidityDays: r.ValidityDays,
		Group:        r.Group,
	}
}
