package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
)

var (
	ErrRedeemCodeNotFound  = infraerrors.NotFound("REDEEM_CODE_NOT_FOUND", "redeem code not found")
	ErrRedeemCodeUsed      = infraerrors.Conflict("REDEEM_CODE_USED", "redeem code already used")
	ErrRedeemOfferClaimed  = infraerrors.Conflict("REDEEM_OFFER_ALREADY_CLAIMED", "this offer can only be redeemed once per account")
	ErrInsufficientBalance = infraerrors.BadRequest("INSUFFICIENT_BALANCE", "insufficient balance")
	ErrRedeemRateLimited   = infraerrors.TooManyRequests("REDEEM_RATE_LIMITED", "too many failed attempts, please try again later")
	ErrRedeemCodeLocked    = infraerrors.Conflict("REDEEM_CODE_LOCKED", "redeem code is being processed, please try again")
)

const (
	redeemMaxErrorsPerHour  = 20
	redeemRateLimitDuration = time.Hour
	redeemLockDuration      = 10 * time.Second // 锁超时时间，防止死锁
)

// RedeemCache defines cache operations for redeem service
type RedeemCache interface {
	GetRedeemAttemptCount(ctx context.Context, userID int64) (int, error)
	IncrementRedeemAttemptCount(ctx context.Context, userID int64) error

	AcquireRedeemLock(ctx context.Context, code string, ttl time.Duration) (bool, error)
	ReleaseRedeemLock(ctx context.Context, code string) error
}

// NativeCheckoutRedeemPolicy describes whether a stocked card is reserved for
// checkout, whether the offer explicitly permits public manual redemption, and
// whether the current account has already consumed its lifetime claim.
type NativeCheckoutRedeemPolicy struct {
	Restricted          bool
	ManualRedeemEnabled bool
	AlreadyClaimed      bool
}

// NativeCheckoutRedeemGuard protects stocked card-shop inventory. Manual
// redemption is opt-in per offer; the database remains the final concurrent
// once-per-account authority when two different card codes are submitted at
// the same time.
type NativeCheckoutRedeemGuard interface {
	GetNativeCheckoutRedeemPolicy(ctx context.Context, redeemCodeID, userID int64) (NativeCheckoutRedeemPolicy, error)
}

type RedeemCodeRepository interface {
	Create(ctx context.Context, code *RedeemCode) error
	CreateBatch(ctx context.Context, codes []RedeemCode) error
	GetByID(ctx context.Context, id int64) (*RedeemCode, error)
	GetByCode(ctx context.Context, code string) (*RedeemCode, error)
	Update(ctx context.Context, code *RedeemCode) error
	Delete(ctx context.Context, id int64) error
	Use(ctx context.Context, id, userID int64) error

	List(ctx context.Context, params pagination.PaginationParams) ([]RedeemCode, *pagination.PaginationResult, error)
	ListWithFilters(ctx context.Context, params pagination.PaginationParams, codeType, status, search string) ([]RedeemCode, *pagination.PaginationResult, error)
	ListByUser(ctx context.Context, userID int64, limit int) ([]RedeemCode, error)
	// ListByUserPaginated returns paginated balance/concurrency history for a specific user.
	// codeType filter is optional - pass empty string to return all types.
	ListByUserPaginated(ctx context.Context, userID int64, params pagination.PaginationParams, codeType string) ([]RedeemCode, *pagination.PaginationResult, error)
	// SumPositiveBalanceByUser returns the total recharged amount (sum of positive balance values) for a user.
	SumPositiveBalanceByUser(ctx context.Context, userID int64) (float64, error)
	// SumGiftedRedeemValue 返回已赠送礼品/赔付卡密的面值总额（元）。
	// 口径：purpose IN (gift, compensation) 且 sales_status = gifted。
	SumGiftedRedeemValue(ctx context.Context) (float64, error)
}

// GenerateCodesRequest 生成兑换码请求
type GenerateCodesRequest struct {
	Count int     `json:"count"`
	Value float64 `json:"value"`
	Type  string  `json:"type"`
}

