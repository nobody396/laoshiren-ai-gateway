package service

import (
	"context"
	"time"
)

const (
	GPTImageTaskStatusSubmitted  = "submitted"
	GPTImageTaskStatusProcessing = "processing"
	GPTImageTaskStatusCompleted  = "completed"
	GPTImageTaskStatusFailed     = "failed"

	GPTImageStorageStatusPending   = "pending"
	GPTImageStorageStatusUploading = "uploading"
	GPTImageStorageStatusStored    = "stored"
	GPTImageStorageStatusFailed    = "failed"
	GPTImageBillingStatusPending   = "pending"
	GPTImageBillingStatusBilling   = "billing"
	GPTImageBillingStatusBilled    = "billed"
	GPTImageBillingStatusFailed    = "failed"
)

type GPTImageTask struct {
	ID                 int64
	TaskID             string
	UserID             int64
	APIKeyID           int64
	AccountID          int64
	Model              string
	UpstreamModel      string
	Resolution         string
	Size               string
	Status             string
	StorageStatus      string
	BillingStatus      string
	S3ObjectKeys       []string
	MediaToken         string
	ImageCount         int
	InboundEndpoint    string
	UpstreamEndpoint   string
	UserAgent          string
	IPAddress          string
	RequestPayloadHash string
	ErrorMessage       string
	BilledAt           *time.Time
	CompletedAt        *time.Time
	FailedAt           *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type GPTImageTaskRepository interface {
	UpsertSubmitted(ctx context.Context, task *GPTImageTask) error
	GetByTaskID(ctx context.Context, taskID string) (*GPTImageTask, error)
	ListStoredByTaskIDs(ctx context.Context, taskIDs []string) (map[string]*GPTImageTask, error)
	ListPendingSettlement(ctx context.Context, limit int) ([]*GPTImageTask, error)
	UpdateUpstreamStatus(ctx context.Context, taskID, status, errorMessage string) error
	TryClaimStorage(ctx context.Context, taskID string) (*GPTImageTask, bool, error)
	MarkStorageStored(ctx context.Context, taskID string, objectKeys []string, imageCount int, mediaToken string) error
	MarkStorageFailed(ctx context.Context, taskID, errorMessage string) error
	TryClaimBilling(ctx context.Context, taskID string) (*GPTImageTask, bool, error)
	MarkBilled(ctx context.Context, taskID string) error
	ReleaseBillingClaim(ctx context.Context, taskID string, errorMessage string) error
}
