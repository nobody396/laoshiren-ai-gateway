package service

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	dbuser "github.com/bozhouDev/DragonCode-sub2api/ent/user"
	"github.com/bozhouDev/DragonCode-sub2api/internal/domain"
	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
)

const (
	feedbackCreateLimit   = 10
	feedbackCreateWindow  = time.Hour
	feedbackMaxImages     = 5
	feedbackMaxBodyLen    = 5000
	feedbackMaxContextLen = 2000
)

type FeedbackService struct {
	repo                 FeedbackRepository
	userRepo             UserRepository
	settingService       *SettingService
	emailQueue           *EmailQueueService
	rateLimitCache       FeedbackRateLimitCache
	imageStorage         FeedbackImageStorage
	affiliateConsumption AffiliateConsumptionRepository
	entClient            *dbent.Client
	billingCache         BillingCache
	authInvalidator      APIKeyAuthCacheInvalidator
}

func NewFeedbackService(
	repo FeedbackRepository,
	userRepo UserRepository,
	settingService *SettingService,
	emailQueue *EmailQueueService,
	rateLimitCache FeedbackRateLimitCache,
	imageStorage FeedbackImageStorage,
	affiliateConsumption AffiliateConsumptionRepository,
	entClient *dbent.Client,
	billingCache BillingCache,
	authInvalidator APIKeyAuthCacheInvalidator,
) *FeedbackService {
	return &FeedbackService{
		repo:                 repo,
		userRepo:             userRepo,
		settingService:       settingService,
		emailQueue:           emailQueue,
		rateLimitCache:       rateLimitCache,
		imageStorage:         imageStorage,
		affiliateConsumption: affiliateConsumption,
		entClient:            entClient,
		billingCache:         billingCache,
		authInvalidator:      authInvalidator,
	}
}

func (s *FeedbackService) Create(ctx context.Context, userID int64, input CreateFeedbackInput) (*Feedback, error) {
	if err := s.enforceCreateRateLimit(ctx, userID); err != nil {
		return nil, err
	}

	content, images, requestID, err := normalizeFeedbackSubmission(input.Content, input.Images, input.RequestID)
	if err != nil {
		return nil, err
	}
	contact, err := s.registeredFeedbackContact(ctx, userID)
	if err != nil {
		return nil, err
	}
	category := domain.FeedbackCategoryOther
	title := deriveFeedbackTitle(content)

	feedback := &Feedback{
		UserID:           userID,
		Category:         category,
		Title:            title,
		Content:          content,
		Images:           images,
		Contact:          contact,
		RequestID:        requestID,
		Priority:         domain.DefaultFeedbackPriority(category),
		Status:           FeedbackStatusPending,
		TriageStatus:     domain.FeedbackTriageUnreviewed,
		RepairDifficulty: domain.FeedbackDifficultyUnknown,
		OwnerDecision:    domain.FeedbackDecisionPending,
		FixStatus:        domain.FeedbackFixNotStarted,
		ReplyCount:       0,
	}
	if err := s.createFeedbackWithEvent(ctx, feedback); err != nil {
		return nil, fmt.Errorf("create feedback: %w", err)
	}

	// Email is intentionally not sent automatically. A message-specific owner
	// approval is required; the local feedback skill handles preview/send later.
	return feedback, nil
}

func (s *FeedbackService) ListByUser(ctx context.Context, userID int64, params pagination.PaginationParams, filters FeedbackListFilters) ([]Feedback, *pagination.PaginationResult, error) {
	if filters.Status != "" && !domain.IsValidFeedbackStatus(filters.Status) {
		return nil, nil, infraerrors.BadRequest("FEEDBACK_STATUS_INVALID", "invalid feedback status")
	}
	return s.repo.ListByUser(ctx, userID, params, filters)
}

func (s *FeedbackService) GetByUser(ctx context.Context, userID, feedbackID int64) (*FeedbackDetail, error) {
	feedback, err := s.repo.GetByID(ctx, feedbackID)
	if err != nil {
		return nil, err
	}
	if feedback.UserID != userID {
		return nil, ErrFeedbackNotFound
	}

	replies, err := s.repo.ListRepliesByFeedbackID(ctx, feedbackID)
	if err != nil {
		return nil, fmt.Errorf("list feedback replies: %w", err)
	}
	detail := &FeedbackDetail{Feedback: *feedback, Replies: replies}
	if err := s.appendWorkflowDetail(ctx, detail); err != nil {
		return nil, fmt.Errorf("load feedback workflow: %w", err)
	}
	return detail, nil
}

