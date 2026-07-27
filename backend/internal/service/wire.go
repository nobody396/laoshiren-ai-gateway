package service

import (
	"context"
	"database/sql"
	"time"

	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/logger"
	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
)

// BuildInfo contains build information
type BuildInfo struct {
	Version   string
	BuildType string
}

// ProvidePricingService creates and initializes PricingService
func ProvidePricingService(cfg *config.Config, remoteClient PricingRemoteClient) (*PricingService, error) {
	return NewPricingService(cfg, remoteClient), nil
}

// ProvideUpdateService creates UpdateService with BuildInfo
func ProvideUpdateService(cache UpdateCache, githubClient GitHubReleaseClient, buildInfo BuildInfo) *UpdateService {
	return NewUpdateService(cache, githubClient, buildInfo.Version, buildInfo.BuildType)
}

// ProvideEmailQueueService creates EmailQueueService with default worker count
func ProvideEmailQueueService(emailService *EmailService, alertCache BalanceAlertCache) *EmailQueueService {
	return NewEmailQueueService(emailService, alertCache, 3)
}

// ProvideTokenRefreshService creates and starts TokenRefreshService
func ProvideTokenRefreshService(
	accountRepo AccountRepository,
	oauthService *OAuthService,
	openaiOAuthService *OpenAIOAuthService,
	geminiOAuthService *GeminiOAuthService,
	antigravityOAuthService *AntigravityOAuthService,
	cacheInvalidator TokenCacheInvalidator,
	schedulerCache SchedulerCache,
	cfg *config.Config,
	tempUnschedCache TempUnschedCache,
	privacyClientFactory PrivacyClientFactory,
	proxyRepo ProxyRepository,
	refreshAPI *OAuthRefreshAPI,
) *TokenRefreshService {
	svc := NewTokenRefreshService(accountRepo, oauthService, openaiOAuthService, geminiOAuthService, antigravityOAuthService, cacheInvalidator, schedulerCache, cfg, tempUnschedCache)
	// 注入 OpenAI privacy opt-out 依赖
	svc.SetPrivacyDeps(privacyClientFactory, proxyRepo)
	// 注入统一 OAuth 刷新 API（消除 TokenRefreshService 与 TokenProvider 之间的竞争条件）
	svc.SetRefreshAPI(refreshAPI)
	// 调用侧显式注入后台刷新策略，避免策略漂移
	svc.SetRefreshPolicy(DefaultBackgroundRefreshPolicy())
	return svc
}

// ProvideClaudeTokenProvider creates ClaudeTokenProvider with OAuthRefreshAPI injection
func ProvideClaudeTokenProvider(
	accountRepo AccountRepository,
	tokenCache GeminiTokenCache,
	oauthService *OAuthService,
	refreshAPI *OAuthRefreshAPI,
) *ClaudeTokenProvider {
	p := NewClaudeTokenProvider(accountRepo, tokenCache, oauthService)
	executor := NewClaudeTokenRefresher(oauthService)
	p.SetRefreshAPI(refreshAPI, executor)
	p.SetRefreshPolicy(ClaudeProviderRefreshPolicy())
	return p
}

// ProvideOpenAITokenProvider creates OpenAITokenProvider with OAuthRefreshAPI injection
func ProvideOpenAITokenProvider(
	accountRepo AccountRepository,
	tokenCache GeminiTokenCache,
	openaiOAuthService *OpenAIOAuthService,
	refreshAPI *OAuthRefreshAPI,
) *OpenAITokenProvider {
	p := NewOpenAITokenProvider(accountRepo, tokenCache, openaiOAuthService)
	executor := NewOpenAITokenRefresher(openaiOAuthService, accountRepo)
	p.SetRefreshAPI(refreshAPI, executor)
	p.SetRefreshPolicy(OpenAIProviderRefreshPolicy())
	return p
}

// ProvideGeminiTokenProvider creates GeminiTokenProvider with OAuthRefreshAPI injection
func ProvideGeminiTokenProvider(
	accountRepo AccountRepository,
	tokenCache GeminiTokenCache,
	geminiOAuthService *GeminiOAuthService,
	refreshAPI *OAuthRefreshAPI,
) *GeminiTokenProvider {
	p := NewGeminiTokenProvider(accountRepo, tokenCache, geminiOAuthService)
	executor := NewGeminiTokenRefresher(geminiOAuthService)
	p.SetRefreshAPI(refreshAPI, executor)
	p.SetRefreshPolicy(GeminiProviderRefreshPolicy())
	return p
}

