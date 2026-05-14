package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"
)

// BalanceAlertUserReader 余额预警服务需要的用户信息读取接口
type BalanceAlertUserReader interface {
	GetByID(ctx context.Context, id int64) (*User, error)
}

// BalanceAlertService 余额预警服务
type BalanceAlertService struct {
	alertCache  BalanceAlertCache
	attrDefRepo UserAttributeDefinitionRepository
	attrValRepo UserAttributeValueRepository
	settingRepo SettingRepository
	emailQueue  *EmailQueueService
	userReader  BalanceAlertUserReader
}

// NewBalanceAlertService creates a new BalanceAlertService.
func NewBalanceAlertService(
	alertCache BalanceAlertCache,
	attrDefRepo UserAttributeDefinitionRepository,
	attrValRepo UserAttributeValueRepository,
	settingRepo SettingRepository,
	emailQueue *EmailQueueService,
	userReader BalanceAlertUserReader,
) *BalanceAlertService {
	return &BalanceAlertService{
		alertCache:  alertCache,
		attrDefRepo: attrDefRepo,
		attrValRepo: attrValRepo,
		settingRepo: settingRepo,
		emailQueue:  emailQueue,
		userReader:  userReader,
	}
}

const (
	attrKeyBalanceAlertEnabled   = "balance_alert_enabled"
	attrKeyBalanceAlertThreshold = "balance_alert_threshold"
	attrKeyBalanceAlertEmail     = "balance_alert_email"
)

// CheckAndAlert 扣费后异步调用, 检查余额并发送预警邮件.
func (s *BalanceAlertService) CheckAndAlert(_ context.Context, userID int64, balanceAfterDeduct float64) {
	alertCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := s.checkAndAlertInternal(alertCtx, userID, balanceAfterDeduct); err != nil {
		slog.Error("balance alert check failed", "user_id", userID, "error", err)
	}
}

func (s *BalanceAlertService) checkAndAlertInternal(ctx context.Context, userID int64, balanceAfter float64) error {
	globalEnabled, err := s.getGlobalEnabled(ctx)
	if err != nil {
		return fmt.Errorf("get global enabled: %w", err)
	}
	if !globalEnabled {
		return nil
	}

	cfg, err := s.getUserConfig(ctx, userID)
	if err != nil {
		return fmt.Errorf("get user config: %w", err)
	}
	if !cfg.Enabled {
		return nil
	}

	threshold, err := s.getEffectiveThreshold(ctx, cfg.Threshold)
	if err != nil {
		return fmt.Errorf("get effective threshold: %w", err)
	}
	if balanceAfter >= threshold {
		return nil
	}

	notified, err := s.alertCache.IsNotified(ctx, userID)
	if err != nil {
		return fmt.Errorf("check notified: %w", err)
	}
	if notified {
		return nil
	}

	locked, err := s.alertCache.AcquireDispatchLock(ctx, userID)
	if err != nil {
		return fmt.Errorf("acquire dispatch lock: %w", err)
	}
	if !locked {
		return nil
	}

	notified, err = s.alertCache.IsNotified(ctx, userID)
	if err != nil {
		_ = s.alertCache.ClearDispatchLock(ctx, userID)
		return fmt.Errorf("recheck notified: %w", err)
	}
	if notified {
		_ = s.alertCache.ClearDispatchLock(ctx, userID)
		return nil
	}

	email, err := s.getEffectiveEmail(ctx, userID, cfg.Email)
	if err != nil {
		_ = s.alertCache.ClearDispatchLock(ctx, userID)
		return fmt.Errorf("get effective email: %w", err)
	}

	siteName := s.getSiteName(ctx)
	topUpURL := s.getTopUpURL(ctx)
	username := s.getUsername(ctx, userID)
	if s.emailQueue == nil {
		_ = s.alertCache.ClearDispatchLock(ctx, userID)
		return fmt.Errorf("balance alert email queue unavailable")
	}

	if err := s.emailQueue.EnqueueBalanceAlert(
		userID,
		email,
		siteName,
		username,
		strconv.FormatFloat(balanceAfter, 'f', 2, 64),
		strconv.FormatFloat(threshold, 'f', 2, 64),
		topUpURL,
	); err != nil {
		_ = s.alertCache.ClearDispatchLock(ctx, userID)
		return fmt.Errorf("enqueue balance alert: %w", err)
	}

	return nil
}

