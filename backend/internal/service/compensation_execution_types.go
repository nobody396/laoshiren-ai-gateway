package service

import "time"

const (
	SettingKeyCompensationExecutionEnabled  = "compensation_execution_enabled"
	CompensationDraftApprovalConfirmation   = "I_APPROVE_THIS_COMPENSATION_DRAFT"
	CompensationExecutionEnableConfirmation = "I_ENABLE_COMPENSATION_EXECUTION"
)

type CompensationApprovalCommand struct {
	DraftID      int64
	ApprovalKey  string
	Reason       string
	Confirmation string
	PreviewHash  string
	ActorUserID  int64
}
type CompensationDraftApproval struct {
	ID                  int64     `json:"id"`
	DraftID             int64     `json:"draft_id"`
	DraftEvidenceHash   string    `json:"draft_evidence_hash"`
	ApprovedPreviewHash string    `json:"approved_preview_hash"`
	ApprovalKey         string    `json:"approval_key"`
	ApprovalReason      string    `json:"approval_reason"`
	ApprovedByUserID    int64     `json:"approved_by_user_id"`
	ApprovedAt          time.Time `json:"approved_at"`
}
type CompensationExecutionSettingsCommand struct {
	Enabled      bool
	Confirmation string
	ActorUserID  int64
}
type CompensationExecution struct {
	ID             int64                         `json:"id"`
	ExecutionKey   string                        `json:"execution_key"`
	ApprovalID     int64                         `json:"approval_id"`
	DraftUserID    int64                         `json:"draft_user_id"`
	IncidentID     int64                         `json:"incident_id"`
	UserID         int64                         `json:"user_id"`
	BenefitChannel string                        `json:"benefit_channel"`
	AmountCNYFen   int64                         `json:"amount_cny_fen"`
	State          string                        `json:"state"`
	AttemptCount   int                           `json:"attempt_count"`
	LastError      string                        `json:"last_error,omitempty"`
	AppliedAt      *time.Time                    `json:"applied_at,omitempty"`
	VerifiedAt     *time.Time                    `json:"verified_at,omitempty"`
	Receipt        *CompensationExecutionReceipt `json:"receipt,omitempty"`
}
type CompensationExecutionReceipt struct {
	ID          int64          `json:"id"`
	ExecutionID int64          `json:"execution_id"`
	Payload     map[string]any `json:"payload"`
	PayloadHash string         `json:"payload_hash"`
	VerifiedAt  time.Time      `json:"verified_at"`
}
type CompensationExecutionBatch struct {
	DraftID        int64                   `json:"draft_id"`
	ApprovalID     int64                   `json:"approval_id"`
	State          string                  `json:"state"`
	Executions     []CompensationExecution `json:"executions"`
	VerifiedCount  int                     `json:"verified_count"`
	FailedCount    int                     `json:"failed_count"`
	NoticesCreated int                     `json:"notices_created"`
}
type CompensationExecutionPreview struct {
	DraftID             int64                       `json:"draft_id"`
	Latest              bool                        `json:"latest"`
	IncidentResolved    bool                        `json:"incident_resolved"`
	ShadowGateEligible  bool                        `json:"shadow_gate_eligible"`
	ExecutionEnabled    bool                        `json:"execution_enabled"`
	ApprovalID          *int64                      `json:"approval_id,omitempty"`
	TotalCNYFen         int64                       `json:"total_cny_fen"`
	BalanceCNYFen       int64                       `json:"balance_cny_fen"`
	BuilderPassCNYFen   int64                       `json:"builder_pass_cny_fen"`
	UserCount           int                         `json:"user_count"`
	Executable          bool                        `json:"executable"`
	Blockers            []string                    `json:"blockers"`
	NoticePreviews      []CompensationNoticePreview `json:"notice_previews"`
	ApprovalPreviewHash string                      `json:"approval_preview_hash"`
}
type CompensationNoticePreview struct {
	UserID                int64    `json:"user_id"`
	Services              []string `json:"services"`
	BalanceCNYFen         int64    `json:"balance_cny_fen"`
	BuilderPassCNYFen     int64    `json:"builder_pass_cny_fen"`
	FailedRequestsCharged bool     `json:"failed_requests_charged"`
	Body                  string   `json:"body"`
}
type ErroneousChargeRefundCommand struct {
	IdempotencyKey string
	UsageLogID     int64
	UserID         int64
	Reason         string
	ActorUserID    int64
}
type ErroneousChargeRefund struct {
	ID             int64     `json:"id"`
	IdempotencyKey string    `json:"idempotency_key"`
	UsageLogID     int64     `json:"usage_log_id"`
	UserID         int64     `json:"user_id"`
	AmountMicros   int64     `json:"amount_micros"`
	BillingType    int16     `json:"billing_type"`
	AssetType      string    `json:"asset_type"`
	AssetID        int64     `json:"asset_id"`
	Reason         string    `json:"reason"`
	CreatedAt      time.Time `json:"created_at"`
}