// ProvideAntigravityTokenProvider creates AntigravityTokenProvider with OAuthRefreshAPI injection
func ProvideAntigravityTokenProvider(
	accountRepo AccountRepository,
	tokenCache GeminiTokenCache,
	antigravityOAuthService *AntigravityOAuthService,
	refreshAPI *OAuthRefreshAPI,
	tempUnschedCache TempUnschedCache,
) *AntigravityTokenProvider {
	p := NewAntigravityTokenProvider(accountRepo, tokenCache, antigravityOAuthService)
	executor := NewAntigravityTokenRefresher(antigravityOAuthService)
	p.SetRefreshAPI(refreshAPI, executor)
	p.SetRefreshPolicy(AntigravityProviderRefreshPolicy())
	p.SetTempUnschedCache(tempUnschedCache)
	return p
}

// ProvideOAuthRefreshAPI avoids Wire variadic resolution issues.
func ProvideOAuthRefreshAPI(accountRepo AccountRepository, tokenCache GeminiTokenCache) *OAuthRefreshAPI {
	return NewOAuthRefreshAPI(accountRepo, tokenCache)
}

// ProvideDashboardAggregationService 创建并启动仪表盘聚合服务
func ProvideDashboardAggregationService(repo DashboardAggregationRepository, timingWheel *TimingWheelService, cfg *config.Config) *DashboardAggregationService {
	svc := NewDashboardAggregationService(repo, timingWheel, cfg)
	return svc
}

// ProvideUsageCleanupService 创建并启动使用记录清理任务服务
func ProvideUsageCleanupService(repo UsageCleanupRepository, timingWheel *TimingWheelService, dashboardAgg *DashboardAggregationService, cfg *config.Config) *UsageCleanupService {
	svc := NewUsageCleanupService(repo, timingWheel, dashboardAgg, cfg)
	return svc
}

// ProvideAgentLevelEvaluatorService creates and starts the monthly agent level evaluator.
func ProvideAgentLevelEvaluatorService(commission *CommissionService, cfg *config.Config) *AgentLevelEvaluatorService {
	svc := NewAgentLevelEvaluatorService(commission, cfg)
	return svc
}

// ProvideAccountExpiryService creates and starts AccountExpiryService.
func ProvideAccountExpiryService(accountRepo AccountRepository) *AccountExpiryService {
	svc := NewAccountExpiryService(accountRepo, time.Minute)
	return svc
}

// ProvideSubscriptionExpiryService creates and starts SubscriptionExpiryService.
func ProvideSubscriptionExpiryService(userSubRepo UserSubscriptionRepository) *SubscriptionExpiryService {
	svc := NewSubscriptionExpiryService(userSubRepo, time.Minute)
	return svc
}

// ProvideTimingWheelService creates and starts TimingWheelService
func ProvideTimingWheelService() (*TimingWheelService, error) {
	svc, err := NewTimingWheelService()
	if err != nil {
		return nil, err
	}
	return svc, nil
}

// ProvideDeferredService creates and starts DeferredService
func ProvideDeferredService(accountRepo AccountRepository, timingWheel *TimingWheelService) *DeferredService {
	svc := NewDeferredService(accountRepo, timingWheel, 10*time.Second)
	return svc
}

// ProvideConcurrencyService creates ConcurrencyService and starts slot cleanup worker.
func ProvideConcurrencyService(cache ConcurrencyCache, accountRepo AccountRepository, cfg *config.Config) *ConcurrencyService {
	svc := NewConcurrencyService(cache)
	return svc
}

// ProvideUserMessageQueueService 创建用户消息串行队列服务并启动清理 worker
func ProvideUserMessageQueueService(cache UserMsgQueueCache, rpmCache RPMCache, cfg *config.Config) *UserMessageQueueService {
	svc := NewUserMessageQueueService(cache, rpmCache, &cfg.Gateway.UserMessageQueue)
	return svc
}

func ProvideUsageRecordWorkerPool(cfg *config.Config, accountingWorker *AccountingWorker) *UsageRecordWorkerPool {
	pool := NewUsageRecordWorkerPool(cfg)
	pool.accountingWorker = accountingWorker
	return pool
}