// ResetNotifiedFlag 充值后调用, 余额恢复时清除已通知标记.
func (s *BalanceAlertService) ResetNotifiedFlag(ctx context.Context, userID int64, newBalance float64) {
	cfg, err := s.getUserConfig(ctx, userID)
	if err != nil {
		slog.Error("balance alert: get config for reset", "user_id", userID, "error", err)
		return
	}
	threshold, err := s.getEffectiveThreshold(ctx, cfg.Threshold)
	if err != nil {
		slog.Error("balance alert: get threshold for reset", "user_id", userID, "error", err)
		return
	}

	if newBalance >= threshold {
		if err := s.alertCache.ClearNotified(ctx, userID); err != nil {
			slog.Error("balance alert: clear notified", "user_id", userID, "error", err)
		}
		if err := s.alertCache.ClearDispatchLock(ctx, userID); err != nil {
			slog.Error("balance alert: clear dispatch lock", "user_id", userID, "error", err)
		}
	}
}

// BalanceAlertResponse 用户端 GET 响应
type BalanceAlertResponse struct {
	Enabled            bool    `json:"enabled"`
	Threshold          *string `json:"threshold"`           // nil = 使用全局默认
	Email              string  `json:"email"`               // "" = 使用登录邮箱
	EffectiveThreshold string  `json:"effective_threshold"` // 实际生效阈值
	EffectiveEmail     string  `json:"effective_email"`     // 实际生效邮箱
}

// GetUserAlertConfig 获取当前用户的余额预警配置 (用户端 GET)
func (s *BalanceAlertService) GetUserAlertConfig(ctx context.Context, userID int64) (*BalanceAlertResponse, error) {
	cfg, err := s.getUserConfig(ctx, userID)
	if err != nil {
		return nil, err
	}

	effectiveThreshold, err := s.getEffectiveThreshold(ctx, cfg.Threshold)
	if err != nil {
		return nil, err
	}
	effectiveEmail, err := s.getEffectiveEmail(ctx, userID, cfg.Email)
	if err != nil {
		return nil, err
	}

	resp := &BalanceAlertResponse{
		Enabled:            cfg.Enabled,
		Email:              cfg.Email,
		EffectiveThreshold: strconv.FormatFloat(effectiveThreshold, 'f', 2, 64),
		EffectiveEmail:     effectiveEmail,
	}
	if cfg.Threshold > 0 {
		thStr := strconv.FormatFloat(cfg.Threshold, 'f', 2, 64)
		resp.Threshold = &thStr
	}

	return resp, nil
}

// UpdateUserAlertConfigInput 用户端 PUT 请求
type UpdateUserAlertConfigInput struct {
	Enabled        *bool    `json:"enabled"`
	Threshold      *float64 `json:"threshold"`       // nil = 不修改
	ClearThreshold bool     `json:"clear_threshold"` // true = 清除自定义阈值
	Email          *string  `json:"email"`           // nil = 不修改, "" = 清除自定义
}

// UpdateUserAlertConfig 更新用户余额预警配置 (用户端 PUT)
func (s *BalanceAlertService) UpdateUserAlertConfig(ctx context.Context, userID int64, input UpdateUserAlertConfigInput) error {
	defs, err := s.attrDefRepo.List(ctx, true)
	if err != nil {
		return fmt.Errorf("list attribute definitions: %w", err)
	}
	defIDMap := make(map[string]int64, len(defs))
	for _, d := range defs {
		defIDMap[d.Key] = d.ID
	}

	var updates []UpdateUserAttributeInput

	if input.Enabled != nil {
		if id, ok := defIDMap[attrKeyBalanceAlertEnabled]; ok {
			val := "false"
			if *input.Enabled {
				val = "true"
			}
			updates = append(updates, UpdateUserAttributeInput{AttributeID: id, Value: val})
		}
	}

	if input.ClearThreshold {
		if id, ok := defIDMap[attrKeyBalanceAlertThreshold]; ok {
			updates = append(updates, UpdateUserAttributeInput{AttributeID: id, Value: ""})
		}
	} else if input.Threshold != nil {
		if id, ok := defIDMap[attrKeyBalanceAlertThreshold]; ok {
			val := ""
			if *input.Threshold >= 0.10 {
				val = strconv.FormatFloat(*input.Threshold, 'f', 2, 64)
			}
			updates = append(updates, UpdateUserAttributeInput{AttributeID: id, Value: val})
		}
	}

	if input.Email != nil {
		if id, ok := defIDMap[attrKeyBalanceAlertEmail]; ok {
			updates = append(updates, UpdateUserAttributeInput{AttributeID: id, Value: *input.Email})
		}
	}

	if len(updates) > 0 {
		if err := s.attrValRepo.UpsertBatch(ctx, userID, updates); err != nil {
			return fmt.Errorf("update user alert config: %w", err)
		}
		_ = s.alertCache.ClearCachedConfig(ctx, userID)
		_ = s.alertCache.ClearDispatchLock(ctx, userID)
	}

	return nil
}

