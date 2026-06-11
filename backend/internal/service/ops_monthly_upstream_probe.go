package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
)

const (
	monthlyUpstreamProbeInterval = time.Minute
	monthlyUpstreamProbeTimeout  = 25 * time.Second
)

var monthlyUpstreamProbeTargets = []struct {
	AccountName string
	Platform    string
	Model       string
}{
	{AccountName: "pomoai-monthly-codex-0.12", Platform: PlatformOpenAI, Model: "gpt-5.4-mini-openai-compact"},
	{AccountName: "pomoai-monthly-claude-0.4", Platform: PlatformAnthropic, Model: "claude-haiku-4-5"},
}

type MonthlyUpstreamProbePoint struct {
	AccountID    int64     `json:"account_id"`
	AccountName  string    `json:"account_name"`
	Platform     string    `json:"platform"`
	Model        string    `json:"model"`
	Status       string    `json:"status"`
	HTTPStatus   *int      `json:"http_status"`
	LatencyMs    int64     `json:"latency_ms"`
	ErrorCode    string    `json:"error_code"`
	ErrorMessage string    `json:"error_message"`
	CheckedAt    time.Time `json:"checked_at"`
}

type MonthlyUpstreamProbeAccount struct {
	AccountID        int64                       `json:"account_id"`
	AccountName      string                      `json:"account_name"`
	Platform         string                      `json:"platform"`
	Model            string                      `json:"model"`
	LatestStatus     string                      `json:"latest_status"`
	LatestHTTPStatus *int                        `json:"latest_http_status"`
	LatestLatencyMs  int64                       `json:"latest_latency_ms"`
	LatestErrorCode  string                      `json:"latest_error_code"`
	LatestError      string                      `json:"latest_error"`
	LatestCheckedAt  *time.Time                  `json:"latest_checked_at"`
	Uptime           float64                     `json:"uptime"`
	SuccessCount     int                         `json:"success_count"`
	TotalCount       int                         `json:"total_count"`
	Points           []MonthlyUpstreamProbePoint `json:"points"`
}

type MonthlyUpstreamProbeSnapshot struct {
	Enabled       bool                          `json:"enabled"`
	WindowMinutes int                           `json:"window_minutes"`
	GeneratedAt   time.Time                     `json:"generated_at"`
	Accounts      []MonthlyUpstreamProbeAccount `json:"accounts"`
}

type MonthlyUpstreamProbeSettings struct {
	Enabled bool `json:"enabled"`
}

func (s *OpsService) startMonthlyUpstreamProbeRunner() {
	if s == nil || s.opsRepo == nil || s.accountRepo == nil || s.settingRepo == nil {
		return
	}
	go func() {
		timer := time.NewTimer(15 * time.Second)
		defer timer.Stop()
		for {
			<-timer.C
			ctx, cancel := context.WithTimeout(context.Background(), 2*monthlyUpstreamProbeTimeout)
			if s.IsMonthlyUpstreamProbeEnabled(ctx) {
				_ = s.RunMonthlyUpstreamProbeOnce(ctx)
			}
			cancel()
			timer.Reset(monthlyUpstreamProbeInterval)
		}
	}()
}

func (s *OpsService) IsMonthlyUpstreamProbeEnabled(ctx context.Context) bool {
	if s == nil || s.settingRepo == nil {
		return false
	}
	value, err := s.settingRepo.GetValue(ctx, SettingKeyMonthlyUpstreamProbeEnabled)
	if err != nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "on", "enabled":
		return true
	default:
		return false
	}
}

func (s *OpsService) UpdateMonthlyUpstreamProbeSettings(ctx context.Context, req *MonthlyUpstreamProbeSettings) (*MonthlyUpstreamProbeSettings, error) {
	if s == nil || s.settingRepo == nil {
		return nil, fmt.Errorf("settings repository is not available")
	}
	if req == nil {
		req = &MonthlyUpstreamProbeSettings{}
	}
	value := "false"
	if req.Enabled {
		value = "true"
	}
	if err := s.settingRepo.Set(ctx, SettingKeyMonthlyUpstreamProbeEnabled, value); err != nil {
		return nil, err
	}
	return &MonthlyUpstreamProbeSettings{Enabled: req.Enabled}, nil
}