// ProvideSchedulerSnapshotService creates and starts SchedulerSnapshotService.
func ProvideSchedulerSnapshotService(
	cache SchedulerCache,
	outboxRepo SchedulerOutboxRepository,
	accountRepo AccountRepository,
	groupRepo GroupRepository,
	cfg *config.Config,
) *SchedulerSnapshotService {
	svc := NewSchedulerSnapshotService(cache, outboxRepo, accountRepo, groupRepo, cfg)
	return svc
}

// ProvideRateLimitService creates RateLimitService with optional dependencies.
func ProvideRateLimitService(
	accountRepo AccountRepository,
	usageRepo UsageLogRepository,
	cfg *config.Config,
	geminiQuotaService *GeminiQuotaService,
	tempUnschedCache TempUnschedCache,
	timeoutCounterCache TimeoutCounterCache,
	openAI403CounterCache OpenAI403CounterCache,
	settingService *SettingService,
	tokenCacheInvalidator TokenCacheInvalidator,
) *RateLimitService {
	svc := NewRateLimitService(accountRepo, usageRepo, cfg, geminiQuotaService, tempUnschedCache)
	svc.SetTimeoutCounterCache(timeoutCounterCache)
	svc.SetOpenAI403CounterCache(openAI403CounterCache)
	svc.SetSettingService(settingService)
	svc.SetTokenCacheInvalidator(tokenCacheInvalidator)
	return svc
}

// ProvideOpsMetricsCollector creates and starts OpsMetricsCollector.
func ProvideOpsMetricsCollector(
	opsRepo OpsRepository,
	settingRepo SettingRepository,
	accountRepo AccountRepository,
	concurrencyService *ConcurrencyService,
	db *sql.DB,
	redisClient *redis.Client,
	cfg *config.Config,
) *OpsMetricsCollector {
	collector := NewOpsMetricsCollector(opsRepo, settingRepo, accountRepo, concurrencyService, db, redisClient, cfg)
	return collector
}

// ProvideOpsAggregationService creates and starts OpsAggregationService (hourly/daily pre-aggregation).
func ProvideOpsAggregationService(
	opsRepo OpsRepository,
	settingRepo SettingRepository,
	db *sql.DB,
	redisClient *redis.Client,
	cfg *config.Config,
) *OpsAggregationService {
	svc := NewOpsAggregationService(opsRepo, settingRepo, db, redisClient, cfg)
	return svc
}

// ProvideOpsAlertEvaluatorService creates and starts OpsAlertEvaluatorService.
func ProvideOpsAlertEvaluatorService(
	opsService *OpsService,
	opsRepo OpsRepository,
	emailService *EmailService,
	redisClient *redis.Client,
	cfg *config.Config,
) *OpsAlertEvaluatorService {
	svc := NewOpsAlertEvaluatorService(opsService, opsRepo, emailService, redisClient, cfg)
	return svc
}

// ProvideOpsCleanupService creates and starts OpsCleanupService (cron scheduled).
func ProvideOpsCleanupService(
	opsRepo OpsRepository,
	db *sql.DB,
	redisClient *redis.Client,
	cfg *config.Config,
) *OpsCleanupService {
	svc := NewOpsCleanupService(opsRepo, db, redisClient, cfg)
	return svc
}

func ProvideOpsSystemLogSink(opsRepo OpsRepository) *OpsSystemLogSink {
	sink := NewOpsSystemLogSink(opsRepo)
	return sink
}

func buildIdempotencyConfig(cfg *config.Config) IdempotencyConfig {
	idempotencyCfg := DefaultIdempotencyConfig()
	if cfg != nil {
		if cfg.Idempotency.DefaultTTLSeconds > 0 {
			idempotencyCfg.DefaultTTL = time.Duration(cfg.Idempotency.DefaultTTLSeconds) * time.Second
		}
		if cfg.Idempotency.SystemOperationTTLSeconds > 0 {
			idempotencyCfg.SystemOperationTTL = time.Duration(cfg.Idempotency.SystemOperationTTLSeconds) * time.Second
		}
		if cfg.Idempotency.ProcessingTimeoutSeconds > 0 {
			idempotencyCfg.ProcessingTimeout = time.Duration(cfg.Idempotency.ProcessingTimeoutSeconds) * time.Second
		}
		if cfg.Idempotency.FailedRetryBackoffSeconds > 0 {
			idempotencyCfg.FailedRetryBackoff = time.Duration(cfg.Idempotency.FailedRetryBackoffSeconds) * time.Second
		}
		if cfg.Idempotency.MaxStoredResponseLen > 0 {
			idempotencyCfg.MaxStoredResponseLen = cfg.Idempotency.MaxStoredResponseLen
		}
		idempotencyCfg.ObserveOnly = cfg.Idempotency.ObserveOnly
	}
	return idempotencyCfg
}

