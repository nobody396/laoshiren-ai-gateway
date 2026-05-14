package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/logger"
)

// Task type constants
const (
	TaskTypeVerifyCode    = "verify_code"
	TaskTypePasswordReset = "password_reset"
	TaskTypeBalanceAlert  = "balance_alert"
	TaskTypeFeedbackNew   = "feedback_new"
	TaskTypeFeedbackReply = "feedback_reply"
)

// EmailTask 邮件发送任务
type EmailTask struct {
	Email          string
	SiteName       string
	TaskType       string
	ResetURL       string
	FeedbackID     int64
	Title          string
	Category       string
	Priority       string
	Summary        string
	FeedbackURL    string
	Username       string
	UserEmail      string
	Balance        string
	AlertThreshold string
	TopUpURL       string
	AlertUserID    int64
}

// EmailQueueService 异步邮件队列服务
type EmailQueueService struct {
	emailService      *EmailService
	balanceAlertCache BalanceAlertCache
	taskChan          chan EmailTask
	wg                sync.WaitGroup
	stopChan          chan struct{}
	workers           int
}

// NewEmailQueueService 创建邮件队列服务
func NewEmailQueueService(emailService *EmailService, balanceAlertCache BalanceAlertCache, workers int) *EmailQueueService {
	if workers <= 0 {
		workers = 3 // 默认3个工作协程
	}

	service := &EmailQueueService{
		emailService:      emailService,
		balanceAlertCache: balanceAlertCache,
		taskChan:          make(chan EmailTask, 100), // 缓冲100个任务
		stopChan:          make(chan struct{}),
		workers:           workers,
	}

	// 启动工作协程
	service.start()

	return service
}

// start 启动工作协程
func (s *EmailQueueService) start() {
	for i := 0; i < s.workers; i++ {
		s.wg.Add(1)
		go s.worker(i)
	}
	logger.LegacyPrintf("service.email_queue", "[EmailQueue] Started %d workers", s.workers)
}

// worker 工作协程
func (s *EmailQueueService) worker(id int) {
	defer s.wg.Done()

	for {
		select {
		case task := <-s.taskChan:
			s.processTask(id, task)
		case <-s.stopChan:
			logger.LegacyPrintf("service.email_queue", "[EmailQueue] Worker %d stopping", id)
			return
		}
	}
}

// processTask 处理任务
func (s *EmailQueueService) processTask(workerID int, task EmailTask) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	switch task.TaskType {
	case TaskTypeVerifyCode:
		if err := s.emailService.SendVerifyCode(ctx, task.Email, task.SiteName); err != nil {
			logger.LegacyPrintf("service.email_queue", "[EmailQueue] Worker %d failed to send verify code to %s: %v", workerID, task.Email, err)
		} else {
			logger.LegacyPrintf("service.email_queue", "[EmailQueue] Worker %d sent verify code to %s", workerID, task.Email)
		}
	case TaskTypePasswordReset:
		if err := s.emailService.SendPasswordResetEmailWithCooldown(ctx, task.Email, task.SiteName, task.ResetURL); err != nil {
			logger.LegacyPrintf("service.email_queue", "[EmailQueue] Worker %d failed to send password reset to %s: %v", workerID, task.Email, err)
		} else {
			logger.LegacyPrintf("service.email_queue", "[EmailQueue] Worker %d sent password reset to %s", workerID, task.Email)
		}
	case TaskTypeBalanceAlert:
		if s.shouldSkipBalanceAlert(ctx, task) {
			logger.LegacyPrintf("service.email_queue", "[EmailQueue] Worker %d dropped stale balance alert for user %d", workerID, task.AlertUserID)
			return
		}
		if err := s.emailService.SendBalanceAlert(ctx, task.Email, task.SiteName, task.Username, task.Balance, task.AlertThreshold, task.TopUpURL); err != nil {
			logger.LegacyPrintf("service.email_queue", "[EmailQueue] Worker %d failed to send balance alert to %s: %v", workerID, task.Email, err)
			s.finishBalanceAlertTask(ctx, task, false)
		} else {
			logger.LegacyPrintf("service.email_queue", "[EmailQueue] Worker %d sent balance alert to %s", workerID, task.Email)
			s.finishBalanceAlertTask(ctx, task, true)
		}
	case TaskTypeFeedbackNew:
		if err := s.emailService.SendFeedbackNewEmail(ctx, task.Email, task.SiteName, task.Title, task.Category, task.Priority, task.Username, task.UserEmail, task.FeedbackURL); err != nil {
			logger.LegacyPrintf("service.email_queue", "[EmailQueue] Worker %d failed to send feedback_new to %s: %v", workerID, task.Email, err)
		} else {
			logger.LegacyPrintf("service.email_queue", "[EmailQueue] Worker %d sent feedback_new to %s", workerID, task.Email)
		}
	case TaskTypeFeedbackReply:
		if err := s.emailService.SendFeedbackReplyEmail(ctx, task.Email, task.SiteName, task.Title, task.Summary, task.FeedbackURL); err != nil {
			logger.LegacyPrintf("service.email_queue", "[EmailQueue] Worker %d failed to send feedback_reply to %s: %v", workerID, task.Email, err)
		} else {
			logger.LegacyPrintf("service.email_queue", "[EmailQueue] Worker %d sent feedback_reply to %s", workerID, task.Email)
		}
	default:
		logger.LegacyPrintf("service.email_queue", "[EmailQueue] Worker %d unknown task type: %s", workerID, task.TaskType)
	}
}