func (s *OpsService) GetMonthlyUpstreamProbeSnapshot(ctx context.Context, windowMinutes int) (*MonthlyUpstreamProbeSnapshot, error) {
	if windowMinutes <= 0 {
		windowMinutes = 60
	}
	if windowMinutes > 24*60 {
		windowMinutes = 24 * 60
	}
	if s == nil || s.opsRepo == nil {
		return nil, fmt.Errorf("ops repository is not available")
	}

	now := time.Now()
	points, err := s.opsRepo.ListMonthlyUpstreamProbeResults(ctx, now.Add(-time.Duration(windowMinutes)*time.Minute))
	if err != nil {
		return nil, err
	}

	byAccount := make(map[string]*MonthlyUpstreamProbeAccount)
	for _, point := range points {
		key := point.AccountName
		if key == "" {
			key = point.Platform + ":" + point.Model
		}
		item := byAccount[key]
		if item == nil {
			item = &MonthlyUpstreamProbeAccount{
				AccountID:   point.AccountID,
				AccountName: point.AccountName,
				Platform:    point.Platform,
				Model:       point.Model,
				Points:      make([]MonthlyUpstreamProbePoint, 0, windowMinutes),
			}
			byAccount[key] = item
		}
		item.Points = append(item.Points, point)
		item.TotalCount++
		if isMonthlyUpstreamProbeHealthy(point.Status) {
			item.SuccessCount++
		}
		if item.LatestCheckedAt == nil || point.CheckedAt.After(*item.LatestCheckedAt) {
			checkedAt := point.CheckedAt
			item.LatestCheckedAt = &checkedAt
			item.LatestStatus = point.Status
			item.LatestHTTPStatus = point.HTTPStatus
			item.LatestLatencyMs = point.LatencyMs
			item.LatestErrorCode = point.ErrorCode
			item.LatestError = point.ErrorMessage
			item.Model = point.Model
		}
	}

	accounts := make([]MonthlyUpstreamProbeAccount, 0, len(byAccount))
	for _, item := range byAccount {
		sort.Slice(item.Points, func(i, j int) bool {
			return item.Points[i].CheckedAt.Before(item.Points[j].CheckedAt)
		})
		if item.TotalCount > 0 {
			item.Uptime = float64(item.SuccessCount) / float64(item.TotalCount)
		}
		accounts = append(accounts, *item)
	}
	sort.Slice(accounts, func(i, j int) bool {
		if accounts[i].Platform == accounts[j].Platform {
			return accounts[i].AccountName < accounts[j].AccountName
		}
		return accounts[i].Platform < accounts[j].Platform
	})

	return &MonthlyUpstreamProbeSnapshot{
		Enabled:       s.IsMonthlyUpstreamProbeEnabled(ctx),
		WindowMinutes: windowMinutes,
		GeneratedAt:   now,
		Accounts:      accounts,
	}, nil
}