func ProvideIdempotencyCoordinator(repo IdempotencyRepository, cfg *config.Config) *IdempotencyCoordinator {
	coordinator := NewIdempotencyCoordinator(repo, buildIdempotencyConfig(cfg))
	SetDefaultIdempotencyCoordinator(coordinator)
	return coordinator
}

func ProvideSystemOperationLockService(repo IdempotencyRepository, cfg *config.Config) *SystemOperationLockService {
	return NewSystemOperationLockService(repo, buildIdempotencyConfig(cfg))
}

func ProvideIdempotencyCleanupService(repo IdempotencyRepository, cfg *config.Config) *IdempotencyCleanupService {
	svc := NewIdempotencyCleanupService(repo, cfg)
	return svc
}

// ProvideScheduledTestService creates ScheduledTestService.
func ProvideScheduledTestService(
	planRepo ScheduledTestPlanRepository,
	resultRepo ScheduledTestResultRepository,
) *ScheduledTestService {
	return NewScheduledTestService(planRepo, resultRepo)
}

// ProvideScheduledTestRunnerService creates and starts ScheduledTestRunnerService.
func ProvideScheduledTestRunnerService(
	planRepo ScheduledTestPlanRepository,
	scheduledSvc *ScheduledTestService,
	accountTestSvc *AccountTestService,
	rateLimitSvc *RateLimitService,
	cfg *config.Config,
) *ScheduledTestRunnerService {
	svc := NewScheduledTestRunnerService(planRepo, scheduledSvc, accountTestSvc, rateLimitSvc, cfg)
	return svc
}

// ProvideOpsScheduledReportService creates and starts OpsScheduledReportService.
func ProvideOpsScheduledReportService(
	opsService *OpsService,
	userService *UserService,
	emailService *EmailService,
	redisClient *redis.Client,
	cfg *config.Config,
) *OpsScheduledReportService {
	svc := NewOpsScheduledReportService(opsService, userService, emailService, redisClient, cfg)
	return svc
}

func ProvideDownloadResourceService(cfg *config.Config, githubClient GitHubReleaseClient) *DownloadResourceService {
	svc := NewDownloadResourceService(cfg, githubClient)
	return svc
}

// ProvideAPIKeyAuthCacheInvalidator 提供 API Key 认证缓存失效能力
func ProvideAPIKeyAuthCacheInvalidator(apiKeyService *APIKeyService) APIKeyAuthCacheInvalidator {
	return apiKeyService
}

// ProvideBackupService creates and starts BackupService
func ProvideBackupService(
	settingRepo SettingRepository,
	cfg *config.Config,
	encryptor SecretEncryptor,
	storeFactory BackupObjectStoreFactory,
	dumper DBDumper,
) *BackupService {
	svc := NewBackupService(settingRepo, cfg, encryptor, storeFactory, dumper)
	return svc
}

// ProvideSettingService wires SettingService with group reader, proxy repo, and update callbacks.
func ProvideSettingService(
	settingRepo SettingRepository,
	groupRepo GroupRepository,
	proxyRepo ProxyRepository,
	cfg *config.Config,
	balanceAlertService *BalanceAlertService,
) *SettingService {
	svc := NewSettingService(settingRepo, cfg)
	svc.SetDefaultSubscriptionGroupReader(groupRepo)
	svc.SetProxyRepository(proxyRepo)
	if balanceAlertService != nil {
		svc.AddOnUpdateCallback(func() {
			balanceAlertService.ClearGlobalCache(context.Background())
		})
	}
	return svc
}

func ProvideUserService(userRepo UserRepository, settingRepo SettingRepository, authCacheInvalidator APIKeyAuthCacheInvalidator, billingCache BillingCache) *UserService {
	svc := NewUserService(userRepo, authCacheInvalidator, billingCache)
	svc.SetSettingRepo(settingRepo)
	return svc
}