func (s *FeedbackService) UpdateByUser(ctx context.Context, userID, feedbackID int64, input UpdateFeedbackByUserInput) (*Feedback, error) {
	feedback, err := s.repo.GetByID(ctx, feedbackID)
	if err != nil {
		return nil, err
	}
	if feedback.UserID != userID {
		return nil, ErrFeedbackNotFound
	}
	if feedback.Status == FeedbackStatusClosed || feedback.TriageStatus != domain.FeedbackTriageUnreviewed {
		return nil, ErrFeedbackClosed
	}

	content, images, requestID, err := normalizeFeedbackSubmission(input.Content, input.Images, input.RequestID)
	if err != nil {
		return nil, err
	}
	contact, err := s.registeredFeedbackContact(ctx, userID)
	if err != nil {
		return nil, err
	}

	feedback.Title = deriveFeedbackTitle(content)
	feedback.Content = content
	feedback.Images = images
	feedback.Contact = contact
	feedback.RequestID = requestID
	if err := s.repo.Update(ctx, feedback); err != nil {
		return nil, fmt.Errorf("update feedback: %w", err)
	}
	return feedback, nil
}

func (s *FeedbackService) ReplyByUser(ctx context.Context, userID, feedbackID int64, input CreateFeedbackReplyInput) (*FeedbackReply, error) {
	feedback, err := s.repo.GetByID(ctx, feedbackID)
	if err != nil {
		return nil, err
	}
	if feedback.UserID != userID {
		return nil, ErrFeedbackNotFound
	}
	if feedback.Status == FeedbackStatusClosed {
		return nil, ErrFeedbackClosed
	}

	content, images, err := normalizeReplyContent(input.Content, input.Images)
	if err != nil {
		return nil, err
	}

	reply := &FeedbackReply{
		FeedbackID: feedbackID,
		UserID:     userID,
		Role:       FeedbackReplyRoleUser,
		Content:    content,
		Images:     images,
	}
	if feedback.TriageStatus == domain.FeedbackTriageNeedsInfo {
		feedback.TriageStatus = domain.FeedbackTriageUnreviewed
	}
	if err := s.createReplyAndUpdateSummary(ctx, feedback, reply, FeedbackStatusProcessing); err != nil {
		return nil, err
	}
	return reply, nil
}

func (s *FeedbackService) ListForAdmin(ctx context.Context, params pagination.PaginationParams, filters AdminFeedbackListFilters) ([]Feedback, *pagination.PaginationResult, error) {
	if filters.Category != "" && !domain.IsValidFeedbackCategory(filters.Category) {
		return nil, nil, infraerrors.BadRequest("FEEDBACK_CATEGORY_INVALID", "invalid feedback category")
	}
	if filters.Status != "" && !domain.IsValidFeedbackStatus(filters.Status) {
		return nil, nil, infraerrors.BadRequest("FEEDBACK_STATUS_INVALID", "invalid feedback status")
	}
	if filters.Priority != "" && !domain.IsValidFeedbackPriority(filters.Priority) {
		return nil, nil, infraerrors.BadRequest("FEEDBACK_PRIORITY_INVALID", "invalid feedback priority")
	}
	return s.repo.ListForAdmin(ctx, params, filters)
}

func (s *FeedbackService) GetForAdmin(ctx context.Context, feedbackID int64) (*FeedbackDetail, error) {
	feedback, err := s.repo.GetByID(ctx, feedbackID)
	if err != nil {
		return nil, err
	}
	replies, err := s.repo.ListRepliesByFeedbackID(ctx, feedbackID)
	if err != nil {
		return nil, fmt.Errorf("list feedback replies: %w", err)
	}
	detail := &FeedbackDetail{Feedback: *feedback, Replies: replies}
	if err := s.appendWorkflowDetail(ctx, detail); err != nil {
		return nil, fmt.Errorf("load feedback workflow: %w", err)
	}
	return detail, nil
}

func (s *FeedbackService) ReplyByAdmin(ctx context.Context, adminUserID, feedbackID int64, input CreateFeedbackReplyInput) (*FeedbackReply, error) {
	feedback, err := s.repo.GetByID(ctx, feedbackID)
	if err != nil {
		return nil, err
	}
	if feedback.Status == FeedbackStatusClosed {
		return nil, ErrFeedbackClosed
	}

	content, images, err := normalizeReplyContent(input.Content, input.Images)
	if err != nil {
		return nil, err
	}

	replyUserID, err := s.resolveAdminReplyUserID(ctx, adminUserID)
	if err != nil {
		return nil, err
	}

	reply := &FeedbackReply{
		FeedbackID: feedbackID,
		UserID:     replyUserID,
		Role:       FeedbackReplyRoleAdmin,
		Content:    content,
		Images:     images,
	}
	if err := s.createReplyAndUpdateSummary(ctx, feedback, reply, FeedbackStatusReplied); err != nil {
		return nil, err
	}

	return reply, nil
}

