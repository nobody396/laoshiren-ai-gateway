package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
)

func TestRunSupplierProbeTreatsOKWithPunctuationAsReachable(t *testing.T) {
	result := runSupplierProbe(context.Background(), testSupplierProbeServer(t, `OK.`))

	if result.Status != SupplierProbeStatusSuccess {
		t.Fatalf("expected probe success, got status=%q error=%q", result.Status, result.ErrorMessage)
	}
	if result.SubStatus != SupplierProbeSubStatusNone {
		t.Fatalf("expected no sub status, got %q", result.SubStatus)
	}
	if result.AccuracyOK {
		t.Fatalf("expected accuracy_ok=false because OK. does not answer the dynamic probe")
	}
	if result.ResponseText != "OK." {
		t.Fatalf("expected response text to be preserved, got %q", result.ResponseText)
	}
}

func TestRunSupplierProbeMarksExpectedOpenAIAnswerAccurate(t *testing.T) {
	result := runSupplierProbe(context.Background(), testSupplierProbeServer(t, "__EXPECTED__"))

	if result.Status != SupplierProbeStatusSuccess {
		t.Fatalf("expected probe success, got status=%q error=%q", result.Status, result.ErrorMessage)
	}
	if !result.AccuracyOK {
		t.Fatalf("expected accuracy_ok for dynamic expected answer, response=%q", result.ResponseText)
	}
}

func TestRunSupplierProbeTreatsAnyNonEmptyModelResponseAsReachable(t *testing.T) {
	result := runSupplierProbe(context.Background(), testSupplierProbeServer(t, `service reachable`))

	if result.Status != SupplierProbeStatusSuccess {
		t.Fatalf("expected probe success, got status=%q error=%q", result.Status, result.ErrorMessage)
	}
	if result.SubStatus != SupplierProbeSubStatusNone {
		t.Fatalf("expected no sub status, got %q", result.SubStatus)
	}
	if result.AccuracyOK {
		t.Fatalf("expected accuracy_ok=false for mismatched response")
	}
	if result.ErrorMessage != "" {
		t.Fatalf("expected no mismatch error, got %q", result.ErrorMessage)
	}
}

func TestRunSupplierProbeFailsEmptyModelResponse(t *testing.T) {
	result := runSupplierProbe(context.Background(), testSupplierProbeServer(t, ``))

	if result.Status != SupplierProbeStatusDegraded {
		t.Fatalf("expected probe degraded for empty response, got status=%q", result.Status)
	}
	if result.SubStatus != SupplierProbeSubStatusContentMismatch {
		t.Fatalf("expected content mismatch, got %q", result.SubStatus)
	}
	if result.ErrorMessage != "probe response was empty" {
		t.Fatalf("expected empty response error, got %q", result.ErrorMessage)
	}
}

func TestRunSupplierProbeUsesCheapClaudeHaikuProbe(t *testing.T) {
	var sawMessagesEndpoint bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Fatalf("unexpected probe path %q", r.URL.Path)
		}
		if r.URL.Query().Get("beta") != "true" {
			t.Fatalf("expected beta=true query, got %q", r.URL.RawQuery)
		}
		if !strings.Contains(r.Header.Get("User-Agent"), "claude-cli/") {
			t.Fatalf("expected claude-cli user agent, got %q", r.Header.Get("User-Agent"))
		}
		var payload struct {
			Model     string `json:"model"`
			MaxTokens int    `json:"max_tokens"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload.Model != DefaultSupplierProbeModel {
			t.Fatalf("expected model %q, got %q", DefaultSupplierProbeModel, payload.Model)
		}
		if payload.MaxTokens != 1 {
			t.Fatalf("expected max_tokens=1, got %d", payload.MaxTokens)
		}
		sawMessagesEndpoint = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"content":[{"type":"text","text":"#"}]}`)
	}))
	t.Cleanup(server.Close)

	result := runSupplierProbe(context.Background(), &Supplier{
		ID:         1,
		BaseURL:    server.URL,
		APIKey:     "sk-test",
		ProbeModel: DefaultSupplierProbeModel,
	})

	if !sawMessagesEndpoint {
		t.Fatalf("expected Claude messages endpoint to be called")
	}
	if result.Status != SupplierProbeStatusSuccess {
		t.Fatalf("expected probe success, got status=%q error=%q", result.Status, result.ErrorMessage)
	}
	if !result.AccuracyOK {
		t.Fatalf("expected accuracy_ok for # response")
	}
	if result.HTTPCode != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d", result.HTTPCode)
	}
	if result.ResponseText != "#" {
		t.Fatalf("expected response text #, got %q", result.ResponseText)
	}
}