func ProvideOpenAIGatewayService(
	accountRepo AccountRepository,
	usageLogRepo UsageLogRepository,
	usageBillingRepo UsageBillingRepository,
	userRepo UserRepository,
	userSubRepo UserSubscriptionRepository,
	userGroupRateRepo UserGroupRateRepository,
	cache GatewayCache,
	cfg *config.Config,
	schedulerSnapshot *SchedulerSnapshotService,
	concurrencyService *ConcurrencyService,
	billingService *BillingService,
	rateLimitService *RateLimitService,
	billingCacheService *BillingCacheService,
	httpUpstream HTTPUpstream,
	deferredService *DeferredService,
	openAITokenProvider *OpenAITokenProvider,
	resolver *ModelPricingResolver,
	channelService *ChannelService,
	accountQuotaAlertService *AccountQuotaAlertService,
	balanceAlertService *BalanceAlertService,
	commissionService *CommissionService,
	gptImageTaskRepo GPTImageTaskRepository,
	gptImageS3Storage *GPTImageS3Storage,
	settingService *SettingService,
	accountingService *AccountingService,
) *OpenAIGatewayService {
	svc := NewOpenAIGatewayService(
		accountRepo, usageLogRepo, usageBillingRepo, userRepo, userSubRepo,
		userGroupRateRepo, cache, cfg, schedulerSnapshot, concurrencyService,
		billingService, rateLimitService, billingCacheService, httpUpstream,
		deferredService, openAITokenProvider, resolver, channelService,
		accountQuotaAlertService, balanceAlertService, commissionService,
		gptImageTaskRepo, gptImageS3Storage, settingService,
	)
	svc.SetGatewayPipeline(NewGatewayPipeline(
		PreselectedGatewaySelector{},
		PreserveGatewayFailover{},
		NewAccountingPipelineMeter(accountingService),
	))
	return svc
}

// ProvideAuthService injects the application UnitOfWork without expanding the
// legacy constructor used by focused unit tests.
func ProvideAuthService(
	entClient *dbent.Client,
	userRepo UserRepository,
	redeemRepo RedeemCodeRepository,
	refreshTokenCache RefreshTokenCache,
	ssoTicketCache SSOTicketCache,
	cfg *config.Config,
	settingService *SettingService,
	emailService *EmailService,
	turnstileService *TurnstileService,
	emailQueueService *EmailQueueService,
	promoService *PromoService,
	defaultSubAssigner DefaultSubscriptionAssigner,
	commissionService *CommissionService,
	unitOfWork UnitOfWork,
) *AuthService {
	svc := NewAuthService(
		entClient,
		userRepo,
		redeemRepo,
		refreshTokenCache,
		ssoTicketCache,
		cfg,
		settingService,
		emailService,
		turnstileService,
		emailQueueService,
		promoService,
		defaultSubAssigner,
		commissionService,
	)
	svc.SetUnitOfWork(unitOfWork)
	return svc
}