// resolveAdminReplyUserID maps the reply author to a real users row. The
// global admin API key authenticates as a virtual service principal (user id
// -1) that has no users row; persisting it directly violates the
// feedback_replies.user_id foreign key, so those replies are attributed to the
// earliest active admin account. The reply role stays "admin", which is what
// the UI renders.
func (s *FeedbackService) resolveAdminReplyUserID(ctx context.Context, adminUserID int64) (int64, error) {
	if adminUserID > 0 {
		return adminUserID, nil
	}
	if s.entClient == nil {
		return 0, fmt.Errorf("feedback service ent client is not configured")
	}
	id, err := s.entClient.User.Query().
		Where(dbuser.RoleEQ(domain.RoleAdmin), dbuser.StatusEQ(domain.StatusActive)).
		Order(dbent.Asc(dbuser.FieldID)).
		FirstID(ctx)
	if err != nil {
		return 0, fmt.Errorf("resolve admin reply author: %w", err)
	}
	return id, nil
}

func (s *FeedbackService) UpdateStatus(ctx context.Context, feedbackID int64, input UpdateFeedbackStatusInput) error {
	status := domain.NormalizeFeedbackStatus(input.Status)
	if !domain.IsValidFeedbackStatus(status) {
		return infraerrors.BadRequest("FEEDBACK_STATUS_INVALID", "invalid feedback status")
	}

	feedback, err := s.repo.GetByID(ctx, feedbackID)
	if err != nil {
		return err
	}
	feedback.Status = status
	return s.repo.Update(ctx, feedback)
}

func (s *FeedbackService) UpdatePriority(ctx context.Context, feedbackID int64, input UpdateFeedbackPriorityInput) error {
	priority := domain.NormalizeFeedbackPriority(input.Priority)
	if !domain.IsValidFeedbackPriority(priority) {
		return infraerrors.BadRequest("FEEDBACK_PRIORITY_INVALID", "invalid feedback priority")
	}

	feedback, err := s.repo.GetByID(ctx, feedbackID)
	if err != nil {
		return err
	}
	feedback.Priority = priority
	return s.repo.Update(ctx, feedback)
}

func (s *FeedbackService) BatchUpdateStatus(ctx context.Context, input BatchUpdateFeedbackStatusInput) (int, error) {
	if len(input.IDs) == 0 || len(input.IDs) > 50 {
		return 0, ErrFeedbackBatchLimitExceeded
	}
	status := domain.NormalizeFeedbackStatus(input.Status)
	if !domain.IsValidFeedbackStatus(status) {
		return 0, infraerrors.BadRequest("FEEDBACK_STATUS_INVALID", "invalid feedback status")
	}
	return s.repo.BatchUpdateStatus(ctx, input.IDs, status)
}

func (s *FeedbackService) Delete(ctx context.Context, feedbackID int64) error {
	if _, err := s.repo.GetByID(ctx, feedbackID); err != nil {
		return err
	}
	return s.repo.Delete(ctx, feedbackID)
}

func (s *FeedbackService) BatchDelete(ctx context.Context, ids []int64) (int, error) {
	if len(ids) == 0 || len(ids) > 50 {
		return 0, ErrFeedbackBatchLimitExceeded
	}
	return s.repo.BatchDelete(ctx, ids)
}

func (s *FeedbackService) UploadImage(ctx context.Context, userID int64, filename, contentType string, size int64, body io.Reader) (string, error) {
	if s.imageStorage == nil || !s.imageStorage.Enabled(ctx) {
		return "", ErrFeedbackUploadUnavailable
	}
	ext := feedbackExtFromFilename(filename, contentType)
	objectKey := buildFeedbackObjectKey(userID, ext)
	if err := s.imageStorage.UploadObject(ctx, objectKey, body, size, contentType); err != nil {
		return "", fmt.Errorf("upload feedback image: %w", err)
	}
	url, err := s.imageStorage.GetAccessURL(ctx, objectKey)
	if err != nil {
		return "", fmt.Errorf("get feedback image url: %w", err)
	}
	return url, nil
}