func (s *OpsService) RunMonthlyUpstreamProbeOnce(ctx context.Context) error {
	if s == nil || s.accountRepo == nil || s.opsRepo == nil {
		return fmt.Errorf("monthly upstream probe dependencies are not available")
	}

	accounts, err := s.loadMonthlyUpstreamProbeAccounts(ctx)
	if err != nil {
		return err
	}
	var errs []error
	for _, target := range monthlyUpstreamProbeTargets {
		account := accounts[target.AccountName]
		var point MonthlyUpstreamProbePoint
		if account == nil {
			point = MonthlyUpstreamProbePoint{
				AccountName:  target.AccountName,
				Platform:     target.Platform,
				Model:        target.Model,
				Status:       "failed",
				ErrorCode:    "missing_account",
				ErrorMessage: "monthly upstream account was not found",
				CheckedAt:    time.Now(),
			}
		} else {
			point = probeMonthlyUpstreamAccount(ctx, account, target.Model)
		}
		if err := s.opsRepo.InsertMonthlyUpstreamProbeResult(ctx, &point); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *OpsService) loadMonthlyUpstreamProbeAccounts(ctx context.Context) (map[string]*Account, error) {
	accounts, _, err := s.accountRepo.ListWithFilters(ctx, pagination.PaginationParams{Page: 1, PageSize: 100}, "", "", "", "pomoai-monthly", 0)
	if err != nil {
		return nil, err
	}
	out := make(map[string]*Account)
	for i := range accounts {
		account := accounts[i]
		for _, target := range monthlyUpstreamProbeTargets {
			if account.Name == target.AccountName {
				copy := account
				out[account.Name] = &copy
			}
		}
	}
	return out, nil
}

func probeMonthlyUpstreamAccount(ctx context.Context, account *Account, model string) MonthlyUpstreamProbePoint {
	if account == nil {
		return MonthlyUpstreamProbePoint{Status: "failed", ErrorCode: "nil_account", CheckedAt: time.Now()}
	}
	if !account.IsSchedulable() {
		return MonthlyUpstreamProbePoint{
			AccountID:    account.ID,
			AccountName:  account.Name,
			Platform:     account.Platform,
			Model:        model,
			Status:       "not_schedulable",
			ErrorMessage: fmt.Sprintf("account status=%s schedulable=%t", account.Status, account.Schedulable),
			CheckedAt:    time.Now(),
		}
	}

	switch account.Platform {
	case PlatformOpenAI:
		return probeMonthlyOpenAIUpstream(ctx, account, model)
	case PlatformAnthropic:
		return probeMonthlyAnthropicUpstream(ctx, account, model)
	default:
		return MonthlyUpstreamProbePoint{
			AccountID:    account.ID,
			AccountName:  account.Name,
			Platform:     account.Platform,
			Model:        model,
			Status:       "failed",
			ErrorCode:    "unsupported_platform",
			ErrorMessage: "unsupported platform for monthly upstream probe",
			CheckedAt:    time.Now(),
		}
	}
}

func probeMonthlyOpenAIUpstream(ctx context.Context, account *Account, model string) MonthlyUpstreamProbePoint {
	apiKey := strings.TrimSpace(account.GetOpenAIApiKey())
	if apiKey == "" {
		return monthlyProbeLocalFailure(account, model, "missing_api_key", "missing upstream api key")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(account.GetOpenAIBaseURL()), "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	url := buildOpenAIResponsesURL(baseURL)
	if strings.HasSuffix(model, "-openai-compact") {
		url = appendOpenAIResponsesRequestPathSuffix(url, "/compact")
	}
	body, _ := json.Marshal(createOpenAICompactProbePayload(model))
	return executeMonthlyProbeHTTP(ctx, account, model, url, map[string]string{
		"Authorization":   "Bearer " + apiKey,
		"Content-Type":    "application/json",
		"Accept":          "application/json",
		"OpenAI-Beta":     "responses=experimental",
		"Originator":      "codex_cli_rs",
		"User-Agent":      "codex_cli_rs/0.0.0 monthly-upstream-probe",
		"Version":         "0.0.0",
		"Session_ID":      fmt.Sprintf("monthly_probe_%d", account.ID),
		"Conversation_ID": fmt.Sprintf("monthly_probe_%d", account.ID),
	}, body)
}

func probeMonthlyAnthropicUpstream(ctx context.Context, account *Account, model string) MonthlyUpstreamProbePoint {
	apiKey := strings.TrimSpace(account.GetCredential("api_key"))
	if apiKey == "" {
		return monthlyProbeLocalFailure(account, model, "missing_api_key", "missing upstream api key")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(account.GetBaseURL()), "/")
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	body, _ := json.Marshal(map[string]any{
		"model":      model,
		"max_tokens": 1,
		"messages":   []map[string]string{{"role": "user", "content": "OK"}},
		"metadata":   map[string]string{"user_id": fmt.Sprintf("monthly-probe-%d", account.ID)},
		"stream":     false,
	})
	return executeMonthlyProbeHTTP(ctx, account, model, buildSupplierAnthropicMessagesURL(baseURL), map[string]string{
		"Authorization":     "Bearer " + apiKey,
		"Content-Type":      "application/json",
		"Accept":            "application/json",
		"User-Agent":        "claude-cli/2.1.84 (external, cli)",
		"Anthropic-Version": "2023-06-01",
		"Anthropic-Beta":    "oauth-2025-04-20,interleaved-thinking-2025-05-14",
		"Anthropic-Dangerous-Direct-Browser-Access": "true",
		"X-App": "cli",
	}, body)
}

func executeMonthlyProbeHTTP(ctx context.Context, account *Account, model string, url string, headers map[string]string, body []byte) MonthlyUpstreamProbePoint {
	probeCtx, cancel := context.WithTimeout(ctx, monthlyUpstreamProbeTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return monthlyProbeLocalFailure(account, model, "build_request_failed", err.Error())
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	started := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return monthlyProbeLocalFailure(account, model, "request_failed", err.Error())
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return monthlyProbeLocalFailure(account, model, "read_response_failed", err.Error())
	}

	httpStatus := resp.StatusCode
	point := MonthlyUpstreamProbePoint{
		AccountID:   account.ID,
		AccountName: account.Name,
		Platform:    account.Platform,
		Model:       model,
		HTTPStatus:  &httpStatus,
		LatencyMs:   time.Since(started).Milliseconds(),
		CheckedAt:   time.Now(),
	}
	point.ErrorCode, point.ErrorMessage = extractMonthlyProbeError(raw)
	switch {
	case httpStatus >= 200 && httpStatus < 300:
		if point.LatencyMs > 5000 {
			point.Status = "slow"
		} else {
			point.Status = "ok"
		}
	case httpStatus == http.StatusTooManyRequests:
		point.Status = "rate_limited"
	default:
		point.Status = "failed"
	}
	point.ErrorMessage = sanitizeMonthlyProbeError(point.ErrorMessage, account)
	return point
}

func monthlyProbeLocalFailure(account *Account, model, code, message string) MonthlyUpstreamProbePoint {
	point := MonthlyUpstreamProbePoint{
		Model:        model,
		Status:       "failed",
		ErrorCode:    code,
		ErrorMessage: message,
		CheckedAt:    time.Now(),
	}
	if account != nil {
		point.AccountID = account.ID
		point.AccountName = account.Name
		point.Platform = account.Platform
		point.ErrorMessage = sanitizeMonthlyProbeError(point.ErrorMessage, account)
	}
	return point
}

func extractMonthlyProbeError(raw []byte) (string, string) {
	var parsed struct {
		Error any `json:"error"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", strings.TrimSpace(string(raw))
	}
	if obj, ok := parsed.Error.(map[string]any); ok {
		code, _ := obj["code"].(string)
		if code == "" {
			code, _ = obj["type"].(string)
		}
		message, _ := obj["message"].(string)
		return code, message
	}
	return "", ""
}

func sanitizeMonthlyProbeError(message string, account *Account) string {
	message = strings.TrimSpace(strings.ReplaceAll(message, "\n", " "))
	if account != nil {
		for _, key := range []string{"api_key", "access_token", "refresh_token"} {
			if value := account.GetCredential(key); value != "" {
				message = strings.ReplaceAll(message, value, "[redacted]")
			}
		}
	}
	if len(message) > 1000 {
		return message[:1000]
	}
	return message
}

func isMonthlyUpstreamProbeHealthy(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "ok", "slow", "rate_limited":
		return true
	default:
		return false
	}
}
