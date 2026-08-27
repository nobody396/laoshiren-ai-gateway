package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

const (
	defaultGPTImageTaskSettlementInterval = 30 * time.Second
	defaultGPTImageTaskSettlementBatch    = 20
)

type GPTImageTaskSettlementService struct {
	taskRepo       GPTImageTaskRepository
	accountRepo    AccountRepository
	userSubRepo    UserSubscriptionRepository
	apiKeyService  *APIKeyService
	gatewayService *OpenAIGatewayService
	cfg            *config.Config

	stopCh chan struct{}
	doneCh chan struct{}
	once   sync.Once
	mu     sync.Mutex
}

func NewGPTImageTaskSettlementService(
	taskRepo GPTImageTaskRepository,
	accountRepo AccountRepository,
	userSubRepo UserSubscriptionRepository,
	apiKeyService *APIKeyService,
	gatewayService *OpenAIGatewayService,
	cfg *config.Config,
) *GPTImageTaskSettlementService {
	return &GPTImageTaskSettlementService{
		taskRepo:       taskRepo,
		accountRepo:    accountRepo,
		userSubRepo:    userSubRepo,
		apiKeyService:  apiKeyService,
		gatewayService: gatewayService,
		cfg:            cfg,
		stopCh:         make(chan struct{}),
		doneCh:         make(chan struct{}),
	}
}

func ProvideGPTImageTaskSettlementService(
	taskRepo GPTImageTaskRepository,
	accountRepo AccountRepository,
	userSubRepo UserSubscriptionRepository,
	apiKeyService *APIKeyService,
	gatewayService *OpenAIGatewayService,
	cfg *config.Config,
) *GPTImageTaskSettlementService {
	svc := NewGPTImageTaskSettlementService(taskRepo, accountRepo, userSubRepo, apiKeyService, gatewayService, cfg)
	svc.Start()
	return svc
}

func (s *GPTImageTaskSettlementService) Start() {
	if s == nil || !s.enabled() {
		if s != nil {
			close(s.doneCh)
		}
		return
	}
	go s.loop()
}

func (s *GPTImageTaskSettlementService) Stop() {
	if s == nil {
		return
	}
	s.once.Do(func() { close(s.stopCh) })
	select {
	case <-s.doneCh:
	case <-time.After(5 * time.Second):
	}
}

func (s *GPTImageTaskSettlementService) enabled() bool {
	if s.taskRepo == nil || s.accountRepo == nil || s.apiKeyService == nil || s.gatewayService == nil {
		return false
	}
	if s.cfg == nil {
		return true
	}
	return s.cfg.Gateway.GPTImageTaskSettlement.Enabled
}

func (s *GPTImageTaskSettlementService) interval() time.Duration {
	if s.cfg == nil || s.cfg.Gateway.GPTImageTaskSettlement.IntervalSeconds <= 0 {
		return defaultGPTImageTaskSettlementInterval
	}
	interval := time.Duration(s.cfg.Gateway.GPTImageTaskSettlement.IntervalSeconds) * time.Second
	if interval < 5*time.Second {
		return 5 * time.Second
	}
	return interval
}

func (s *GPTImageTaskSettlementService) batchSize() int {
	if s.cfg == nil || s.cfg.Gateway.GPTImageTaskSettlement.BatchSize <= 0 {
		return defaultGPTImageTaskSettlementBatch
	}
	if s.cfg.Gateway.GPTImageTaskSettlement.BatchSize > 100 {
		return 100
	}
	return s.cfg.Gateway.GPTImageTaskSettlement.BatchSize
}

func (s *GPTImageTaskSettlementService) loop() {
	defer close(s.doneCh)
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-timer.C:
			s.runOnce(context.Background())
			timer.Reset(s.interval())
		}
	}
}