func TestRunSupplierProbeClassifiesRateLimitAsDegraded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = fmt.Fprint(w, `rate limited`)
	}))
	t.Cleanup(server.Close)

	result := runSupplierProbe(context.Background(), &Supplier{
		ID:         1,
		BaseURL:    server.URL,
		APIKey:     "sk-test",
		ProbeModel: "gpt-5.1-codex-mini",
	})

	if result.Status != SupplierProbeStatusDegraded {
		t.Fatalf("expected degraded for 429, got status=%q error=%q", result.Status, result.ErrorMessage)
	}
	if result.SubStatus != SupplierProbeSubStatusRateLimit {
		t.Fatalf("expected rate_limit, got %q", result.SubStatus)
	}
	if result.HTTPCode != http.StatusTooManyRequests {
		t.Fatalf("expected HTTP 429, got %d", result.HTTPCode)
	}
}

func TestSupplierFromAccountSkipsUnschedulableAccounts(t *testing.T) {
	supplier, reason := supplierFromAccount(Account{
		ID:          12,
		Name:        "disabled-routing",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: false,
		Credentials: map[string]any{
			"base_url": "https://example.invalid",
			"api_key":  "sk-test",
		},
	})

	if supplier != nil {
		t.Fatalf("expected no supplier for unschedulable account, got %#v", supplier)
	}
	if reason != "账号未启用调度" {
		t.Fatalf("expected unschedulable skip reason, got %q", reason)
	}
}

func TestSupplierFromAccountDisablesAutomaticProbeByDefault(t *testing.T) {
	supplier, reason := supplierFromAccount(Account{
		ID:          39,
		Name:        "image-only-route",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{
			"base_url": "https://images.example.invalid",
			"api_key":  "sk-test",
		},
	})

	if reason != "" {
		t.Fatalf("unexpected skip reason: %q", reason)
	}
	if supplier == nil {
		t.Fatal("expected supplier")
	}
	if supplier.ProbeEnabled {
		t.Fatal("account-synced suppliers must not start paid or protocol-incompatible probes automatically")
	}
}

func TestSupplierProbeSnapshotShowsDisabledProbeWithoutCountingIt(t *testing.T) {
	now := time.Now()
	sourceOne := int64(101)
	sourceTwo := int64(102)
	repo := &supplierProbeSnapshotRepo{
		suppliers: []Supplier{
			{
				ID:                   1,
				Name:                 "enabled",
				SourceAccountID:      &sourceOne,
				SourcePlatform:       PlatformOpenAI,
				ProbeEnabled:         true,
				ProbeModel:           DefaultSupplierOpenAIProbeModel,
				ProbeIntervalMinutes: DefaultSupplierProbeIntervalMinutes,
				LastProbeStatus:      SupplierProbeStatusSuccess,
			},
			{
				ID:                   2,
				Name:                 "paused",
				SourceAccountID:      &sourceTwo,
				SourcePlatform:       PlatformOpenAI,
				ProbeEnabled:         false,
				ProbeModel:           DefaultSupplierOpenAIProbeModel,
				ProbeIntervalMinutes: DefaultSupplierProbeIntervalMinutes,
				LastProbeStatus:      SupplierProbeStatusFailed,
			},
		},
		results: []SupplierProbeResult{
			{ID: 1, SupplierID: 1, Status: SupplierProbeStatusSuccess, LatencyMs: 100, CheckedAt: now.Add(-5 * time.Minute)},
			{ID: 2, SupplierID: 2, Status: SupplierProbeStatusFailed, LatencyMs: 200, CheckedAt: now.Add(-4 * time.Minute)},
		},
	}
	svc := NewSupplierService(repo, nil)

	snapshot, err := svc.GetProbeSnapshot(context.Background(), 60, 7)
	if err != nil {
		t.Fatalf("get snapshot: %v", err)
	}
	if snapshot.TotalSuppliers != 2 {
		t.Fatalf("expected total suppliers to include disabled probes, got %d", snapshot.TotalSuppliers)
	}
	if snapshot.EnabledSuppliers != 1 {
		t.Fatalf("expected only enabled probes counted as enabled, got %d", snapshot.EnabledSuppliers)
	}
	if len(snapshot.Suppliers) != 2 {
		t.Fatalf("expected both enabled and disabled probes in snapshot rows, got %d", len(snapshot.Suppliers))
	}
	if snapshot.WindowTotal != 1 || snapshot.WindowSuccess != 1 || snapshot.WindowFailed != 0 {
		t.Fatalf("expected disabled probe history excluded from global window, got total=%d success=%d failed=%d", snapshot.WindowTotal, snapshot.WindowSuccess, snapshot.WindowFailed)
	}

	var paused SupplierProbeSnapshotItem
	for _, item := range snapshot.Suppliers {
		if item.ID == 2 {
			paused = item
			break
		}
	}
	if paused.ID == 0 {
		t.Fatalf("expected disabled probe row to remain visible")
	}
	if paused.ProbeEnabled {
		t.Fatalf("expected paused row to preserve probe_enabled=false")
	}
	if paused.WindowTotal != 0 {
		t.Fatalf("expected paused row not to contribute window metrics, got %d", paused.WindowTotal)
	}
	if len(paused.Points) != 1 {
		t.Fatalf("expected paused row to keep recent history points, got %d", len(paused.Points))
	}
}