// ProviderSet is the Wire provider set for all services
var ProviderSet = wire.NewSet(
	// Core services
	ProvideAuthService,
	ProvideUserService,
	NewAPIKeyService,
	NewClientSetupService,
	ProvideAPIKeyAuthCacheInvalidator,
	NewGroupService,
	NewAccountService,
	NewProxyService,
	NewRedeemService,
	NewPromoService,
	NewInvoiceService,
	ProvideUsageService,
	NewDashboardService,
	ProvidePricingService,
	NewBillingService,
	NewBillingCacheService,
	NewAnnouncementService,
	NewChangelogService,
	NewFinanceTransactionService,
	NewFeedbackService,
	NewFeedbackImageStorage,
	wire.Bind(new(FeedbackImageStorage), new(*DisabledFeedbackImageStorage)),
	NewGPTImageS3Storage,
	NewFinanceReceiptS3Storage,
	ProvideGPTImageTaskSettlementService,
	NewAdminService,
	NewGatewayService,
	ProvideOpenAIGatewayService,
	NewOAuthService,
	NewOpenAIOAuthService,
	NewGeminiOAuthService,
	NewGeminiQuotaService,
	NewCompositeTokenCacheInvalidator,
	wire.Bind(new(TokenCacheInvalidator), new(*CompositeTokenCacheInvalidator)),
	NewAntigravityOAuthService,
	ProvideOAuthRefreshAPI,
	ProvideGeminiTokenProvider,
	NewGeminiMessagesCompatService,
	ProvideAntigravityTokenProvider,
	ProvideOpenAITokenProvider,
	ProvideClaudeTokenProvider,
	NewAntigravityGatewayService,
	ProvideRateLimitService,
	NewAccountUsageService,
	NewAccountTestService,
	ProvideSettingService,
	NewDataManagementService,
	ProvideBackupService,
	ProvideOpsSystemLogSink,
	NewOpsService,
	ProvideOpsMetricsCollector,
	ProvideOpsAggregationService,
	ProvideOpsAlertEvaluatorService,
	ProvideOpsCleanupService,
	ProvideOpsScheduledReportService,
	NewEmailService,
	ProvideEmailQueueService,
	NewTurnstileService,
	NewSubscriptionService,
	wire.Bind(new(DefaultSubscriptionAssigner), new(*SubscriptionService)),
	ProvideConcurrencyService,
	ProvideUserMessageQueueService,
	NewAccountingService,
	NewAccountingWorker,
	ProvideUsageRecordWorkerPool,
	ProvideSchedulerSnapshotService,
	NewIdentityService,
	NewCRSSyncService,
	ProvideUpdateService,
	ProvideDownloadResourceService,
	ProvideTokenRefreshService,
	ProvideAccountExpiryService,
	ProvideSubscriptionExpiryService,
	ProvideTimingWheelService,
	ProvideDashboardAggregationService,
	ProvideUsageCleanupService,
	ProvideAgentLevelEvaluatorService,
	ProvideDeferredService,
	NewAntigravityQuotaFetcher,
	NewUserAttributeService,
	NewUsageCache,
	NewTotpService,
	NewErrorPassthroughService,
	NewTLSFingerprintProfileService,
	NewDigestSessionStore,
	ProvideIdempotencyCoordinator,
	ProvideSystemOperationLockService,
	ProvideIdempotencyCleanupService,
	ProvidePendingAuthSessionCleanupService,
	ProvideScheduledTestService,
	ProvideScheduledTestRunnerService,
	NewGroupCapacityService,
	NewChannelService,
	ProvideSupplierService,
	NewModelPricingResolver,
	NewCommissionService,
	NewAffiliateProgramService,
	NewPaymentService,
	NewTopupService,
	NewRBACService,
	ProvideAccountQuotaAlertService,
	ProvideBalanceAlertService,
	ProvideRootLifecycle,
)

func ProvideSupplierService(repo SupplierRepository, accountRepo AccountRepository) *SupplierService {
	svc := NewSupplierService(repo, accountRepo)
	svc.StartProbeRunner()
	return svc
}

func ProvideAccountQuotaAlertService(emailService *EmailService, settingRepo SettingRepository, accountRepo AccountRepository) *AccountQuotaAlertService {
	return NewAccountQuotaAlertService(emailService, settingRepo, accountRepo)
}

func ProvideBalanceAlertService(
	alertCache BalanceAlertCache,
	attrDefRepo UserAttributeDefinitionRepository,
	attrValRepo UserAttributeValueRepository,
	settingRepo SettingRepository,
	emailQueue *EmailQueueService,
	userRepo UserRepository,
) *BalanceAlertService {
	return NewBalanceAlertService(alertCache, attrDefRepo, attrValRepo, settingRepo, emailQueue, userRepo)
}

func ProvidePendingAuthSessionCleanupService(identityService *IdentityService) *PendingAuthSessionCleanupService {
	svc := NewPendingAuthSessionCleanupService(identityService, time.Hour)
	return svc
}

