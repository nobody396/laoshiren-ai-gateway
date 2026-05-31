package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
)

const (
	SupplierStatusEvaluating = "evaluating"
	SupplierStatusActive     = "active"

	SupplierProbeStatusUnknown  = "unknown"
	SupplierProbeStatusSuccess  = "success"
	SupplierProbeStatusDegraded = "degraded"
	SupplierProbeStatusFailed   = "failed"

	DefaultSupplierProbeModel           = "claude-haiku-4-5-20251001"
	DefaultSupplierProbeIntervalMinutes = 30

	SupplierProbeSubStatusNone            = ""
	SupplierProbeSubStatusSlowLatency     = "slow_latency"
	SupplierProbeSubStatusRateLimit       = "rate_limit"
	SupplierProbeSubStatusServerError     = "server_error"
	SupplierProbeSubStatusClientError     = "client_error"
	SupplierProbeSubStatusAuthError       = "auth_error"
	SupplierProbeSubStatusInvalidRequest  = "invalid_request"
	SupplierProbeSubStatusNetworkError    = "network_error"
	SupplierProbeSubStatusResponseTimeout = "response_timeout"
	SupplierProbeSubStatusContentMismatch = "content_mismatch"

	supplierProbeTimeout       = 30 * time.Second
	supplierProbeSlowLatency   = 5 * time.Second
	supplierProbeMaxAttempts   = 3
	supplierProbeRetryBaseWait = 200 * time.Millisecond
)

var (
	ErrSupplierNotFound = infraerrors.NotFound("SUPPLIER_NOT_FOUND", "supplier not found")
	ErrSupplierExists   = infraerrors.Conflict("SUPPLIER_EXISTS", "supplier name already exists")
)

// Supplier represents an upstream supplier under evaluation.
type Supplier struct {
	ID                   int64      `json:"id"`
	Name                 string     `json:"name"`
	WebsiteURL           string     `json:"website_url"`
	BaseURL              string     `json:"base_url"`
	APIKey               string     `json:"api_key"`
	UpstreamGroup        string     `json:"upstream_group"`
	ContactPlatform      string     `json:"contact_platform"`
	ContactValue         string     `json:"contact_value"`
	Status               string     `json:"status"`
	CostRMBPerUSD        *float64   `json:"cost_rmb_per_usd"`
	Notes                string     `json:"notes"`
	ProbeEnabled         bool       `json:"probe_enabled"`
	ProbeModel           string     `json:"probe_model"`
	ProbeIntervalMinutes int        `json:"probe_interval_minutes"`
	LastProbeStatus      string     `json:"last_probe_status"`
	LastProbeSubStatus   string     `json:"last_probe_sub_status"`
	LastProbeHTTPCode    *int       `json:"last_probe_http_code"`
	LastProbeLatencyMs   *int64     `json:"last_probe_latency_ms"`
	LastProbeError       string     `json:"last_probe_error"`
	LastProbeAt          *time.Time `json:"last_probe_at"`
	NextProbeAt          *time.Time `json:"next_probe_at"`
	ProbeSuccessRate     float64    `json:"probe_success_rate"`
	ProbeSuccessCount    int        `json:"probe_success_count"`
	ProbeTotalCount      int        `json:"probe_total_count"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`

	TargetGroupIDs []int64 `json:"target_group_ids"`

	TargetGroupCount int `json:"target_group_count"`
}

type SupplierProbeResult struct {
	ID           int64     `json:"id"`
	SupplierID   int64     `json:"supplier_id"`
	Status       string    `json:"status"`
	SubStatus    string    `json:"sub_status"`
	HTTPCode     int       `json:"http_code"`
	Model        string    `json:"model"`
	LatencyMs    int64     `json:"latency_ms"`
	AccuracyOK   bool      `json:"accuracy_ok"`
	ResponseText string    `json:"response_text"`
	ErrorMessage string    `json:"error_message"`
	CheckedAt    time.Time `json:"checked_at"`
	CreatedAt    time.Time `json:"created_at"`
}

type SupplierListFilter struct {
	Status      string
	ProbeStatus string
	Search      string
}

