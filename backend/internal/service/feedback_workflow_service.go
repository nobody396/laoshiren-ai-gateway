package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	dbfeedback "github.com/bozhouDev/DragonCode-sub2api/ent/feedback"
	dbfeedbackevent "github.com/bozhouDev/DragonCode-sub2api/ent/feedbackevent"
	dbfeedbackreward "github.com/bozhouDev/DragonCode-sub2api/ent/feedbackreward"
	dbuser "github.com/bozhouDev/DragonCode-sub2api/ent/user"
	dbnotification "github.com/bozhouDev/DragonCode-sub2api/ent/usernotification"
	"github.com/bozhouDev/DragonCode-sub2api/internal/domain"
	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
)

const (
	feedbackActorUser    = "user"
	feedbackActorAgent   = "agent"
	feedbackActorAdmin   = "admin"
	feedbackRewardReason = "有效反馈共创奖励"
)

func (s *FeedbackService) createFeedbackWithEvent(ctx context.Context, feedback *Feedback) error {
	if s.entClient == nil {
		return s.repo.Create(ctx, feedback)
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	if err := s.repo.Create(txCtx, feedback); err != nil {
		return err
	}
	actorID := feedback.UserID
	if _, err := tx.FeedbackEvent.Create().
		SetFeedbackID(feedback.ID).
		SetEventType(domain.FeedbackEventSubmitted).
		SetActorType(feedbackActorUser).
		SetActorUserID(actorID).
		SetSummary("用户已提交反馈").
		SetMetadata(map[string]string{"category": feedback.Category}).
		Save(ctx); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *FeedbackService) appendWorkflowDetail(ctx context.Context, detail *FeedbackDetail) error {
	if detail == nil || s.entClient == nil {
		return nil
	}
	events, err := s.entClient.FeedbackEvent.Query().
		Where(dbfeedbackevent.FeedbackIDEQ(detail.Feedback.ID)).
		Order(dbent.Asc(dbfeedbackevent.FieldCreatedAt), dbent.Asc(dbfeedbackevent.FieldID)).All(ctx)
	if err != nil {
		return err
	}
	detail.Events = make([]FeedbackEvent, 0, len(events))
	for _, event := range events {
		detail.Events = append(detail.Events, feedbackEventFromEntity(event))
	}
	reward, err := s.entClient.FeedbackReward.Query().Where(dbfeedbackreward.FeedbackIDEQ(detail.Feedback.ID)).Only(ctx)
	if err == nil {
		converted := feedbackRewardFromEntity(reward)
		detail.Reward = &converted
	}
	if err != nil && !dbent.IsNotFound(err) {
		return err
	}
	return nil
}

func (s *FeedbackService) ListAgentQueue(ctx context.Context, params pagination.PaginationParams) ([]Feedback, *pagination.PaginationResult, error) {
	return s.ListForAdmin(ctx, params, AdminFeedbackListFilters{TriageStatus: domain.FeedbackTriageUnreviewed})
}

func (s *FeedbackService) TriageByAgent(ctx context.Context, feedbackID int64, input AgentTriageFeedbackInput) (*Feedback, error) {
	status := domain.NormalizeFeedbackTriageStatus(input.TriageStatus)
	priority := domain.NormalizeFeedbackTriagePriority(input.TriagePriority)
	difficulty := domain.NormalizeFeedbackDifficulty(input.RepairDifficulty)
	summary := strings.TrimSpace(input.TriageSummary)
	recommendation := strings.TrimSpace(input.RepairRecommendation)
	if !domain.IsValidFeedbackTriageStatus(status) {
		return nil, infraerrors.BadRequest("FEEDBACK_TRIAGE_STATUS_INVALID", "invalid triage status")
	}
	if !domain.IsValidFeedbackTriagePriority(priority) {
		return nil, infraerrors.BadRequest("FEEDBACK_TRIAGE_PRIORITY_INVALID", "invalid triage priority")
	}
	if !domain.IsValidFeedbackDifficulty(difficulty) {
		return nil, infraerrors.BadRequest("FEEDBACK_DIFFICULTY_INVALID", "invalid repair difficulty")
	}
	if summary == "" || len([]rune(summary)) > 2000 {
		return nil, infraerrors.BadRequest("FEEDBACK_TRIAGE_SUMMARY_INVALID", "triage summary is required and must be at most 2000 characters")
	}
	if len([]rune(recommendation)) > 4000 {
		return nil, infraerrors.BadRequest("FEEDBACK_RECOMMENDATION_INVALID", "repair recommendation must be at most 4000 characters")
	}
	if input.TriageConfidence != nil && (*input.TriageConfidence < 0 || *input.TriageConfidence > 1) {
		return nil, infraerrors.BadRequest("FEEDBACK_CONFIDENCE_INVALID", "triage confidence must be between 0 and 1")
	}
	if status == domain.FeedbackTriageDuplicate && input.DuplicateOfID == nil {
		return nil, infraerrors.BadRequest("FEEDBACK_DUPLICATE_TARGET_REQUIRED", "duplicate feedback requires duplicate_of_id")
	}
	if s.entClient == nil {
		return nil, fmt.Errorf("feedback workflow database is not configured")
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	item, err := tx.Feedback.Query().Where(dbfeedback.IDEQ(feedbackID)).Only(ctx)
	if err != nil {
		return nil, translateFeedbackEntError(err)
	}
	if item.TriageStatus != domain.FeedbackTriageUnreviewed {
		if feedbackTriageMatches(item, status, priority, summary, difficulty, recommendation, input.TriageConfidence, input.DuplicateOfID) {
			return feedbackEntityForWorkflow(item), nil
		}
		return nil, domain.ErrFeedbackAlreadyTriaged
	}
	update := tx.Feedback.Update().Where(dbfeedback.IDEQ(feedbackID), dbfeedback.TriageStatusEQ(domain.FeedbackTriageUnreviewed)).
		SetTriageStatus(status).
		SetTriagePriority(priority).
		SetTriageSummary(summary).
		SetRepairDifficulty(difficulty).
		SetRepairRecommendation(recommendation).
		SetPriority(legacyPriorityFromTriage(priority))
	if input.TriageConfidence != nil {
		update.SetTriageConfidence(*input.TriageConfidence)
	} else {
		update.ClearTriageConfidence()
	}
	if input.DuplicateOfID != nil {
		update.SetDuplicateOfID(*input.DuplicateOfID)
	} else {
		update.ClearDuplicateOfID()
	}
	affected, err := update.Save(ctx)
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		latest, queryErr := tx.Feedback.Query().Where(dbfeedback.IDEQ(feedbackID)).Only(ctx)
		if queryErr != nil {
			return nil, translateFeedbackEntError(queryErr)
		}
		if feedbackTriageMatches(latest, status, priority, summary, difficulty, recommendation, input.TriageConfidence, input.DuplicateOfID) {
			return feedbackEntityForWorkflow(latest), nil
		}
		return nil, domain.ErrFeedbackAlreadyTriaged
	}
	updated, err := tx.Feedback.Query().Where(dbfeedback.IDEQ(feedbackID)).Only(ctx)
	if err != nil {
		return nil, translateFeedbackEntError(err)
	}
	metadata := map[string]string{"priority": priority, "result": status, "difficulty": difficulty}
	if _, err := tx.FeedbackEvent.Create().SetFeedbackID(feedbackID).SetEventType(domain.FeedbackEventTriaged).
		SetActorType(feedbackActorAgent).SetSummary("反馈已完成核查").SetMetadata(metadata).Save(ctx); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return feedbackEntityForWorkflow(updated), nil
}

func feedbackTriageMatches(item *dbent.Feedback, status, priority, summary, difficulty, recommendation string, confidence *float64, duplicateID *int64) bool {
	if item == nil || item.TriageStatus != status || item.TriagePriority != priority || item.TriageSummary != summary || item.RepairDifficulty != difficulty || item.RepairRecommendation != recommendation {
		return false
	}
	if (item.TriageConfidence == nil) != (confidence == nil) || item.TriageConfidence != nil && *item.TriageConfidence != *confidence {
		return false
	}
	if (item.DuplicateOfID == nil) != (duplicateID == nil) || item.DuplicateOfID != nil && *item.DuplicateOfID != *duplicateID {
		return false
	}
	return true
}

func (s *FeedbackService) AcceptBatch(ctx context.Context, input AcceptFeedbackBatchInput) []AcceptFeedbackResult {
	results := make([]AcceptFeedbackResult, 0, len(input.IDs))
	batchID := strings.TrimSpace(input.BatchID)
	if len(input.IDs) == 0 || len(input.IDs) > 50 || batchID == "" || len(batchID) > 64 {
		return []AcceptFeedbackResult{{Error: "invalid feedback ids or batch id"}}
	}
	seen := make(map[int64]struct{}, len(input.IDs))
	for _, id := range input.IDs {
		if id <= 0 {
			results = append(results, AcceptFeedbackResult{FeedbackID: id, Error: "invalid feedback id"})
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result, err := s.acceptOne(ctx, id, batchID, positiveUserID(input.OperatorUserID), input.OwnerOverride)
		if err != nil {
			result.Error = err.Error()
		}
		results = append(results, result)
	}
	return results
}

func (s *FeedbackService) acceptOne(ctx context.Context, feedbackID int64, batchID string, operatorID *int64, ownerOverride bool) (AcceptFeedbackResult, error) {
	result := AcceptFeedbackResult{FeedbackID: feedbackID}
	if s.entClient == nil {
		return result, fmt.Errorf("feedback workflow database is not configured")
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback() }()
	item, err := tx.Feedback.Query().Where(dbfeedback.IDEQ(feedbackID)).Only(ctx)
	if err != nil {
		return result, translateFeedbackEntError(err)
	}
	if existing, queryErr := tx.FeedbackReward.Query().Where(dbfeedbackreward.FeedbackIDEQ(feedbackID)).Only(ctx); queryErr == nil {
		reward := feedbackRewardFromEntity(existing)
		result.Reward, result.AlreadyAccepted = &reward, true
		return result, nil
	} else if !dbent.IsNotFound(queryErr) {
		return result, queryErr
	}
	if item.TriageStatus != domain.FeedbackTriageConfirmed {
		if !ownerOverride || item.TriageStatus == domain.FeedbackTriageUnreviewed || item.OwnerDecision != domain.FeedbackDecisionPending {
			return result, domain.ErrFeedbackNotConfirmed
		}
		previousStatus := item.TriageStatus
		item, err = tx.Feedback.UpdateOne(item).SetTriageStatus(domain.FeedbackTriageConfirmed).Save(ctx)
		if err != nil {
			return result, err
		}
		if err := createFeedbackEvent(ctx, tx, feedbackID, domain.FeedbackEventTriaged, feedbackActorAdmin, operatorID, "负责人复核后确认采纳", map[string]string{
			"owner_override":  "true",
			"previous_result": previousStatus,
			"result":          domain.FeedbackTriageConfirmed,
		}); err != nil {
			return result, err
		}
	}
	now := time.Now()
	if _, err := tx.User.UpdateOneID(item.UserID).AddBalance(domain.FeedbackRewardAmount).Save(ctx); err != nil {
		return result, err
	}
	dedupe := "feedback_reward:" + strconv.FormatInt(feedbackID, 10)
	if s.affiliateConsumption == nil {
		return result, errors.New("feedback reward balance lot ledger is not configured")
	}
	txCtx := dbent.NewTxContext(ctx, tx)
	if err := s.affiliateConsumption.RecordBalanceLot(txCtx, AffiliateBalanceLotInput{
		UserID:          item.UserID,
		SourceType:      AffiliateSourceGift,
		SourceID:        feedbackID,
		SourceKey:       dedupe,
		AmountMicros:    AffiliateMicrosFromFloat(domain.FeedbackRewardAmount),
		AffiliatePolicy: AffiliateSourcePolicyNone,
		OccurredAt:      now,
	}); err != nil {
		return result, fmt.Errorf("record feedback reward balance lot: %w", err)
	}
	ledger, err := tx.AccountChangeRecord.Create().SetUserID(item.UserID).
		SetAssetType(AccountChangeAssetBalance).SetReason(AccountChangeReasonFeedbackReward).
		SetDelta(domain.FeedbackRewardAmount).SetSourceType(AccountChangeSourceFeedback).SetSourceID(feedbackID).
		SetReferenceNo("FB-" + strconv.FormatInt(feedbackID, 10)).SetNillableOperatorUserID(operatorID).
		SetNotes(feedbackRewardReason).SetDedupeKey(dedupe).Save(ctx)
	if err != nil {
		return result, err
	}
	rewardEntity, err := tx.FeedbackReward.Create().SetFeedbackID(feedbackID).SetUserID(item.UserID).
		SetAmount(domain.FeedbackRewardAmount).SetReason(feedbackRewardReason).SetBatchID(batchID).
		SetNillableOperatorUserID(operatorID).SetAccountChangeRecordID(ledger.ID).SetGrantedAt(now).Save(ctx)
	if err != nil {
		return result, err
	}
	if _, err := tx.Feedback.UpdateOne(item).SetOwnerDecision(domain.FeedbackDecisionApproved).
		SetStatus(domain.FeedbackStatusProcessing).SetAcceptedAt(now).Save(ctx); err != nil {
		return result, err
	}
	if err := createFeedbackEvent(ctx, tx, feedbackID, domain.FeedbackEventAccepted, feedbackActorAdmin, operatorID, "反馈已被采纳", map[string]string{"batch_id": batchID}); err != nil {
		return result, err
	}
	if err := createFeedbackEvent(ctx, tx, feedbackID, domain.FeedbackEventRewardGranted, feedbackActorAdmin, operatorID, "已发放 5 元共创奖励", map[string]string{"amount": "5.00", "ledger_id": strconv.FormatInt(ledger.ID, 10)}); err != nil {
		return result, err
	}
	if err := createUserNotification(ctx, tx, item.UserID, feedbackID, "feedback_reward", "反馈已采纳", "感谢你的反馈，我们已赠送 5 元额度。", "reward:"+strconv.FormatInt(feedbackID, 10)); err != nil {
		return result, err
	}
	if err := tx.Commit(); err != nil {
		return result, err
	}
	reward := feedbackRewardFromEntity(rewardEntity)
	result.Reward = &reward
	s.invalidateRewardCaches(ctx, item.UserID)
	return result, nil
}

func (s *FeedbackService) MarkFixing(ctx context.Context, feedbackID int64, operatorID *int64) error {
	if s.entClient == nil {
		return fmt.Errorf("feedback workflow database is not configured")
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	item, err := tx.Feedback.Query().Where(dbfeedback.IDEQ(feedbackID)).Only(ctx)
	if err != nil {
		return translateFeedbackEntError(err)
	}
	if item.OwnerDecision != domain.FeedbackDecisionApproved {
		return domain.ErrFeedbackNotApproved
	}
	if item.FixStatus == domain.FeedbackFixFixing {
		return nil
	}
	if _, err := tx.Feedback.UpdateOne(item).SetFixStatus(domain.FeedbackFixFixing).Save(ctx); err != nil {
		return err
	}
	if err := createFeedbackEvent(ctx, tx, feedbackID, domain.FeedbackEventFixStarted, feedbackActorAgent, positiveUserID(operatorID), "反馈已进入修复流程", nil); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *FeedbackService) Complete(ctx context.Context, feedbackID int64, input CompleteFeedbackInput) error {
	version := strings.TrimSpace(input.ResolvedVersion)
	if len(version) > 64 {
		return infraerrors.BadRequest("FEEDBACK_VERSION_INVALID", "resolved version must be at most 64 characters")
	}
	if s.entClient == nil {
		return fmt.Errorf("feedback workflow database is not configured")
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	item, err := tx.Feedback.Query().Where(dbfeedback.IDEQ(feedbackID)).Only(ctx)
	if err != nil {
		return translateFeedbackEntError(err)
	}
	if item.OwnerDecision != domain.FeedbackDecisionApproved {
		return domain.ErrFeedbackNotApproved
	}
	alreadyCompleted := item.FixStatus == domain.FeedbackFixAwaitingVerify && item.ResolvedVersion == version
	if alreadyCompleted && !input.NotifyInApp {
		return nil
	}
	if alreadyCompleted && input.NotifyInApp {
		dedupe := "fixed:" + strconv.FormatInt(feedbackID, 10) + ":" + version
		exists, queryErr := tx.UserNotification.Query().Where(dbnotification.DedupeKeyEQ(dedupe)).Exist(ctx)
		if queryErr != nil {
			return queryErr
		}
		if exists {
			return nil
		}
		if err := createUserNotification(ctx, tx, item.UserID, feedbackID, "feedback_fixed", "你反馈的问题已修复", "请打开反馈详情验证是否已经解决。", dedupe); err != nil {
			return err
		}
		if err := createFeedbackEvent(ctx, tx, feedbackID, domain.FeedbackEventNotified, feedbackActorAdmin, positiveUserID(input.OperatorUserID), "已通过站内通知邀请用户验证", map[string]string{"channel": "in_app"}); err != nil {
			return err
		}
		return tx.Commit()
	}
	now := time.Now()
	if _, err := tx.Feedback.UpdateOne(item).SetFixStatus(domain.FeedbackFixAwaitingVerify).
		SetStatus(domain.FeedbackStatusReplied).SetResolvedVersion(version).SetResolvedAt(now).ClearVerifiedAt().Save(ctx); err != nil {
		return err
	}
	metadata := map[string]string{}
	if version != "" {
		metadata["version"] = version
	}
	if err := createFeedbackEvent(ctx, tx, feedbackID, domain.FeedbackEventFixed, feedbackActorAgent, positiveUserID(input.OperatorUserID), "反馈问题已修复，等待用户验证", metadata); err != nil {
		return err
	}
	if input.NotifyInApp {
		if err := createUserNotification(ctx, tx, item.UserID, feedbackID, "feedback_fixed", "你反馈的问题已修复", "请打开反馈详情验证是否已经解决。", "fixed:"+strconv.FormatInt(feedbackID, 10)+":"+version); err != nil {
			return err
		}
		if err := createFeedbackEvent(ctx, tx, feedbackID, domain.FeedbackEventNotified, feedbackActorAdmin, positiveUserID(input.OperatorUserID), "已通过站内通知邀请用户验证", map[string]string{"channel": "in_app"}); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *FeedbackService) RecordNotification(ctx context.Context, feedbackID int64, input RecordFeedbackNotificationInput) error {
	channel := strings.ToLower(strings.TrimSpace(input.Channel))
	reference := strings.TrimSpace(input.DeliveryReference)
	if channel != "email" {
		return infraerrors.BadRequest("FEEDBACK_NOTIFICATION_CHANNEL_INVALID", "only email delivery records are accepted")
	}
	if reference == "" || len(reference) > 255 {
		return infraerrors.BadRequest("FEEDBACK_NOTIFICATION_REFERENCE_INVALID", "delivery reference is required and must be at most 255 characters")
	}
	if s.entClient == nil {
		return fmt.Errorf("feedback workflow database is not configured")
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	item, err := tx.Feedback.Query().Where(dbfeedback.IDEQ(feedbackID)).Only(ctx)
	if err != nil {
		return translateFeedbackEntError(err)
	}
	if item.FixStatus != domain.FeedbackFixAwaitingVerify {
		return domain.ErrFeedbackNotAwaitingVerify
	}
	if err := createFeedbackEvent(ctx, tx, feedbackID, domain.FeedbackEventNotified, feedbackActorAdmin, positiveUserID(input.OperatorUserID), "已通过邮件邀请用户验证", map[string]string{"channel": "email", "delivery_reference": reference}); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *FeedbackService) VerifyByUser(ctx context.Context, userID, feedbackID int64, input VerifyFeedbackInput) error {
	if len([]rune(strings.TrimSpace(input.Note))) > 2000 {
		return infraerrors.BadRequest("FEEDBACK_VERIFY_NOTE_INVALID", "verification note must be at most 2000 characters")
	}
	if s.entClient == nil {
		return fmt.Errorf("feedback workflow database is not configured")
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	item, err := tx.Feedback.Query().Where(dbfeedback.IDEQ(feedbackID), dbfeedback.UserIDEQ(userID)).Only(ctx)
	if err != nil {
		return translateFeedbackEntError(err)
	}
	if item.FixStatus != domain.FeedbackFixAwaitingVerify {
		return domain.ErrFeedbackNotAwaitingVerify
	}
	now := time.Now()
	actorID := userID
	if input.Resolved {
		if _, err := tx.Feedback.UpdateOne(item).SetFixStatus(domain.FeedbackFixVerified).SetStatus(domain.FeedbackStatusClosed).SetVerifiedAt(now).Save(ctx); err != nil {
			return err
		}
		return commitFeedbackEvent(ctx, tx, feedbackID, domain.FeedbackEventVerified, feedbackActorUser, &actorID, "用户已确认问题解决", strings.TrimSpace(input.Note))
	}
	if _, err := tx.Feedback.UpdateOne(item).SetFixStatus(domain.FeedbackFixReopened).SetStatus(domain.FeedbackStatusProcessing).ClearVerifiedAt().Save(ctx); err != nil {
		return err
	}
	return commitFeedbackEvent(ctx, tx, feedbackID, domain.FeedbackEventReopened, feedbackActorUser, &actorID, "用户反馈问题仍未解决，工单已重新打开", strings.TrimSpace(input.Note))
}

func (s *FeedbackService) ListRewards(ctx context.Context, params pagination.PaginationParams, filters FeedbackRewardListFilters) ([]FeedbackReward, *pagination.PaginationResult, error) {
	query := s.entClient.FeedbackReward.Query()
	if filters.UserID != nil {
		query = query.Where(dbfeedbackreward.UserIDEQ(*filters.UserID))
	}
	if strings.TrimSpace(filters.BatchID) != "" {
		query = query.Where(dbfeedbackreward.BatchIDEQ(strings.TrimSpace(filters.BatchID)))
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, nil, err
	}
	items, err := query.Offset(params.Offset()).Limit(params.Limit()).Order(dbent.Desc(dbfeedbackreward.FieldID)).All(ctx)
	if err != nil {
		return nil, nil, err
	}
	out := make([]FeedbackReward, 0, len(items))
	userIDs := make([]int64, 0, len(items))
	seenUsers := map[int64]struct{}{}
	for _, item := range items {
		if _, ok := seenUsers[item.UserID]; !ok {
			seenUsers[item.UserID] = struct{}{}
			userIDs = append(userIDs, item.UserID)
		}
	}
	usersByID := map[int64]*FeedbackUser{}
	if len(userIDs) > 0 {
		users, userErr := s.entClient.User.Query().Where(dbuser.IDIn(userIDs...)).Select(dbuser.FieldID, dbuser.FieldEmail, dbuser.FieldUsername).All(ctx)
		if userErr != nil {
			return nil, nil, userErr
		}
		for _, user := range users {
			usersByID[user.ID] = &FeedbackUser{ID: user.ID, Email: user.Email, Username: user.Username}
		}
	}
	for _, item := range items {
		reward := feedbackRewardFromEntity(item)
		reward.User = usersByID[item.UserID]
		out = append(out, reward)
	}
	pages := total / params.Limit()
	if total%params.Limit() != 0 {
		pages++
	}
	return out, &pagination.PaginationResult{Total: int64(total), Page: params.Page, PageSize: params.Limit(), Pages: pages}, nil
}

func (s *FeedbackService) ListNotifications(ctx context.Context, userID int64, limit int) (*UserNotificationListResult, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	items, err := s.entClient.UserNotification.Query().Where(dbnotification.UserIDEQ(userID)).Limit(limit).Order(dbent.Desc(dbnotification.FieldID)).All(ctx)
	if err != nil {
		return nil, err
	}
	unread, err := s.entClient.UserNotification.Query().Where(dbnotification.UserIDEQ(userID), dbnotification.ReadAtIsNil()).Count(ctx)
	if err != nil {
		return nil, err
	}
	out := &UserNotificationListResult{Items: make([]UserNotification, 0, len(items)), UnreadCount: unread}
	for _, item := range items {
		out.Items = append(out.Items, userNotificationFromEntity(item))
	}
	return out, nil
}

func (s *FeedbackService) MarkNotificationRead(ctx context.Context, userID, notificationID int64) error {
	now := time.Now()
	affected, err := s.entClient.UserNotification.Update().Where(dbnotification.IDEQ(notificationID), dbnotification.UserIDEQ(userID)).SetReadAt(now).Save(ctx)
	if err != nil {
		return err
	}
	if affected == 0 {
		return infraerrors.NotFound("NOTIFICATION_NOT_FOUND", "notification not found")
	}
	return nil
}

func (s *FeedbackService) MarkAllNotificationsRead(ctx context.Context, userID int64) (int, error) {
	return s.entClient.UserNotification.Update().Where(dbnotification.UserIDEQ(userID), dbnotification.ReadAtIsNil()).SetReadAt(time.Now()).Save(ctx)
}

func (s *FeedbackService) GetFeedbackImageAccess(ctx context.Context, token string) (*FeedbackImageAccess, error) {
	access, err := s.imageStorage.ResolveToken(ctx, token, 10*time.Minute)
	if err != nil {
		if errors.Is(err, ErrFeedbackImageTokenInvalid) {
			return nil, infraerrors.BadRequest("FEEDBACK_IMAGE_TOKEN_INVALID", "invalid feedback image token")
		}
		if errors.Is(err, ErrFeedbackImageNotFound) {
			return nil, infraerrors.NotFound("FEEDBACK_IMAGE_NOT_FOUND", "feedback image not found")
		}
		return nil, fmt.Errorf("resolve feedback image: %w", err)
	}
	return access, nil
}

func createFeedbackEvent(ctx context.Context, tx *dbent.Tx, feedbackID int64, eventType, actorType string, actorID *int64, summary string, metadata map[string]string) error {
	if metadata == nil {
		metadata = map[string]string{}
	}
	_, err := tx.FeedbackEvent.Create().SetFeedbackID(feedbackID).SetEventType(eventType).SetActorType(actorType).
		SetNillableActorUserID(positiveUserID(actorID)).SetSummary(summary).SetMetadata(metadata).Save(ctx)
	return err
}

func commitFeedbackEvent(ctx context.Context, tx *dbent.Tx, feedbackID int64, eventType, actorType string, actorID *int64, summary, note string) error {
	metadata := map[string]string{}
	if note != "" {
		metadata["note"] = note
	}
	if err := createFeedbackEvent(ctx, tx, feedbackID, eventType, actorType, actorID, summary, metadata); err != nil {
		return err
	}
	return tx.Commit()
}

func createUserNotification(ctx context.Context, tx *dbent.Tx, userID, feedbackID int64, typ, title, body, dedupe string) error {
	_, err := tx.UserNotification.Create().SetUserID(userID).SetFeedbackID(feedbackID).SetType(typ).SetTitle(title).SetBody(body).
		SetActionURL("/feedbacks/" + strconv.FormatInt(feedbackID, 10)).SetDedupeKey(dedupe).Save(ctx)
	return err
}

func positiveUserID(id *int64) *int64 {
	if id != nil && *id > 0 {
		return id
	}
	return nil
}

func (s *FeedbackService) invalidateRewardCaches(ctx context.Context, userID int64) {
	cacheCtx := context.WithoutCancel(ctx)
	cacheCtx, cancel := context.WithTimeout(cacheCtx, 5*time.Second)
	defer cancel()
	if s.authInvalidator != nil {
		s.authInvalidator.InvalidateAuthCacheByUserID(cacheCtx, userID)
	}
	if s.billingCache != nil {
		_ = s.billingCache.InvalidateUserBalance(cacheCtx, userID)
	}
}

func legacyPriorityFromTriage(priority string) string {
	switch priority {
	case domain.FeedbackTriagePriorityP0:
		return domain.FeedbackPriorityUrgent
	case domain.FeedbackTriagePriorityP1:
		return domain.FeedbackPriorityHigh
	case domain.FeedbackTriagePriorityP2:
		return domain.FeedbackPriorityNormal
	default:
		return domain.FeedbackPriorityLow
	}
}

func translateFeedbackEntError(err error) error {
	if dbent.IsNotFound(err) {
		return domain.ErrFeedbackNotFound
	}
	return err
}

func feedbackEntityForWorkflow(item *dbent.Feedback) *Feedback {
	if item == nil {
		return nil
	}
	return &Feedback{ID: item.ID, UserID: item.UserID, Category: item.Category, Title: item.Title, Content: item.Content, Images: append([]string(nil), item.Images...), Contact: item.Contact, RequestID: item.RequestID, Priority: item.Priority, Status: item.Status, TriageStatus: item.TriageStatus, TriagePriority: item.TriagePriority, TriageSummary: item.TriageSummary, TriageConfidence: item.TriageConfidence, RepairDifficulty: item.RepairDifficulty, RepairRecommendation: item.RepairRecommendation, OwnerDecision: item.OwnerDecision, FixStatus: item.FixStatus, DuplicateOfID: item.DuplicateOfID, ResolvedVersion: item.ResolvedVersion, AcceptedAt: item.AcceptedAt, ResolvedAt: item.ResolvedAt, VerifiedAt: item.VerifiedAt, ReplyCount: item.ReplyCount, LastReplyAt: item.LastReplyAt, LastReplyRole: item.LastReplyRole, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt, DeletedAt: item.DeletedAt}
}

func feedbackEventFromEntity(item *dbent.FeedbackEvent) FeedbackEvent {
	return FeedbackEvent{ID: item.ID, FeedbackID: item.FeedbackID, EventType: item.EventType, ActorType: item.ActorType, ActorUserID: item.ActorUserID, Summary: item.Summary, Metadata: item.Metadata, CreatedAt: item.CreatedAt}
}
func feedbackRewardFromEntity(item *dbent.FeedbackReward) FeedbackReward {
	return FeedbackReward{ID: item.ID, FeedbackID: item.FeedbackID, UserID: item.UserID, Amount: item.Amount, Reason: item.Reason, BatchID: item.BatchID, OperatorUserID: item.OperatorUserID, AccountChangeRecordID: item.AccountChangeRecordID, GrantedAt: item.GrantedAt, CreatedAt: item.CreatedAt}
}
func userNotificationFromEntity(item *dbent.UserNotification) UserNotification {
	return UserNotification{ID: item.ID, UserID: item.UserID, FeedbackID: item.FeedbackID, Type: item.Type, Title: item.Title, Body: item.Body, ActionURL: item.ActionURL, ReadAt: item.ReadAt, CreatedAt: item.CreatedAt}
}