func ProvideRootLifecycle(
	cfg *config.Config,
	accountRepo AccountRepository,
	pricing *PricingService,
	apiKeyService *APIKeyService,
	billingCache *BillingCacheService,
	emailQueue *EmailQueueService,
	subscriptionService *SubscriptionService,
	accountingWorker *AccountingWorker,
	usageRecordPool *UsageRecordWorkerPool,
	timingWheel *TimingWheelService,
	dashboardAggregation *DashboardAggregationService,
	deferred *DeferredService,
	schedulerSnapshot *SchedulerSnapshotService,
	concurrencyService *ConcurrencyService,
	userMessageQueue *UserMessageQueueService,
	tokenRefresh *TokenRefreshService,
	accountExpiry *AccountExpiryService,
	subscriptionExpiry *SubscriptionExpiryService,
	usageCleanup *UsageCleanupService,
	agentLevelEvaluator *AgentLevelEvaluatorService,
	opsMetrics *OpsMetricsCollector,
	opsAggregation *OpsAggregationService,
	opsAlert *OpsAlertEvaluatorService,
	opsCleanup *OpsCleanupService,
	opsReport *OpsScheduledReportService,
	opsSink *OpsSystemLogSink,
	idempotencyCleanup *IdempotencyCleanupService,
	scheduledTests *ScheduledTestRunnerService,
	downloadResources *DownloadResourceService,
	backupService *BackupService,
	pendingAuthCleanup *PendingAuthSessionCleanupService,
) *Lifecycle {
	component := func(name string, start func(), stop func()) LifecycleComponent {
		return LifecycleFunc{
			ComponentName: name,
			StartFunc: func(context.Context) error {
				if start != nil {
					start()
				}
				return nil
			},
			StopFunc: func(context.Context) error {
				if stop != nil {
					stop()
				}
				return nil
			},
		}
	}

	var authSubscriberCancel context.CancelFunc
	components := []LifecycleComponent{
		LifecycleFunc{ComponentName: "pricing", StartFunc: func(context.Context) error { return pricing.Initialize() }, StopFunc: func(context.Context) error { pricing.Stop(); return nil }},
		component("billing-cache", billingCache.Start, billingCache.Stop),
		component("email-queue", emailQueue.Start, emailQueue.Stop),
		component("subscription-maintenance", subscriptionService.Start, subscriptionService.Stop),
		component("accounting-worker", accountingWorker.Start, accountingWorker.Stop),
		component("usage-record-pool", usageRecordPool.Start, usageRecordPool.Stop),
		component("timing-wheel", timingWheel.Start, timingWheel.Stop),
		component("dashboard-aggregation", dashboardAggregation.Start, nil),
		component("deferred-writes", deferred.Start, deferred.Stop),
		component("scheduler-snapshot", schedulerSnapshot.Start, schedulerSnapshot.Stop),
		component("concurrency-cleanup", func() {
			if err := concurrencyService.CleanupStaleProcessSlots(context.Background()); err != nil {
				logger.LegacyPrintf("service.concurrency", "Warning: startup cleanup stale process slots failed: %v", err)
			}
			if cfg != nil {
				concurrencyService.StartSlotCleanupWorker(accountRepo, cfg.Gateway.Scheduling.SlotCleanupInterval)
			}
		}, concurrencyService.StopSlotCleanupWorker),
		component("user-message-cleanup", func() {
			if cfg != nil && cfg.Gateway.UserMessageQueue.CleanupIntervalSeconds > 0 {
				userMessageQueue.StartCleanupWorker(time.Duration(cfg.Gateway.UserMessageQueue.CleanupIntervalSeconds) * time.Second)
			}
		}, userMessageQueue.Stop),
		component("api-key-auth-subscriber", func() {
			ctx, cancel := context.WithCancel(context.Background())
			authSubscriberCancel = cancel
			apiKeyService.StartAuthCacheInvalidationSubscriber(ctx)
		}, func() {
			if authSubscriberCancel != nil {
				authSubscriberCancel()
			}
		}),
		component("token-refresh", tokenRefresh.Start, tokenRefresh.Stop),
		component("account-expiry", accountExpiry.Start, accountExpiry.Stop),
		component("subscription-expiry", subscriptionExpiry.Start, subscriptionExpiry.Stop),
		component("usage-cleanup", usageCleanup.Start, usageCleanup.Stop),
		component("agent-level-evaluator", agentLevelEvaluator.Start, agentLevelEvaluator.Stop),
		component("ops-metrics", opsMetrics.Start, opsMetrics.Stop),
		component("ops-aggregation", opsAggregation.Start, opsAggregation.Stop),
		component("ops-alert", opsAlert.Start, opsAlert.Stop),
		component("ops-cleanup", opsCleanup.Start, opsCleanup.Stop),
		component("ops-report", opsReport.Start, opsReport.Stop),
		component("ops-log-sink", func() { opsSink.Start(); logger.SetSink(opsSink) }, func() { logger.SetSink(nil); opsSink.Stop() }),
		component("idempotency-cleanup", idempotencyCleanup.Start, idempotencyCleanup.Stop),
		component("scheduled-tests", scheduledTests.Start, scheduledTests.Stop),
		component("download-resources", downloadResources.Start, downloadResources.Stop),
		component("backup", backupService.Start, backupService.Stop),
		component("pending-auth-cleanup", pendingAuthCleanup.Start, pendingAuthCleanup.Stop),
	}
	return NewLifecycle(components...)
}