type CreateSupplierInput struct {
	Name                 string
	WebsiteURL           string
	BaseURL              string
	APIKey               string
	UpstreamGroup        string
	ContactPlatform      string
	ContactValue         string
	Status               string
	CostRMBPerUSD        *float64
	Notes                string
	ProbeEnabled         bool
	ProbeModel           string
	ProbeIntervalMinutes int
	TargetGroupIDs       []int64
}

type UpdateSupplierInput struct {
	Name                 *string
	WebsiteURL           *string
	BaseURL              *string
	APIKey               *string
	UpstreamGroup        *string
	ContactPlatform      *string
	ContactValue         *string
	Status               *string
	CostRMBPerUSD        *float64
	Notes                *string
	ProbeEnabled         *bool
	ProbeModel           *string
	ProbeIntervalMinutes *int
	TargetGroupIDs       *[]int64
}

type SupplierRepository interface {
	Create(ctx context.Context, supplier *Supplier) error
	GetByID(ctx context.Context, id int64) (*Supplier, error)
	Update(ctx context.Context, supplier *Supplier, replaceGroups bool, replaceAccounts bool) error
	RecordProbeResult(ctx context.Context, supplierID int64, result *SupplierProbeResult) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, params pagination.PaginationParams, filter SupplierListFilter) ([]Supplier, *pagination.PaginationResult, error)
}

type SupplierService struct {
	repo SupplierRepository
}

func NewSupplierService(repo SupplierRepository) *SupplierService {
	return &SupplierService{repo: repo}
}

func (s *SupplierService) List(ctx context.Context, params pagination.PaginationParams, filter SupplierListFilter) ([]Supplier, *pagination.PaginationResult, error) {
	filter.Status = normalizeOptionalSupplierStatus(filter.Status)
	filter.ProbeStatus = normalizeOptionalSupplierProbeStatus(filter.ProbeStatus)
	filter.Search = strings.TrimSpace(filter.Search)
	if len(filter.Search) > 100 {
		filter.Search = filter.Search[:100]
	}
	return s.repo.List(ctx, params, filter)
}

func (s *SupplierService) GetByID(ctx context.Context, id int64) (*Supplier, error) {
	if id <= 0 {
		return nil, infraerrors.BadRequest("INVALID_SUPPLIER_ID", "invalid supplier id")
	}
	return s.repo.GetByID(ctx, id)
}

func (s *SupplierService) Create(ctx context.Context, input *CreateSupplierInput) (*Supplier, error) {
	if input == nil {
		return nil, infraerrors.BadRequest("INVALID_SUPPLIER", "supplier payload is required")
	}
	supplier := &Supplier{
		Name:                 strings.TrimSpace(input.Name),
		WebsiteURL:           strings.TrimSpace(input.WebsiteURL),
		BaseURL:              strings.TrimSpace(input.BaseURL),
		APIKey:               strings.TrimSpace(input.APIKey),
		UpstreamGroup:        strings.TrimSpace(input.UpstreamGroup),
		ContactPlatform:      normalizeSupplierContactPlatform(input.ContactPlatform),
		ContactValue:         strings.TrimSpace(input.ContactValue),
		Status:               normalizeSupplierStatus(input.Status),
		CostRMBPerUSD:        input.CostRMBPerUSD,
		Notes:                strings.TrimSpace(input.Notes),
		ProbeEnabled:         input.ProbeEnabled,
		ProbeModel:           normalizeSupplierProbeModel(input.ProbeModel),
		ProbeIntervalMinutes: normalizeSupplierProbeInterval(input.ProbeIntervalMinutes),
		LastProbeStatus:      SupplierProbeStatusUnknown,
		TargetGroupIDs:       normalizeInt64IDs(input.TargetGroupIDs),
	}
	if err := validateSupplier(supplier); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, supplier); err != nil {
		return nil, err
	}
	return supplier, nil
}