// RedeemCodeResponse 兑换码响应
type RedeemCodeResponse struct {
	Code      string    `json:"code"`
	Value     float64   `json:"value"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// RedeemService 兑换码服务
type RedeemService struct {
	redeemRepo           RedeemCodeRepository
	accountChangeRepo    AccountChangeRecordRepository
	userRepo             UserRepository
	subscriptionService  *SubscriptionService
	cache                RedeemCache
	billingCacheService  *BillingCacheService
	entClient            *dbent.Client
	authCacheInvalidator APIKeyAuthCacheInvalidator
	commissionService    *CommissionService
	balanceAlertService  *BalanceAlertService
	affiliateConsumption AffiliateConsumptionRepository
	affiliateRewards     *AffiliateRewardService
	nativeCheckoutGuard  NativeCheckoutRedeemGuard
}

func (s *RedeemService) SetNativeCheckoutRedeemGuard(guard NativeCheckoutRedeemGuard) {
	s.nativeCheckoutGuard = guard
}

// NewRedeemService 创建兑换码服务实例
func NewRedeemService(
	redeemRepo RedeemCodeRepository,
	accountChangeRepo AccountChangeRecordRepository,
	userRepo UserRepository,
	subscriptionService *SubscriptionService,
	cache RedeemCache,
	billingCacheService *BillingCacheService,
	entClient *dbent.Client,
	authCacheInvalidator APIKeyAuthCacheInvalidator,
	commissionService *CommissionService,
	balanceAlertService *BalanceAlertService,
	affiliateConsumption AffiliateConsumptionRepository,
	affiliateRewards *AffiliateRewardService,
) *RedeemService {
	return &RedeemService{
		redeemRepo:           redeemRepo,
		accountChangeRepo:    accountChangeRepo,
		userRepo:             userRepo,
		subscriptionService:  subscriptionService,
		cache:                cache,
		billingCacheService:  billingCacheService,
		entClient:            entClient,
		authCacheInvalidator: authCacheInvalidator,
		commissionService:    commissionService,
		balanceAlertService:  balanceAlertService,
		affiliateConsumption: affiliateConsumption,
		affiliateRewards:     affiliateRewards,
	}
}

// GenerateRandomCode 生成随机兑换码
func (s *RedeemService) GenerateRandomCode() (string, error) {
	// 生成16字节随机数据
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}

	// 转换为十六进制字符串
	code := hex.EncodeToString(bytes)

	// 格式化为 XXXX-XXXX-XXXX-XXXX 格式
	parts := []string{
		strings.ToUpper(code[0:8]),
		strings.ToUpper(code[8:16]),
		strings.ToUpper(code[16:24]),
		strings.ToUpper(code[24:32]),
	}

	return strings.Join(parts, "-"), nil
}

// GenerateCodes 批量生成兑换码
func (s *RedeemService) GenerateCodes(ctx context.Context, req GenerateCodesRequest) ([]RedeemCode, error) {
	if req.Count <= 0 {
		return nil, errors.New("count must be greater than 0")
	}

	// 邀请码类型不需要数值，其他类型需要
	if req.Type != RedeemTypeInvitation && req.Value <= 0 {
		return nil, errors.New("value must be greater than 0")
	}

	if req.Count > 1000 {
		return nil, errors.New("cannot generate more than 1000 codes at once")
	}

	codeType := req.Type
	if codeType == "" {
		codeType = RedeemTypeBalance
	}

	// 邀请码类型的 value 设为 0
	value := req.Value
	if codeType == RedeemTypeInvitation {
		value = 0
	}

	codes := make([]RedeemCode, 0, req.Count)
	for i := 0; i < req.Count; i++ {
		code, err := s.GenerateRandomCode()
		if err != nil {
			return nil, fmt.Errorf("generate code: %w", err)
		}

		codes = append(codes, RedeemCode{
			Code:   code,
			Type:   codeType,
			Value:  value,
			Status: StatusUnused,
		})
	}

	// 批量插入
	if err := s.redeemRepo.CreateBatch(ctx, codes); err != nil {
		return nil, fmt.Errorf("create batch codes: %w", err)
	}

	return codes, nil
}

// CreateCode creates a redeem code with caller-provided code value.
// It is primarily used by admin integrations that require an external order ID
// to be mapped to a deterministic redeem code.
func (s *RedeemService) CreateCode(ctx context.Context, code *RedeemCode) error {
	if code == nil {
		return errors.New("redeem code is required")
	}
	code.Code = strings.TrimSpace(code.Code)
	if code.Code == "" {
		return errors.New("code is required")
	}
	if code.Type == "" {
		code.Type = RedeemTypeBalance
	}
	if code.Type != RedeemTypeInvitation && code.Value <= 0 {
		return errors.New("value must be greater than 0")
	}
	if code.Type == RedeemTypeSubscription {
		groupIDs := subscriptionRedeemGroupIDs(code)
		if len(groupIDs) == 0 {
			return errors.New("group_id or group_ids is required for subscription type")
		}
		completed, currentMonthly, err := completeCurrentMonthlyCardGroupIDs(groupIDs)
		if err != nil {
			return err
		}
		groupIDs = completed
		if currentMonthly || code.GroupID == nil {
			primaryGroupID := groupIDs[0]
			code.GroupID = &primaryGroupID
		}
		code.GroupIDs = groupIDs
	}
	if code.Status == "" {
		code.Status = StatusUnused
	}

	if err := s.redeemRepo.Create(ctx, code); err != nil {
		return fmt.Errorf("create redeem code: %w", err)
	}
	return nil
}

// checkRedeemRateLimit 检查用户兑换错误次数是否超限
func (s *RedeemService) checkRedeemRateLimit(ctx context.Context, userID int64) error {
	if s.cache == nil {
		return nil
	}

	count, err := s.cache.GetRedeemAttemptCount(ctx, userID)
	if err != nil {
		// Redis 出错时不阻止用户操作
		return nil
	}

	if count >= redeemMaxErrorsPerHour {
		return ErrRedeemRateLimited
	}

	return nil
}

// incrementRedeemErrorCount 增加用户兑换错误计数
func (s *RedeemService) incrementRedeemErrorCount(ctx context.Context, userID int64) {
	if s.cache == nil {
		return
	}

	_ = s.cache.IncrementRedeemAttemptCount(ctx, userID)
}

// acquireRedeemLock 尝试获取兑换码的分布式锁
// 返回 true 表示获取成功，false 表示锁已被占用
func (s *RedeemService) acquireRedeemLock(ctx context.Context, code string) bool {
	if s.cache == nil {
		return true // 无 Redis 时降级为不加锁
	}

	ok, err := s.cache.AcquireRedeemLock(ctx, code, redeemLockDuration)
	if err != nil {
		// Redis 出错时不阻止操作，依赖数据库层面的状态检查
		return true
	}
	return ok
}

// releaseRedeemLock 释放兑换码的分布式锁
func (s *RedeemService) releaseRedeemLock(ctx context.Context, code string) {
	if s.cache == nil {
		return
	}

	_ = s.cache.ReleaseRedeemLock(ctx, code)
}

// Redeem 使用兑换码
func (s *RedeemService) Redeem(ctx context.Context, userID int64, code string) (*RedeemCode, error) {
	// 检查限流
	if err := s.checkRedeemRateLimit(ctx, userID); err != nil {
		return nil, err
	}

	// 获取分布式锁，防止同一兑换码并发使用
	if !s.acquireRedeemLock(ctx, code) {
		return nil, ErrRedeemCodeLocked
	}
	defer s.releaseRedeemLock(ctx, code)

	// 查找兑换码
	redeemCode, err := s.redeemRepo.GetByCode(ctx, code)
	if err != nil {
		if errors.Is(err, ErrRedeemCodeNotFound) {
			s.incrementRedeemErrorCount(ctx, userID)
			return nil, ErrRedeemCodeNotFound
		}
		return nil, fmt.Errorf("get redeem code: %w", err)
	}
	if s.nativeCheckoutGuard != nil {
		policy, guardErr := s.nativeCheckoutGuard.GetNativeCheckoutRedeemPolicy(ctx, redeemCode.ID, userID)
		if guardErr != nil {
			return nil, fmt.Errorf("check native checkout redeem restriction: %w", guardErr)
		}
		if policy.Restricted && !nativeCheckoutRedeemAuthorized(ctx) && !policy.ManualRedeemEnabled {
			s.incrementRedeemErrorCount(ctx, userID)
			return nil, infraerrors.BadRequest("REDEEM_CODE_CHECKOUT_RESTRICTED", "this code is fulfilled automatically by its checkout order")
		}
		if policy.Restricted && !nativeCheckoutRedeemAuthorized(ctx) && policy.AlreadyClaimed {
			s.incrementRedeemErrorCount(ctx, userID)
			return nil, ErrRedeemOfferClaimed
		}
	}

	// 检查兑换码状态
	if !redeemCode.CanUse() {
		s.incrementRedeemErrorCount(ctx, userID)
		return nil, ErrRedeemCodeUsed
	}

	// 验证兑换码类型的前置条件
	if redeemCode.Type == RedeemTypeSubscription && len(subscriptionRedeemGroupIDs(redeemCode)) == 0 {
		return nil, infraerrors.BadRequest("REDEEM_CODE_INVALID", "invalid subscription redeem code: missing group_id")
	}

	// 获取用户信息
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	_ = user // 使用变量避免未使用错误

	// 使用数据库事务保证兑换码标记与权益发放的原子性
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// 将事务放入 context，使 repository 方法能够使用同一事务
	txCtx := dbent.NewTxContext(ctx, tx)

	// 【关键】先标记兑换码为已使用，确保并发安全
	// 利用数据库乐观锁（WHERE status = 'unused'）保证原子性
	if err := s.redeemRepo.Use(txCtx, redeemCode.ID, userID); err != nil {
		if errors.Is(err, ErrRedeemCodeNotFound) || errors.Is(err, ErrRedeemCodeUsed) {
			return nil, ErrRedeemCodeUsed
		}
		if errors.Is(err, ErrRedeemOfferClaimed) {
			return nil, ErrRedeemOfferClaimed
		}
		return nil, fmt.Errorf("mark code as used: %w", err)
	}

	// Use may transition a paid sale_recharge code from inventory to sold.
	// Reload it inside the same transaction before deriving affiliate policy so
	// the balance lot reflects the persisted sale state rather than the stale
	// pre-use snapshot.
	redeemCode, err = s.redeemRepo.GetByID(txCtx, redeemCode.ID)
	if err != nil {
		return nil, fmt.Errorf("reload redeemed code: %w", err)
	}
	if redeemCode.Type == RedeemTypeSubscription {
		if _, err := s.completeRedeemedMonthlyCardBundle(txCtx, redeemCode); err != nil {
			return nil, err
		}
	}

	// 执行兑换逻辑（兑换码已被锁定，此时可安全操作）
	switch redeemCode.Type {
	case RedeemTypeBalance:
		// 增加用户余额
		if err := s.userRepo.UpdateBalance(txCtx, userID, redeemCode.Value); err != nil {
			return nil, fmt.Errorf("update user balance: %w", err)
		}
		if s.affiliateConsumption != nil {
			sourceType, paid := AffiliateSourceFromRedeem(redeemCode.Purpose, redeemCode.SalesStatus)
			if affiliateCommissionPurchaseAuthorized(txCtx) {
				paid = false
			}
			paidValue := RedeemPaidValue(redeemCode)
			occurredAt := time.Now()
			var rewardResult *AffiliateFirstPaidPurchaseResult
			if paid && s.affiliateRewards != nil {
				rewardResult, err = s.affiliateRewards.ProcessFirstPaidPurchase(txCtx, AffiliateFirstPaidPurchaseInput{
					UserID:       userID,
					PurchaseType: AffiliatePurchaseBalanceRedeem,
					SourceID:     redeemCode.ID,
					PurchaseKey:  fmt.Sprintf("redeem:balance:%d", redeemCode.ID),
					AmountMicros: AffiliateMicrosFromFloat(paidValue),
					OccurredAt:   occurredAt,
				})
				if err != nil {
					return nil, fmt.Errorf("process affiliate first paid balance purchase: %w", err)
				}
			}
			policy, partnerID, customerRate, partnerRate := AffiliatePolicyFromPurchaseResult(paid, rewardResult)
			for _, lot := range buildRedeemBalanceLots(redeemCode, userID, sourceType, paid, policy, partnerID, customerRate, partnerRate, occurredAt) {
				if err := s.affiliateConsumption.RecordBalanceLot(txCtx, lot); err != nil {
					return nil, fmt.Errorf("record affiliate balance lot: %w", err)
				}
			}
		}

	case RedeemTypeConcurrency:
		// 增加用户并发数
		if err := s.userRepo.UpdateConcurrency(txCtx, userID, int(redeemCode.Value)); err != nil {
			return nil, fmt.Errorf("update user concurrency: %w", err)
		}

	case RedeemTypeSubscription:
		validityDays := redeemCode.ValidityDays
		if validityDays <= 0 {
			validityDays = 30
		}
		groupIDs := subscriptionRedeemGroupIDs(redeemCode)
		notes := subscriptionRedeemNotes(redeemCode.Code, len(groupIDs) > 1)
		affiliateSubscriptions := make([]AffiliateMonthlySubscription, 0, len(groupIDs))
		affiliateGroups := make([]*Group, 0, len(groupIDs))
		var cycleStartsAt time.Time
		var cycleEndsAt time.Time
		for _, groupID := range groupIDs {
			subscription, _, err := s.subscriptionService.AssignOrExtendSubscription(txCtx, &AssignSubscriptionInput{
				UserID:       userID,
				GroupID:      groupID,
				ValidityDays: validityDays,
				AssignedBy:   0, // 系统分配
				Notes:        notes,
				// 复购兑换即开启新计费周期：延长到期时间的同时清零已用额度
				ResetQuotaOnExtend: true,
			})
			if err != nil {
				return nil, fmt.Errorf("assign or extend subscription group %d: %w", groupID, err)
			}
			group, err := s.subscriptionService.groupRepo.GetByID(txCtx, groupID)
			if err != nil {
				return nil, fmt.Errorf("load subscription group %d for affiliate attribution: %w", groupID, err)
			}
			affiliateGroups = append(affiliateGroups, group)
			affiliateSubscriptions = append(affiliateSubscriptions, AffiliateMonthlySubscription{
				UserSubscriptionID: subscription.ID,
				GroupID:            groupID,
			})
			subscriptionCycleStart := subscription.ExpiresAt.AddDate(0, 0, -validityDays)
			if cycleStartsAt.IsZero() || subscriptionCycleStart.After(cycleStartsAt) {
				cycleStartsAt = subscriptionCycleStart
			}
			if cycleEndsAt.IsZero() || subscription.ExpiresAt.Before(cycleEndsAt) {
				cycleEndsAt = subscription.ExpiresAt
			}
		}
		if s.affiliateConsumption != nil {
			sourceType, paid := AffiliateSourceFromRedeem(redeemCode.Purpose, redeemCode.SalesStatus)
			if affiliateCommissionPurchaseAuthorized(txCtx) {
				paid = false
			}
			creditLimitMicros := AffiliateMonthlyCreditLimitMicros(affiliateGroups, validityDays)
			salePriceMicros := AffiliateMicrosFromFloat(redeemCode.Value)
			if creditLimitMicros > 0 {
				occurredAt := time.Now()
				var rewardResult *AffiliateFirstPaidPurchaseResult
				if paid && salePriceMicros > 0 && s.affiliateRewards != nil {
					rewardResult, err = s.affiliateRewards.ProcessFirstPaidPurchase(txCtx, AffiliateFirstPaidPurchaseInput{
						UserID:       userID,
						PurchaseType: AffiliatePurchaseMonthlyRedeem,
						SourceID:     redeemCode.ID,
						PurchaseKey:  fmt.Sprintf("redeem:subscription:%d", redeemCode.ID),
						AmountMicros: salePriceMicros,
						OccurredAt:   occurredAt,
					})
					if err != nil {
						return nil, fmt.Errorf("process affiliate first paid monthly purchase: %w", err)
					}
				}
				policy, partnerID, customerRate, partnerRate := AffiliatePolicyFromPurchaseResult(
					paid && salePriceMicros > 0,
					rewardResult,
				)
				productCode, pricingTableVersion := AffiliateMonthlyCatalogIdentity(affiliateGroups)
				if productCode == "" {
					productCode = fmt.Sprintf("redeem-%d", redeemCode.ID)
				}
				if err := s.affiliateConsumption.RecordMonthlyEntitlement(txCtx, AffiliateMonthlyEntitlementInput{
					UserID:                   userID,
					SourceType:               sourceType,
					SourceID:                 redeemCode.ID,
					SourceKey:                fmt.Sprintf("redeem:subscription:%d", redeemCode.ID),
					ProductCode:              productCode,
					SalePriceMicros:          salePriceMicros,
					CreditLimitMicros:        creditLimitMicros,
					AffiliatePolicy:          policy,
					DirectPartnerID:          partnerID,
					CustomerRebateRateBPS:    customerRate,
					PartnerCommissionRateBPS: partnerRate,
					PricingTableVersion:      pricingTableVersion,
					StartsAt:                 cycleStartsAt,
					EndsAt:                   cycleEndsAt,
					Subscriptions:            affiliateSubscriptions,
				}); err != nil {
					return nil, fmt.Errorf("record affiliate monthly entitlement: %w", err)
				}
			}
		}

	default:
		return nil, fmt.Errorf("unsupported redeem type: %s", redeemCode.Type)
	}

	if s.accountChangeRepo != nil {
		now := time.Now()
		record := &AccountChangeRecord{
			UserID:      userID,
			AssetType:   accountChangeAssetTypeFromRedeemType(redeemCode.Type),
			Reason:      AccountChangeReasonRedeemCode,
			Delta:       redeemCodeDeltaForAccountChange(redeemCode),
			SourceType:  AccountChangeSourceRedeemCode,
			SourceID:    &redeemCode.ID,
			ReferenceNo: redeemCode.Code,
			Notes:       redeemCode.Notes,
			GroupID:     redeemCode.GroupID,
			ValidityDays: func() int {
				if redeemCode.Type == RedeemTypeSubscription {
					if redeemCode.ValidityDays > 0 {
						return redeemCode.ValidityDays
					}
					return 30
				}
				return 0
			}(),
			CreatedAt: now,
			DedupeKey: ptrString(fmt.Sprintf("redeem_code:%d", redeemCode.ID)),
		}
		if err := s.accountChangeRepo.Create(txCtx, record); err != nil {
			return nil, fmt.Errorf("create account change record: %w", err)
		}
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	// 事务提交成功后失效缓存
	s.invalidateRedeemCaches(ctx, userID, redeemCode)

	if redeemCode.Type == RedeemTypeBalance && s.balanceAlertService != nil {
		go func() {
			s.resetBalanceAlertNotifiedFlag(userID)
		}()
	}

	// 重新获取更新后的兑换码
	redeemCode, err = s.redeemRepo.GetByID(ctx, redeemCode.ID)
	if err != nil {
		return nil, fmt.Errorf("get updated redeem code: %w", err)
	}

	return redeemCode, nil
}

func subscriptionRedeemGroupIDs(code *RedeemCode) []int64 {
	if code == nil {
		return nil
	}
	seen := make(map[int64]struct{}, len(code.GroupIDs)+1)
	out := make([]int64, 0, len(code.GroupIDs)+1)
	add := func(id int64) {
		if id <= 0 {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	for _, id := range code.GroupIDs {
		add(id)
	}
	if len(out) == 0 && code.GroupID != nil {
		add(*code.GroupID)
	}
	return out
}

type currentMonthlyRedeemBundle struct {
	plan     string
	groupIDs []int64
}

var currentMonthlyRedeemBundles = []currentMonthlyRedeemBundle{
	{plan: "plus", groupIDs: []int64{40, 41, 48}},
	{plan: "pro", groupIDs: []int64{42, 43, 49}},
	{plan: "max", groupIDs: []int64{44, 45, 50}},
}

// completeCurrentMonthlyCardGroupIDs upgrades any unambiguous current
// Plus/Pro/Max subset to the complete GPT+Claude+Grok bundle. This is used at
// creation and again at redemption so a customer never loses fulfillment
// because an older inventory record omitted a host.
func completeCurrentMonthlyCardGroupIDs(groupIDs []int64) ([]int64, bool, error) {
	normalized := make([]int64, 0, len(groupIDs))
	seen := make(map[int64]struct{}, len(groupIDs))
	for _, groupID := range groupIDs {
		if groupID <= 0 {
			continue
		}
		if _, ok := seen[groupID]; ok {
			continue
		}
		seen[groupID] = struct{}{}
		normalized = append(normalized, groupID)
	}

	matchedBundle := -1
	for index, bundle := range currentMonthlyRedeemBundles {
		for _, groupID := range normalized {
			if containsMonthlyRedeemGroupID(bundle.groupIDs, groupID) {
				if matchedBundle >= 0 && matchedBundle != index {
					return nil, true, fmt.Errorf("current monthly-card groups span multiple plans")
				}
				matchedBundle = index
			}
		}
	}
	if matchedBundle < 0 {
		return normalized, false, nil
	}
	bundle := currentMonthlyRedeemBundles[matchedBundle]
	for _, groupID := range normalized {
		if !containsMonthlyRedeemGroupID(bundle.groupIDs, groupID) {
			return nil, true, fmt.Errorf("current %s monthly card cannot include unrelated group %d", bundle.plan, groupID)
		}
	}
	return append([]int64(nil), bundle.groupIDs...), true, nil
}

func containsMonthlyRedeemGroupID(groupIDs []int64, target int64) bool {
	for _, groupID := range groupIDs {
		if groupID == target {
			return true
		}
	}
	return false
}

func (s *RedeemService) completeRedeemedMonthlyCardBundle(ctx context.Context, code *RedeemCode) (bool, error) {
	if code == nil || code.Type != RedeemTypeSubscription {
		return false, nil
	}
	groupIDs := subscriptionRedeemGroupIDs(code)
	completed, currentMonthly, err := completeCurrentMonthlyCardGroupIDs(groupIDs)
	if err != nil {
		return false, fmt.Errorf("complete current monthly-card bundle: %w", err)
	}
	if !currentMonthly || sameInt64Set(groupIDs, completed) {
		return false, nil
	}
	code.GroupIDs = completed
	primaryGroupID := completed[0]
	code.GroupID = &primaryGroupID
	if err := s.redeemRepo.Update(ctx, code); err != nil {
		return false, fmt.Errorf("persist completed monthly-card bundle: %w", err)
	}
	return true, nil
}

func subscriptionRedeemNotes(code string, sharedQuota bool) string {
	notes := fmt.Sprintf("通过兑换码 %s 兑换", code)
	if !sharedQuota {
		return notes
	}
	marker := SubscriptionRedeemSharedQuotaMarker(code)
	if marker == "" {
		return notes
	}
	return fmt.Sprintf("%s; %s%s", notes, SubscriptionSharedQuotaNoteKey, marker)
}

func (s *RedeemService) resetBalanceAlertNotifiedFlag(userID int64) {
	resetCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	user, err := s.userRepo.GetByID(resetCtx, userID)
	if err != nil {
		return
	}
	s.balanceAlertService.ResetNotifiedFlag(resetCtx, userID, user.Balance)
}

// invalidateRedeemCaches 失效兑换相关的缓存
func (s *RedeemService) invalidateRedeemCaches(ctx context.Context, userID int64, redeemCode *RedeemCode) {
	switch redeemCode.Type {
	case RedeemTypeBalance:
		if s.authCacheInvalidator != nil {
			s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
		}
		if s.billingCacheService == nil {
			return
		}
		go func() {
			cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = s.billingCacheService.InvalidateUserBalance(cacheCtx, userID)
		}()
	case RedeemTypeConcurrency:
		if s.authCacheInvalidator != nil {
			s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
		}
		if s.billingCacheService == nil {
			return
		}
	case RedeemTypeSubscription:
		if s.authCacheInvalidator != nil {
			s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
		}
		if s.billingCacheService == nil {
			return
		}
		if redeemCode.GroupID != nil {
			groupID := *redeemCode.GroupID
			go func() {
				cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = s.billingCacheService.InvalidateSubscription(cacheCtx, userID, groupID)
			}()
		}
	}
}

// GetByID 根据ID获取兑换码
func (s *RedeemService) GetByID(ctx context.Context, id int64) (*RedeemCode, error) {
	code, err := s.redeemRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get redeem code: %w", err)
	}
	return code, nil
}

// GetByCode 根据Code获取兑换码
func (s *RedeemService) GetByCode(ctx context.Context, code string) (*RedeemCode, error) {
	redeemCode, err := s.redeemRepo.GetByCode(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("get redeem code: %w", err)
	}
	return redeemCode, nil
}

// List 获取兑换码列表（管理员功能）
func (s *RedeemService) List(ctx context.Context, params pagination.PaginationParams) ([]RedeemCode, *pagination.PaginationResult, error) {
	codes, pagination, err := s.redeemRepo.List(ctx, params)
	if err != nil {
		return nil, nil, fmt.Errorf("list redeem codes: %w", err)
	}
	return codes, pagination, nil
}

// Delete 删除兑换码（管理员功能）
func (s *RedeemService) Delete(ctx context.Context, id int64) error {
	// 检查兑换码是否存在
	code, err := s.redeemRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get redeem code: %w", err)
	}

	// 不允许删除已使用的兑换码
	if code.IsUsed() {
		return infraerrors.Conflict("REDEEM_CODE_DELETE_USED", "cannot delete used redeem code")
	}

	if err := s.redeemRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete redeem code: %w", err)
	}

	return nil
}

// GetStats 获取兑换码统计信息
func (s *RedeemService) GetStats(ctx context.Context) (map[string]any, error) {
	// TODO: 实现统计逻辑
	// 统计未使用、已使用的兑换码数量
	// 统计总面值等

	stats := map[string]any{
		"total_codes":  0,
		"unused_codes": 0,
		"used_codes":   0,
		"total_value":  0.0,
	}

	return stats, nil
}

// GetUserHistory 获取用户的兑换历史
func (s *RedeemService) GetUserHistory(ctx context.Context, userID int64, limit int) ([]RedeemCode, error) {
	if s.accountChangeRepo != nil {
		records, err := s.accountChangeRepo.ListByUser(ctx, userID, limit, AccountChangeRecordListFilters{
			Reasons: []string{AccountChangeReasonRedeemCode, AccountChangeReasonAdminAdjustment},
		})
		if err != nil {
			return nil, fmt.Errorf("get user account change history: %w", err)
		}

		codes := make([]RedeemCode, 0, len(records))
		for i := range records {
			codes = append(codes, records[i].ToRedeemCode())
		}
		return codes, nil
	}

	codes, err := s.redeemRepo.ListByUser(ctx, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("get user redeem history: %w", err)
	}
	return codes, nil
}

func accountChangeAssetTypeFromRedeemType(redeemType string) string {
	switch redeemType {
	case RedeemTypeBalance:
		return AccountChangeAssetBalance
	case RedeemTypeConcurrency:
		return AccountChangeAssetConcurrency
	case RedeemTypeSubscription:
		return AccountChangeAssetSubscription
	default:
		return redeemType
	}
}

func redeemCodeDeltaForAccountChange(code *RedeemCode) float64 {
	if code == nil {
		return 0
	}
	if code.Type == RedeemTypeSubscription && code.ValidityDays > 0 {
		return float64(code.ValidityDays)
	}
	return code.Value
}

func ptrString(v string) *string {
	return &v
}