func (s *EmailQueueService) shouldSkipBalanceAlert(ctx context.Context, task EmailTask) bool {
	if s.balanceAlertCache == nil || task.AlertUserID <= 0 {
		return false
	}

	locked, err := s.balanceAlertCache.HasDispatchLock(ctx, task.AlertUserID)
	if err != nil {
		logger.LegacyPrintf("service.email_queue", "[EmailQueue] failed to check balance alert dispatch lock for user %d: %v", task.AlertUserID, err)
		return false
	}

	return !locked
}

func (s *EmailQueueService) finishBalanceAlertTask(ctx context.Context, task EmailTask, delivered bool) {
	if s.balanceAlertCache == nil || task.AlertUserID <= 0 {
		return
	}

	if delivered {
		if err := s.balanceAlertCache.SetNotified(ctx, task.AlertUserID); err != nil {
			logger.LegacyPrintf("service.email_queue", "[EmailQueue] failed to mark balance alert notified for user %d: %v", task.AlertUserID, err)
		}
	}

	if err := s.balanceAlertCache.ClearDispatchLock(ctx, task.AlertUserID); err != nil {
		logger.LegacyPrintf("service.email_queue", "[EmailQueue] failed to clear balance alert dispatch lock for user %d: %v", task.AlertUserID, err)
	}
}

// EnqueueVerifyCode 将验证码发送任务加入队列
func (s *EmailQueueService) EnqueueVerifyCode(email, siteName string) error {
	task := EmailTask{
		Email:    email,
		SiteName: siteName,
		TaskType: TaskTypeVerifyCode,
	}

	select {
	case s.taskChan <- task:
		logger.LegacyPrintf("service.email_queue", "[EmailQueue] Enqueued verify code task for %s", email)
		return nil
	default:
		return fmt.Errorf("email queue is full")
	}
}

// EnqueuePasswordReset 将密码重置邮件任务加入队列
func (s *EmailQueueService) EnqueuePasswordReset(email, siteName, resetURL string) error {
	task := EmailTask{
		Email:    email,
		SiteName: siteName,
		TaskType: TaskTypePasswordReset,
		ResetURL: resetURL,
	}

	select {
	case s.taskChan <- task:
		logger.LegacyPrintf("service.email_queue", "[EmailQueue] Enqueued password reset task for %s", email)
		return nil
	default:
		return fmt.Errorf("email queue is full")
	}
}

// EnqueueBalanceAlert 将余额预警邮件任务加入队列
func (s *EmailQueueService) EnqueueBalanceAlert(userID int64, email, siteName, username, balance, threshold, topUpURL string) error {
	task := EmailTask{
		Email:          email,
		SiteName:       siteName,
		TaskType:       TaskTypeBalanceAlert,
		Username:       username,
		Balance:        balance,
		AlertThreshold: threshold,
		TopUpURL:       topUpURL,
		AlertUserID:    userID,
	}
	select {
	case s.taskChan <- task:
		logger.LegacyPrintf("service.email_queue", "[EmailQueue] Enqueued balance alert task for %s", email)
		return nil
	default:
		return fmt.Errorf("email queue is full")
	}
}

func (s *EmailQueueService) EnqueueFeedbackNew(email, siteName string, feedbackID int64, title, category, priority, username, userEmail, feedbackURL string) error {
	task := EmailTask{
		Email:       email,
		SiteName:    siteName,
		TaskType:    TaskTypeFeedbackNew,
		FeedbackID:  feedbackID,
		Title:       title,
		Category:    category,
		Priority:    priority,
		FeedbackURL: feedbackURL,
		Username:    username,
		UserEmail:   userEmail,
	}
	select {
	case s.taskChan <- task:
		logger.LegacyPrintf("service.email_queue", "[EmailQueue] Enqueued feedback_new task for %s", email)
		return nil
	default:
		return fmt.Errorf("email queue is full")
	}
}

func (s *EmailQueueService) EnqueueFeedbackReply(email, siteName string, feedbackID int64, title, summary, feedbackURL string) error {
	task := EmailTask{
		Email:       email,
		SiteName:    siteName,
		TaskType:    TaskTypeFeedbackReply,
		FeedbackID:  feedbackID,
		Title:       title,
		Summary:     summary,
		FeedbackURL: feedbackURL,
	}
	select {
	case s.taskChan <- task:
		logger.LegacyPrintf("service.email_queue", "[EmailQueue] Enqueued feedback_reply task for %s", email)
		return nil
	default:
		return fmt.Errorf("email queue is full")
	}
}

// Stop 停止队列服务
func (s *EmailQueueService) Stop() {
	close(s.stopChan)
	s.wg.Wait()
	logger.LegacyPrintf("service.email_queue", "%s", "[EmailQueue] All workers stopped")
}