// ClearGlobalCache 管理员更新设置后调用.
func (s *BalanceAlertService) ClearGlobalCache(ctx context.Context) {
	if err := s.alertCache.ClearGlobalCache(ctx); err != nil {
		slog.Error("balance alert: clear global cache", "error", err)
	}
}

func (s *BalanceAlertService) getGlobalEnabled(ctx context.Context) (bool, error) {
	cached, err := s.alertCache.GetGlobalEnabled(ctx)
	if err == nil && cached != nil {
		return *cached, nil
	}

	val, err := s.settingRepo.GetValue(ctx, SettingKeyBalanceAlertEnabled)
	if err != nil {
		enabled := false
		if errors.Is(err, ErrSettingNotFound) {
			enabled = true
		}
		_ = s.alertCache.SetGlobalEnabled(ctx, enabled)
		return enabled, nil
	}

	enabled := val == "true"
	_ = s.alertCache.SetGlobalEnabled(ctx, enabled)
	return enabled, nil
}

func (s *BalanceAlertService) getGlobalThreshold(ctx context.Context) (float64, error) {
	cached, err := s.alertCache.GetGlobalThreshold(ctx)
	if err == nil && cached != nil {
		return *cached, nil
	}

	val, err := s.settingRepo.GetValue(ctx, SettingKeyBalanceAlertDefaultThreshold)
	if err != nil {
		defaultThreshold := 5.00
		_ = s.alertCache.SetGlobalThreshold(ctx, defaultThreshold)
		return defaultThreshold, nil
	}

	threshold, err := strconv.ParseFloat(val, 64)
	if err != nil {
		threshold = 5.00
	}
	_ = s.alertCache.SetGlobalThreshold(ctx, threshold)
	return threshold, nil
}

func (s *BalanceAlertService) getUserConfig(ctx context.Context, userID int64) (*BalanceAlertConfig, error) {
	cached, err := s.alertCache.GetCachedConfig(ctx, userID)
	if err == nil && cached != nil {
		return cached, nil
	}

	cfg := &BalanceAlertConfig{
		Enabled:   true,
		Threshold: 0,
		Email:     "",
	}
	attrs, err := s.attrValRepo.GetByUserID(ctx, userID)
	if err != nil {
		return cfg, nil
	}
	defs, err := s.attrDefRepo.List(ctx, true)
	if err != nil {
		return cfg, nil
	}

	defKeyMap := make(map[int64]string, len(defs))
	for _, d := range defs {
		defKeyMap[d.ID] = d.Key
	}

	for _, attr := range attrs {
		switch defKeyMap[attr.AttributeID] {
		case attrKeyBalanceAlertEnabled:
			cfg.Enabled = attr.Value != "false"
		case attrKeyBalanceAlertThreshold:
			if v, parseErr := strconv.ParseFloat(attr.Value, 64); parseErr == nil && v > 0 {
				cfg.Threshold = v
			}
		case attrKeyBalanceAlertEmail:
			cfg.Email = attr.Value
		}
	}

	_ = s.alertCache.SetCachedConfig(ctx, userID, cfg)
	return cfg, nil
}

func (s *BalanceAlertService) getEffectiveThreshold(ctx context.Context, userThreshold float64) (float64, error) {
	if userThreshold > 0 {
		return userThreshold, nil
	}
	return s.getGlobalThreshold(ctx)
}

func (s *BalanceAlertService) getEffectiveEmail(ctx context.Context, userID int64, customEmail string) (string, error) {
	if customEmail != "" {
		return customEmail, nil
	}
	user, err := s.userReader.GetByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("get user email: %w", err)
	}
	return user.Email, nil
}

func (s *BalanceAlertService) getSiteName(ctx context.Context) string {
	val, err := s.settingRepo.GetValue(ctx, SettingKeySiteName)
	if err == nil && val != "" {
		return val
	}
	return "Sub2API"
}

func (s *BalanceAlertService) getTopUpURL(ctx context.Context) string {
	val, err := s.settingRepo.GetValue(ctx, SettingKeyFrontendURL)
	if err == nil && val != "" {
		return val + "/redeem"
	}
	return ""
}

func (s *BalanceAlertService) getUsername(ctx context.Context, userID int64) string {
	user, err := s.userReader.GetByID(ctx, userID)
	if err != nil {
		return ""
	}
	if user.Username != "" {
		return user.Username
	}
	return user.Email
}