func (s *SupplierService) Update(ctx context.Context, id int64, input *UpdateSupplierInput) (*Supplier, error) {
	if id <= 0 {
		return nil, infraerrors.BadRequest("INVALID_SUPPLIER_ID", "invalid supplier id")
	}
	if input == nil {
		return nil, infraerrors.BadRequest("INVALID_SUPPLIER", "supplier payload is required")
	}

	supplier, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if input.Name != nil {
		supplier.Name = strings.TrimSpace(*input.Name)
	}
	if input.WebsiteURL != nil {
		supplier.WebsiteURL = strings.TrimSpace(*input.WebsiteURL)
	}
	if input.BaseURL != nil {
		supplier.BaseURL = strings.TrimSpace(*input.BaseURL)
	}
	if input.APIKey != nil {
		supplier.APIKey = strings.TrimSpace(*input.APIKey)
	}
	if input.UpstreamGroup != nil {
		supplier.UpstreamGroup = strings.TrimSpace(*input.UpstreamGroup)
	}
	if input.ContactPlatform != nil {
		supplier.ContactPlatform = normalizeSupplierContactPlatform(*input.ContactPlatform)
	}
	if input.ContactValue != nil {
		supplier.ContactValue = strings.TrimSpace(*input.ContactValue)
	}
	if input.Status != nil {
		supplier.Status = normalizeSupplierStatus(*input.Status)
	}
	if input.CostRMBPerUSD != nil {
		supplier.CostRMBPerUSD = input.CostRMBPerUSD
	}
	if input.Notes != nil {
		supplier.Notes = strings.TrimSpace(*input.Notes)
	}
	if input.ProbeEnabled != nil {
		supplier.ProbeEnabled = *input.ProbeEnabled
	}
	if input.ProbeModel != nil {
		supplier.ProbeModel = normalizeSupplierProbeModel(*input.ProbeModel)
	}
	if input.ProbeIntervalMinutes != nil {
		supplier.ProbeIntervalMinutes = normalizeSupplierProbeInterval(*input.ProbeIntervalMinutes)
	}

	replaceTargetGroups := input.TargetGroupIDs != nil
	if replaceTargetGroups {
		supplier.TargetGroupIDs = normalizeInt64IDs(*input.TargetGroupIDs)
	}

	if err := validateSupplier(supplier); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, supplier, replaceTargetGroups, false); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *SupplierService) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return infraerrors.BadRequest("INVALID_SUPPLIER_ID", "invalid supplier id")
	}
	return s.repo.Delete(ctx, id)
}

func (s *SupplierService) RunProbe(ctx context.Context, id int64) (*Supplier, *SupplierProbeResult, error) {
	if id <= 0 {
		return nil, nil, infraerrors.BadRequest("INVALID_SUPPLIER_ID", "invalid supplier id")
	}
	supplier, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}

	result := runSupplierProbe(ctx, supplier)
	if err := s.repo.RecordProbeResult(ctx, id, result); err != nil {
		return nil, nil, err
	}
	updated, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	return updated, result, nil
}

func validateSupplier(supplier *Supplier) error {
	if supplier == nil {
		return infraerrors.BadRequest("INVALID_SUPPLIER", "supplier is required")
	}
	if supplier.Name == "" {
		return infraerrors.BadRequest("SUPPLIER_NAME_REQUIRED", "supplier name is required")
	}
	if len(supplier.Name) > 100 {
		return infraerrors.BadRequest("SUPPLIER_NAME_TOO_LONG", "supplier name is too long")
	}
	if len(supplier.WebsiteURL) > 500 || len(supplier.BaseURL) > 500 {
		return infraerrors.BadRequest("SUPPLIER_URL_TOO_LONG", "supplier url is too long")
	}
	if len(supplier.APIKey) > 4000 {
		return infraerrors.BadRequest("SUPPLIER_API_KEY_TOO_LONG", "supplier api key is too long")
	}
	if len(supplier.UpstreamGroup) > 100 {
		return infraerrors.BadRequest("SUPPLIER_UPSTREAM_GROUP_TOO_LONG", "supplier upstream group is too long")
	}
	if !isValidSupplierContactPlatform(supplier.ContactPlatform) {
		return infraerrors.BadRequest("INVALID_SUPPLIER_CONTACT_PLATFORM", "invalid supplier contact platform")
	}
	if len(supplier.ContactValue) > 200 {
		return infraerrors.BadRequest("SUPPLIER_CONTACT_TOO_LONG", "supplier contact is too long")
	}
	if !isValidSupplierStatus(supplier.Status) {
		return infraerrors.BadRequest("INVALID_SUPPLIER_STATUS", "invalid supplier status")
	}
	if supplier.CostRMBPerUSD != nil && *supplier.CostRMBPerUSD < 0 {
		return infraerrors.BadRequest("INVALID_SUPPLIER_COST", "supplier cost amount must be non-negative")
	}
	if len(supplier.ProbeModel) > 100 {
		return infraerrors.BadRequest("SUPPLIER_PROBE_MODEL_TOO_LONG", "supplier probe model is too long")
	}
	if supplier.ProbeIntervalMinutes <= 0 || supplier.ProbeIntervalMinutes > 1440 {
		return infraerrors.BadRequest("INVALID_SUPPLIER_PROBE_INTERVAL", "supplier probe interval is invalid")
	}
	if supplier.LastProbeStatus != "" && !isValidSupplierProbeStatus(supplier.LastProbeStatus) {
		return infraerrors.BadRequest("INVALID_SUPPLIER_PROBE_STATUS", "invalid supplier probe status")
	}
	return nil
}

func normalizeSupplierStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		return SupplierStatusEvaluating
	}
	if status == "candidate" || status == "trial" {
		return SupplierStatusEvaluating
	}
	return status
}

func normalizeOptionalSupplierStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" || status == "all" {
		return ""
	}
	return status
}

func isValidSupplierStatus(status string) bool {
	switch status {
	case SupplierStatusEvaluating, SupplierStatusActive:
		return true
	default:
		return false
	}
}

func normalizeSupplierContactPlatform(platform string) string {
	return strings.ToLower(strings.TrimSpace(platform))
}

func isValidSupplierContactPlatform(platform string) bool {
	switch platform {
	case "", "wechat", "telegram", "qq", "email", "phone", "other":
		return true
	default:
		return false
	}
}

func normalizeSupplierProbeModel(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return DefaultSupplierProbeModel
	}
	return model
}

func normalizeSupplierProbeInterval(minutes int) int {
	if minutes <= 0 {
		return DefaultSupplierProbeIntervalMinutes
	}
	if minutes > 1440 {
		return 1440
	}
	return minutes
}

func normalizeOptionalSupplierProbeStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" || status == "all" {
		return ""
	}
	return status
}

func isValidSupplierProbeStatus(status string) bool {
	switch status {
	case SupplierProbeStatusUnknown, SupplierProbeStatusSuccess, SupplierProbeStatusDegraded, SupplierProbeStatusFailed:
		return true
	default:
		return false
	}
}

func runSupplierProbe(ctx context.Context, supplier *Supplier) *SupplierProbeResult {
	started := time.Now()
	result := &SupplierProbeResult{
		SupplierID: supplier.ID,
		Status:     SupplierProbeStatusFailed,
		SubStatus:  SupplierProbeSubStatusNone,
		Model:      normalizeSupplierProbeModel(supplier.ProbeModel),
		CheckedAt:  started,
	}
	defer func() {
		result.LatencyMs = time.Since(started).Milliseconds()
	}()

	if strings.TrimSpace(supplier.BaseURL) == "" {
		result.ErrorMessage = "base url is required"
		result.SubStatus = SupplierProbeSubStatusInvalidRequest
		return result
	}
	if strings.TrimSpace(supplier.APIKey) == "" {
		result.ErrorMessage = "api key is required"
		result.SubStatus = SupplierProbeSubStatusAuthError
		return result
	}

	probeCtx, cancel := context.WithTimeout(ctx, supplierProbeTimeout)
	defer cancel()

	for attempt := 0; attempt < supplierProbeMaxAttempts; attempt++ {
		resetSupplierProbeAttempt(result)
		if isSupplierClaudeProbeModel(result.Model) {
			runSupplierClaudeHaikuProbe(probeCtx, supplier, result)
		} else {
			runSupplierOpenAIProbe(probeCtx, supplier, result)
		}

		if result.Status != SupplierProbeStatusFailed || attempt+1 >= supplierProbeMaxAttempts {
			return result
		}

		delay := supplierProbeRetryDelay(attempt)
		select {
		case <-probeCtx.Done():
			if result.SubStatus == SupplierProbeSubStatusNone {
				result.SubStatus = SupplierProbeSubStatusResponseTimeout
			}
			if result.ErrorMessage == "" {
				result.ErrorMessage = probeCtx.Err().Error()
			}
			return result
		case <-time.After(delay):
		}
	}
	return result
}

func resetSupplierProbeAttempt(result *SupplierProbeResult) {
	result.Status = SupplierProbeStatusFailed
	result.SubStatus = SupplierProbeSubStatusNone
	result.HTTPCode = 0
	result.AccuracyOK = false
	result.ResponseText = ""
	result.ErrorMessage = ""
}