func (s *FeedbackService) enforceCreateRateLimit(ctx context.Context, userID int64) error {
	if s.rateLimitCache == nil {
		return nil
	}
	allowed, retryAfter, err := s.rateLimitCache.CheckCreateLimit(ctx, userID, feedbackCreateLimit, feedbackCreateWindow)
	if err != nil {
		return nil
	}
	if allowed {
		return nil
	}
	seconds := int(retryAfter.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	return ErrFeedbackRateLimited.WithMetadata(map[string]string{
		"retry_after": strconv.Itoa(seconds),
	})
}

func (s *FeedbackService) createReplyAndUpdateSummary(ctx context.Context, feedback *Feedback, reply *FeedbackReply, nextStatus string) error {
	if s.entClient == nil {
		return fmt.Errorf("feedback service ent client is not configured")
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin feedback reply transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	txCtx := dbent.NewTxContext(ctx, tx)
	if err := s.repo.CreateReply(txCtx, reply); err != nil {
		return fmt.Errorf("create feedback reply: %w", err)
	}

	now := reply.CreatedAt
	if now.IsZero() {
		now = time.Now()
	}
	replyRole := reply.Role
	feedback.ReplyCount++
	feedback.LastReplyAt = &now
	feedback.LastReplyRole = &replyRole
	feedback.Status = nextStatus
	if err := s.repo.Update(txCtx, feedback); err != nil {
		return fmt.Errorf("update feedback after reply: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit feedback reply transaction: %w", err)
	}
	return nil
}

func normalizeFeedbackSubmission(content string, images []string, requestID string) (string, []string, string, error) {
	normalizedContent := strings.TrimSpace(content)
	if normalizedContent == "" || len([]rune(normalizedContent)) > feedbackMaxBodyLen {
		return "", nil, "", infraerrors.BadRequest("FEEDBACK_CONTENT_INVALID", "feedback content must be 1-5000 characters")
	}

	normalizedImages, err := normalizeFeedbackImages(images)
	if err != nil {
		return "", nil, "", err
	}
	if len(normalizedImages) > feedbackMaxImages {
		return "", nil, "", infraerrors.BadRequest("FEEDBACK_IMAGES_INVALID", "feedback supports at most 5 images")
	}

	normalizedRequestID := strings.TrimSpace(requestID)
	if len([]rune(normalizedRequestID)) > feedbackMaxContextLen {
		return "", nil, "", infraerrors.BadRequest("FEEDBACK_REQUEST_ID_INVALID", "request id and error details must be at most 2000 characters")
	}

	return normalizedContent, normalizedImages, normalizedRequestID, nil
}

func (s *FeedbackService) registeredFeedbackContact(ctx context.Context, userID int64) (string, error) {
	if s.userRepo == nil {
		return "", fmt.Errorf("feedback service user repository is not configured")
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("get feedback user: %w", err)
	}
	contact := strings.TrimSpace(user.Email)
	if contact == "" || len([]rune(contact)) > 255 {
		return "", infraerrors.BadRequest("FEEDBACK_CONTACT_INVALID", "registered email is unavailable")
	}
	return contact, nil
}

func deriveFeedbackTitle(content string) string {
	title := strings.Join(strings.Fields(content), " ")
	runes := []rune(title)
	const maxDerivedTitleLen = 60
	if len(runes) > maxDerivedTitleLen {
		return string(runes[:maxDerivedTitleLen]) + "…"
	}
	return title
}

func normalizeReplyContent(content string, images []string) (string, []string, error) {
	normalizedContent := strings.TrimSpace(content)
	if normalizedContent == "" || len([]rune(normalizedContent)) > feedbackMaxBodyLen {
		return "", nil, infraerrors.BadRequest("FEEDBACK_REPLY_CONTENT_INVALID", "feedback reply content must be 1-5000 characters")
	}
	normalizedImages, err := normalizeFeedbackImages(images)
	if err != nil {
		return "", nil, err
	}
	if len(normalizedImages) > feedbackMaxImages {
		return "", nil, infraerrors.BadRequest("FEEDBACK_REPLY_IMAGES_INVALID", "feedback reply supports at most 5 images")
	}
	return normalizedContent, normalizedImages, nil
}

func normalizeFeedbackImages(images []string) ([]string, error) {
	out := make([]string, 0, len(images))
	for _, image := range images {
		trimmed := strings.TrimSpace(image)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "/api/v1/feedback-images/") {
			out = append(out, trimmed)
			continue
		}
		u, err := url.Parse(trimmed)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
			return nil, infraerrors.BadRequest("FEEDBACK_IMAGE_INVALID", "feedback image must be a valid HTTP(S) URL")
		}
		out = append(out, trimmed)
	}
	return out, nil
}

func buildFeedbackObjectKey(userID int64, ext string) string {
	timestamp := time.Now().UTC().Format("20060102T150405Z")
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return fmt.Sprintf("feedbacks/%d/%s_%d%s", userID, timestamp, time.Now().UnixNano(), ext)
}

func feedbackExtFromFilename(filename, contentType string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
		return ext
	}
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	default:
		return ".bin"
	}
}
