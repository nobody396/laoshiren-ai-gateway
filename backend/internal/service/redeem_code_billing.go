package service

import "time"

type RedeemCodeBillingFilters struct {
	Search            string
	Purpose           string
	SalesStatus       string
	RedeemStatus      string
	BatchID           *int64
	AmountMin         *float64
	AmountMax         *float64
	UsedStartTime     *time.Time
	UsedEndTime       *time.Time
	CreatedStartTime  *time.Time
	CreatedEndTime    *time.Time
	IncludeNonRevenue bool
}

type RedeemCodeBillingResult struct {
	Items    []RedeemCodeBillingItem  `json:"items"`
	Summary  RedeemCodeBillingSummary `json:"summary"`
	Total    int64                    `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
	Pages    int                      `json:"pages"`
}

type RedeemCodeBillingItem struct {
	ID                 int64      `json:"id"`
	Code               string     `json:"code"`
	Type               string     `json:"type"`
	Value              float64    `json:"value"`
	Purpose            string     `json:"purpose"`
	SalesStatus        string     `json:"sales_status"`
	RedeemStatus       string     `json:"redeem_status"`
	BatchID            *int64     `json:"batch_id"`
	BatchName          string     `json:"batch_name"`
	SoldAt             *time.Time `json:"sold_at"`
	SoldToNote         string     `json:"sold_to_note"`
	ExternalOrderNo    string     `json:"external_order_no"`
	ExternalOrderURL   string     `json:"external_order_url"`
	InternalNotes      string     `json:"internal_notes"`
	Notes              string     `json:"notes"`
	UsedBy             *int64     `json:"used_by"`
	UsedByEmail        string     `json:"used_by_email"`
	UsedByUsername     string     `json:"used_by_username"`
	UsedAt             *time.Time `json:"used_at"`
	LedgerID           *int64     `json:"ledger_id"`
	LedgerMatched      bool       `json:"ledger_matched"`
	LedgerDelta        *float64   `json:"ledger_delta"`
	CurrentUserBalance *float64   `json:"current_user_balance"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type RedeemCodeBillingSummary struct {
	SaleFaceValue              float64 `json:"sale_face_value"`
	SoldFaceValue              float64 `json:"sold_face_value"`
	RedeemedSaleAmount         float64 `json:"redeemed_sale_amount"`
	SoldUnredeemedFaceValue    float64 `json:"sold_unredeemed_face_value"`
	GiftRedeemedAmount         float64 `json:"gift_redeemed_amount"`
	CompensationRedeemedAmount float64 `json:"compensation_redeemed_amount"`
	InternalTestRedeemedAmount float64 `json:"internal_test_redeemed_amount"`
	LedgerMissingCount         int64   `json:"ledger_missing_count"`
	TotalCodes                 int64   `json:"total_codes"`
	UsedCodes                  int64   `json:"used_codes"`
	UnusedCodes                int64   `json:"unused_codes"`
}