func supplierProbeRetryDelay(attempt int) time.Duration {
	delay := supplierProbeRetryBaseWait
	for i := 0; i < attempt; i++ {
		delay *= 2
	}
	if delay > 2*time.Second {
		return 2 * time.Second
	}
	return delay
}

func runSupplierOpenAIProbe(ctx context.Context, supplier *Supplier, result *SupplierProbeResult) {
	prompt, expected := newSupplierProbePrompt()
	body, err := json.Marshal(map[string]any{
		"model": result.Model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens":  24,
		"temperature": 0,
	})
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("build probe payload: %v", err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, buildOpenAIChatCompletionsURL(supplier.BaseURL), bytes.NewReader(body))
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("build probe request: %v", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(supplier.APIKey))
	req.Header.Set("X-Api-Key", strings.TrimSpace(supplier.APIKey))
	req.Header.Set("Content-Type", "application/json")

	started := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		setSupplierProbeTransportError(result, ctx, err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("read probe response: %v", err)
		result.SubStatus = SupplierProbeSubStatusResponseTimeout
		return
	}

	status, subStatus := classifySupplierProbeHTTPStatus(resp.StatusCode, time.Since(started))
	result.HTTPCode = resp.StatusCode
	result.Status = status
	result.SubStatus = subStatus

	if result.Status == SupplierProbeStatusFailed {
		result.ErrorMessage = supplierProbeHTTPError(resp.StatusCode, raw)
		return
	}

	content := extractOpenAIProbeContent(raw)
	applySupplierProbeContentCheck(result, content, expected)
}

func runSupplierClaudeHaikuProbe(ctx context.Context, supplier *Supplier, result *SupplierProbeResult) {
	body, err := json.Marshal(map[string]any{
		"model":      result.Model,
		"max_tokens": 1,
		"messages": []map[string]string{
			{"role": "user", "content": "quota"},
		},
		"metadata": map[string]string{
			"user_id": fmt.Sprintf("supplier-probe-%d", supplier.ID),
		},
	})
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("build probe payload: %v", err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, buildSupplierAnthropicMessagesURL(supplier.BaseURL), bytes.NewReader(body))
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("build probe request: %v", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(supplier.APIKey))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "claude-cli/2.1.84 (external, cli)")
	req.Header.Set("Anthropic-Version", "2023-06-01")
	req.Header.Set("Anthropic-Beta", "oauth-2025-04-20,interleaved-thinking-2025-05-14")
	req.Header.Set("Anthropic-Dangerous-Direct-Browser-Access", "true")
	req.Header.Set("X-App", "cli")

	started := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		setSupplierProbeTransportError(result, ctx, err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("read probe response: %v", err)
		result.SubStatus = SupplierProbeSubStatusResponseTimeout
		return
	}

	status, subStatus := classifySupplierProbeHTTPStatus(resp.StatusCode, time.Since(started))
	result.HTTPCode = resp.StatusCode
	result.Status = status
	result.SubStatus = subStatus

	if result.Status == SupplierProbeStatusFailed {
		result.ErrorMessage = supplierProbeHTTPError(resp.StatusCode, raw)
		return
	}

	content := extractAnthropicProbeContent(raw)
	applySupplierProbeContentCheck(result, content, "#")
}

func setSupplierProbeTransportError(result *SupplierProbeResult, ctx context.Context, err error) {
	result.Status = SupplierProbeStatusFailed
	if ctx.Err() != nil {
		result.SubStatus = SupplierProbeSubStatusResponseTimeout
		result.ErrorMessage = ctx.Err().Error()
		return
	}
	result.SubStatus = SupplierProbeSubStatusNetworkError
	result.ErrorMessage = err.Error()
}

