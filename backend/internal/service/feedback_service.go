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
	"github.com/bozhouDev/DragonCode-sub2api/internal/domain"
	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
)

const (
	feedbackCreateLimit  = 10
	feedbackCreateWindow = time.Hour
	feedbackMaxImages    = 5
	feedbackMaxTitleLen  = 200
	feedbackMaxBodyLen   = 5000
)

type FeedbackService struct {
	repo           FeedbackRepository
	userRepo       UserRepository
	settingService *SettingService
	emailQueue     *EmailQueueService
	rateLimitCache FeedbackRateLimitCache
	imageStorage   FeedbackImageStorage
	entClient      *dbent.Client
}

func NewFeedbackService(
	repo FeedbackRepository,
	userRepo UserRepository,
	settingService *SettingService,
	emailQueue *EmailQueueService,
	rateLimitCache FeedbackRateLimitCache,
	imageStorage FeedbackImageStorage,
	entClient *dbent.Client,
) *FeedbackService {
	return &FeedbackService{
		repo:           repo,
		userRepo:       userRepo,
		settingService: settingService,
		emailQueue:     emailQueue,
		rateLimitCache: rateLimitCache,
		imageStorage:   imageStorage,
		entClient:      entClient,
	}
}

func (s *FeedbackService) Create(ctx context.Context, userID int64, input CreateFeedbackInput) (*Feedback, error) {
	if err := s.enforceCreateRateLimit(ctx, userID); err != nil {
		return nil, err
	}

	title, content, images, contact, category, err := normalizeFeedbackContent(input.Category, input.Title, input.Content, input.Images, input.Contact)
	if err != nil {
		return nil, err
	}

	feedback := &Feedback{
		UserID:     userID,
		Category:   category,
		Title:      title,
		Content:    content,
		Images:     images,
		Contact:    contact,
		Priority:   domain.DefaultFeedbackPriority(category),
		Status:     FeedbackStatusPending,
		ReplyCount: 0,
	}
	if err := s.repo.Create(ctx, feedback); err != nil {
		return nil, fmt.Errorf("create feedback: %w", err)
	}

	s.enqueueNewFeedbackEmail(ctx, userID, feedback)
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
	return &FeedbackDetail{Feedback: *feedback, Replies: replies}, nil
}

func (s *FeedbackService) UpdateByUser(ctx context.Context, userID, feedbackID int64, input UpdateFeedbackByUserInput) (*Feedback, error) {
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

	title, content, images, contact, category, err := normalizeFeedbackContent(input.Category, input.Title, input.Content, input.Images, input.Contact)
	if err != nil {
		return nil, err
	}

	feedback.Category = category
	feedback.Title = title
	feedback.Content = content
	feedback.Images = images
	feedback.Contact = contact
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
	if feedback.Status == FeedbackStatusPending {
		if err := s.repo.TransitionStatus(ctx, feedbackID, FeedbackStatusPending, FeedbackStatusProcessing); err == nil {
			feedback.Status = FeedbackStatusProcessing
		}
		// Ignore transition error — viewing should still succeed
	}

	replies, err := s.repo.ListRepliesByFeedbackID(ctx, feedbackID)
	if err != nil {
		return nil, fmt.Errorf("list feedback replies: %w", err)
	}
	return &FeedbackDetail{Feedback: *feedback, Replies: replies}, nil
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

	reply := &FeedbackReply{
		FeedbackID: feedbackID,
		UserID:     adminUserID,
		Role:       FeedbackReplyRoleAdmin,
		Content:    content,
		Images:     images,
	}
	if err := s.createReplyAndUpdateSummary(ctx, feedback, reply, FeedbackStatusReplied); err != nil {
		return nil, err
	}

	s.enqueueReplyEmail(ctx, feedback, reply)
	return reply, nil
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

func (s *FeedbackService) enqueueNewFeedbackEmail(ctx context.Context, userID int64, feedback *Feedback) {
	if s.emailQueue == nil || s.settingService == nil {
		return
	}

	targetEmail := strings.TrimSpace(s.settingService.GetFeedbackNotifyEmail(ctx))
	if targetEmail == "" {
		adminUser, err := s.userRepo.GetFirstAdmin(ctx)
		if err == nil && adminUser != nil {
			targetEmail = strings.TrimSpace(adminUser.Email)
		}
	}
	if targetEmail == "" {
		return
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return
	}

	feedbackURL := strings.TrimRight(s.settingService.GetFrontendURL(ctx), "/") + "/admin/feedbacks/" + strconv.FormatInt(feedback.ID, 10)
	_ = s.emailQueue.EnqueueFeedbackNew(
		targetEmail,
		s.settingService.GetSiteName(ctx),
		feedback.ID,
		feedback.Title,
		feedback.Category,
		feedback.Priority,
		user.Username,
		user.Email,
		feedbackURL,
	)
}

func (s *FeedbackService) enqueueReplyEmail(ctx context.Context, feedback *Feedback, reply *FeedbackReply) {
	if s.emailQueue == nil || s.settingService == nil {
		return
	}

	user, err := s.userRepo.GetByID(ctx, feedback.UserID)
	if err != nil || user == nil || strings.TrimSpace(user.Email) == "" {
		return
	}
	feedbackURL := strings.TrimRight(s.settingService.GetFrontendURL(ctx), "/") + "/feedbacks/" + strconv.FormatInt(feedback.ID, 10)
	_ = s.emailQueue.EnqueueFeedbackReply(
		user.Email,
		s.settingService.GetSiteName(ctx),
		feedback.ID,
		feedback.Title,
		buildFeedbackSummary(reply.Content),
		feedbackURL,
	)
}

func normalizeFeedbackContent(category, title, content string, images []string, contact string) (string, string, []string, string, string, error) {
	normalizedCategory := domain.NormalizeFeedbackCategory(category)
	if !domain.IsValidFeedbackCategory(normalizedCategory) {
		return "", "", nil, "", "", infraerrors.BadRequest("FEEDBACK_CATEGORY_INVALID", "invalid feedback category")
	}

	normalizedTitle := strings.TrimSpace(title)
	if normalizedTitle == "" || len([]rune(normalizedTitle)) > feedbackMaxTitleLen {
		return "", "", nil, "", "", infraerrors.BadRequest("FEEDBACK_TITLE_INVALID", "feedback title must be 1-200 characters")
	}

	normalizedContent := strings.TrimSpace(content)
	if normalizedContent == "" || len([]rune(normalizedContent)) > feedbackMaxBodyLen {
		return "", "", nil, "", "", infraerrors.BadRequest("FEEDBACK_CONTENT_INVALID", "feedback content must be 1-5000 characters")
	}

	normalizedImages, err := normalizeFeedbackImages(images)
	if err != nil {
		return "", "", nil, "", "", err
	}
	if len(normalizedImages) > feedbackMaxImages {
		return "", "", nil, "", "", infraerrors.BadRequest("FEEDBACK_IMAGES_INVALID", "feedback supports at most 5 images")
	}

	normalizedContact := strings.TrimSpace(contact)
	if len([]rune(normalizedContact)) > 255 {
		return "", "", nil, "", "", infraerrors.BadRequest("FEEDBACK_CONTACT_INVALID", "feedback contact must be at most 255 characters")
	}

	return normalizedTitle, normalizedContent, normalizedImages, normalizedContact, normalizedCategory, nil
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

func buildFeedbackSummary(content string) string {
	trimmed := strings.Join(strings.Fields(strings.TrimSpace(content)), " ")
	if len([]rune(trimmed)) <= 120 {
		return trimmed
	}
	return string([]rune(trimmed)[:120]) + "..."
}