func TestRunSupplierProbeRetriesFailedRedStatus(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = fmt.Fprint(w, `temporary bad gateway`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"choices":[{"message":{"content":"recovered"}}]}`)
	}))
	t.Cleanup(server.Close)

	result := runSupplierProbe(context.Background(), &Supplier{
		ID:         1,
		BaseURL:    server.URL,
		APIKey:     "sk-test",
		ProbeModel: "gpt-5.1-codex-mini",
	})

	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
	if result.Status != SupplierProbeStatusSuccess {
		t.Fatalf("expected recovered attempt to be success, got status=%q", result.Status)
	}
	if result.SubStatus != SupplierProbeSubStatusNone {
		t.Fatalf("expected no sub status after recovery, got %q", result.SubStatus)
	}
}

type supplierProbeSnapshotRepo struct {
	suppliers []Supplier
	results   []SupplierProbeResult
}

func (r *supplierProbeSnapshotRepo) Create(ctx context.Context, supplier *Supplier) error {
	return nil
}

func (r *supplierProbeSnapshotRepo) GetByID(ctx context.Context, id int64) (*Supplier, error) {
	for i := range r.suppliers {
		if r.suppliers[i].ID == id {
			supplier := r.suppliers[i]
			return &supplier, nil
		}
	}
	return nil, ErrSupplierNotFound
}

func (r *supplierProbeSnapshotRepo) Update(ctx context.Context, supplier *Supplier, replaceGroups bool, replaceAccounts bool) error {
	return nil
}

func (r *supplierProbeSnapshotRepo) BulkSetProbeEnabled(ctx context.Context, ids []int64, enabled bool) (int64, error) {
	return 0, nil
}

func (r *supplierProbeSnapshotRepo) ListDueProbes(ctx context.Context, limit int) ([]Supplier, error) {
	return nil, nil
}

func (r *supplierProbeSnapshotRepo) ListProbeResultsSince(ctx context.Context, since time.Time) ([]SupplierProbeResult, error) {
	results := make([]SupplierProbeResult, 0, len(r.results))
	for _, result := range r.results {
		if !result.CheckedAt.Before(since) {
			results = append(results, result)
		}
	}
	return results, nil
}

func (r *supplierProbeSnapshotRepo) ListProbeResultsBySupplier(ctx context.Context, supplierID int64, since time.Time, limit int) ([]SupplierProbeResult, error) {
	return nil, nil
}

func (r *supplierProbeSnapshotRepo) RecordProbeResult(ctx context.Context, supplierID int64, result *SupplierProbeResult) error {
	return nil
}

func (r *supplierProbeSnapshotRepo) UpsertFromAccount(ctx context.Context, supplier *Supplier) (bool, error) {
	return false, nil
}

func (r *supplierProbeSnapshotRepo) PruneAccountSuppliers(ctx context.Context, activeSourceAccountIDs []int64) (int64, error) {
	return 0, nil
}

func (r *supplierProbeSnapshotRepo) Delete(ctx context.Context, id int64) error {
	return nil
}

func (r *supplierProbeSnapshotRepo) List(ctx context.Context, params pagination.PaginationParams, filter SupplierListFilter) ([]Supplier, *pagination.PaginationResult, error) {
	suppliers := append([]Supplier(nil), r.suppliers...)
	return suppliers, &pagination.PaginationResult{Total: int64(len(suppliers)), Page: 1, PageSize: len(suppliers), Pages: 1}, nil
}

func testSupplierProbeServer(t *testing.T, content string) *Supplier {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected probe path %q", r.URL.Path)
		}
		if content == "__EXPECTED__" {
			var payload struct {
				Messages []struct {
					Content string `json:"content"`
				} `json:"messages"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if len(payload.Messages) == 0 {
				t.Fatalf("expected at least one message")
			}
			const marker = "LSR_PROBE="
			idx := strings.LastIndex(payload.Messages[0].Content, marker)
			if idx < 0 {
				t.Fatalf("expected dynamic marker in prompt %q", payload.Messages[0].Content)
			}
			content = strings.TrimSpace(payload.Messages[0].Content[idx:])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"choices":[{"message":{"content":%q}}]}`, content)
	}))
	t.Cleanup(server.Close)

	return &Supplier{
		ID:         1,
		BaseURL:    server.URL,
		APIKey:     "sk-test",
		ProbeModel: "gpt-5.1-codex-mini",
	}
}