func classifySupplierProbeHTTPStatus(statusCode int, latency time.Duration) (string, string) {
	switch {
	case statusCode >= 200 && statusCode < 300:
		if latency > supplierProbeSlowLatency {
			return SupplierProbeStatusDegraded, SupplierProbeSubStatusSlowLatency
		}
		return SupplierProbeStatusSuccess, SupplierProbeSubStatusNone
	case statusCode >= 300 && statusCode < 400:
		return SupplierProbeStatusSuccess, SupplierProbeSubStatusNone
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		return SupplierProbeStatusFailed, SupplierProbeSubStatusAuthError
	case statusCode == http.StatusBadRequest:
		return SupplierProbeStatusFailed, SupplierProbeSubStatusInvalidRequest
	case statusCode == http.StatusTooManyRequests:
		return SupplierProbeStatusDegraded, SupplierProbeSubStatusRateLimit
	case statusCode >= 500:
		return SupplierProbeStatusFailed, SupplierProbeSubStatusServerError
	case statusCode >= 400:
		return SupplierProbeStatusFailed, SupplierProbeSubStatusClientError
	default:
		return SupplierProbeStatusFailed, SupplierProbeSubStatusClientError
	}
}

func supplierProbeHTTPError(statusCode int, raw []byte) string {
	body := strings.TrimSpace(string(raw))
	if len(body) > 1200 {
		body = body[:1200]
	}
	if body == "" {
		return fmt.Sprintf("upstream returned HTTP %d", statusCode)
	}
	return fmt.Sprintf("upstream returned HTTP %d: %s", statusCode, body)
}

func applySupplierProbeContentCheck(result *SupplierProbeResult, content string, expected string) {
	content = strings.TrimSpace(content)
	result.ResponseText = content
	if result.Status == SupplierProbeStatusDegraded && result.SubStatus == SupplierProbeSubStatusRateLimit {
		return
	}
	if content == "" {
		result.Status = SupplierProbeStatusDegraded
		result.SubStatus = SupplierProbeSubStatusContentMismatch
		result.ErrorMessage = "probe response was empty"
		return
	}
	result.AccuracyOK = expected != "" && strings.Contains(content, expected)
	if expected != "" && !result.AccuracyOK {
		result.Status = SupplierProbeStatusDegraded
		result.SubStatus = SupplierProbeSubStatusContentMismatch
		result.ErrorMessage = "probe response did not contain expected marker"
	}
}

func isSupplierClaudeProbeModel(model string) bool {
	normalized := strings.ToLower(strings.TrimSpace(model))
	return strings.Contains(normalized, "claude")
}

func buildSupplierAnthropicMessagesURL(base string) string {
	normalized := strings.TrimRight(strings.TrimSpace(base), "/")
	if strings.Contains(normalized, "/v1/messages") {
		return normalized
	}
	return buildOpenAIEndpointURL(normalized, "/v1/messages") + "?beta=true"
}

func newSupplierProbePrompt() (prompt string, expected string) {
	seed := time.Now().UnixNano()
	a := int(seed%90) + 10
	b := int((seed/97)%90) + 10
	expected = fmt.Sprintf("LSR_PROBE=%d", a+b)
	prompt = fmt.Sprintf("Calculate: %d + %d = ? Reply ONLY: %s", a, b, expected)
	return prompt, expected
}

func extractOpenAIProbeContent(raw []byte) string {
	var parsed struct {
		Choices []struct {
			Message struct {
				Content any `json:"content"`
			} `json:"message"`
			Text string `json:"text"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return strings.TrimSpace(string(raw))
	}
	if len(parsed.Choices) == 0 {
		return strings.TrimSpace(string(raw))
	}
	if parsed.Choices[0].Text != "" {
		return strings.TrimSpace(parsed.Choices[0].Text)
	}
	switch content := parsed.Choices[0].Message.Content.(type) {
	case string:
		return strings.TrimSpace(content)
	case []any:
		parts := make([]string, 0, len(content))
		for _, item := range content {
			if obj, ok := item.(map[string]any); ok {
				if text, ok := obj["text"].(string); ok {
					parts = append(parts, text)
				}
			}
		}
		return strings.TrimSpace(strings.Join(parts, ""))
	default:
		return strings.TrimSpace(string(raw))
	}
}

func extractAnthropicProbeContent(raw []byte) string {
	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return strings.TrimSpace(string(raw))
	}
	if len(parsed.Content) == 0 {
		return strings.TrimSpace(string(raw))
	}
	parts := make([]string, 0, len(parsed.Content))
	for _, item := range parsed.Content {
		if item.Text != "" {
			parts = append(parts, item.Text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, ""))
}

func normalizeInt64IDs(ids []int64) []int64 {
	if len(ids) == 0 {
		return []int64{}
	}
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