func (s *GPTImageTaskSettlementService) runOnce(ctx context.Context) {
	if s == nil || !s.mu.TryLock() {
		return
	}
	defer s.mu.Unlock()

	runCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	tasks, err := s.taskRepo.ListPendingSettlement(runCtx, s.batchSize())
	if err != nil {
		logger.L().Error("gpt_image.settlement.list_failed", zap.Error(err))
		return
	}
	for _, task := range tasks {
		if task == nil {
			continue
		}
		select {
		case <-runCtx.Done():
			return
		default:
		}
		if err := s.settleTask(runCtx, task); err != nil {
			logger.L().Warn("gpt_image.settlement.task_failed",
				zap.String("task_id", task.TaskID),
				zap.Int64("user_id", task.UserID),
				zap.Int64("api_key_id", task.APIKeyID),
				zap.Error(err),
			)
		}
	}
}

func (s *GPTImageTaskSettlementService) settleTask(ctx context.Context, task *GPTImageTask) error {
	account, err := s.accountRepo.GetByID(ctx, task.AccountID)
	if err != nil {
		return err
	}
	result, err := s.gatewayService.FetchGPTImageTask(ctx, account, task.TaskID)
	if err != nil {
		if result != nil && result.ResponseStatus == 404 {
			_ = s.taskRepo.UpdateUpstreamStatus(ctx, task.TaskID, GPTImageTaskStatusFailed, err.Error())
		}
		return err
	}
	if result == nil {
		return nil
	}

	if result.ImageCount <= 0 {
		return nil
	}

	_, imageCount, err := s.gatewayService.StoreGPTImageTaskResult(ctx, task.TaskID, result.ResponseBody, "")
	if err != nil {
		return err
	}
	claimed, ok := s.gatewayService.ClaimGPTImageTaskBilling(task.TaskID)
	if !ok {
		return nil
	}

	apiKey, err := s.apiKeyService.GetByIDForHistoricalBilling(ctx, claimed.APIKeyID, claimed.UserID)
	if err != nil {
		s.gatewayService.ReleaseGPTImageTaskBillingClaim(task.TaskID)
		return err
	}
	if apiKey == nil || apiKey.User == nil {
		s.gatewayService.ReleaseGPTImageTaskBillingClaim(task.TaskID)
		return errors.New("gpt-image settlement api key or user not found")
	}

	var subscription *UserSubscription
	if apiKey.GroupID != nil && s.userSubRepo != nil {
		if sub, subErr := s.userSubRepo.GetActiveByUserIDAndGroupID(ctx, apiKeyBillingUserID(apiKey), *apiKey.GroupID); subErr == nil {
			subscription = sub
		}
	}

	result.Model = claimed.Model
	result.UpstreamModel = claimed.UpstreamModel
	result.ImageSize = claimed.Resolution
	result.ImageCount = imageCount
	result.RequestID = "gpt-image-task:" + task.TaskID
	if result.ImageCount <= 0 {
		result.ImageCount = claimed.ImageCount
	}

	if err := s.gatewayService.RecordUsage(ctx, &OpenAIRecordUsageInput{
		Result:             result,
		APIKey:             apiKey,
		User:               apiKey.User,
		Account:            &claimed.Account,
		Subscription:       subscription,
		InboundEndpoint:    claimed.InboundEndpoint,
		UpstreamEndpoint:   claimed.UpstreamEndpoint,
		UserAgent:          claimed.UserAgent,
		IPAddress:          claimed.IPAddress,
		RequestPayloadHash: claimed.RequestPayloadHash,
		APIKeyService:      s.apiKeyService,
	}); err != nil {
		s.gatewayService.ReleaseGPTImageTaskBillingClaim(task.TaskID)
		return err
	}
	s.gatewayService.MarkGPTImageTaskBilled(task.TaskID)
	return nil
}

func apiKeyBillingUserID(apiKey *APIKey) int64 {
	if apiKey != nil && apiKey.User != nil && apiKey.User.ID > 0 {
		return apiKey.User.ID
	}
	if apiKey != nil {
		return apiKey.UserID
	}
	return 0
}
