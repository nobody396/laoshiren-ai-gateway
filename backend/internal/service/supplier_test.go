package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunSupplierProbeTreatsOKWithPunctuationAsReachable(t *testing.T) {
	result := runSupplierProbe(context.Background(), testSupplierProbeServer(t, `OK.`))

	if result.Status != SupplierProbeStatusDegraded {
		t.Fatalf("expected probe degraded, got status=%q error=%q", result.Status, result.ErrorMessage)
	}
	if result.SubStatus != SupplierProbeSubStatusContentMismatch {
		t.Fatalf("expected content mismatch, got %q", result.SubStatus)
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

	if result.Status != SupplierProbeStatusDegraded {
		t.Fatalf("expected probe degraded, got status=%q error=%q", result.Status, result.ErrorMessage)
	}
	if result.SubStatus != SupplierProbeSubStatusContentMismatch {
		t.Fatalf("expected content mismatch, got %q", result.SubStatus)
	}
	if result.AccuracyOK {
		t.Fatalf("expected accuracy_ok=false for mismatched response")
	}
	if result.ErrorMessage != "probe response did not contain expected marker" {
		t.Fatalf("expected mismatch error, got %q", result.ErrorMessage)
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
	if result.Status != SupplierProbeStatusDegraded {
		t.Fatalf("expected recovered attempt to be degraded by content mismatch, got status=%q", result.Status)
	}
	if result.SubStatus != SupplierProbeSubStatusContentMismatch {
		t.Fatalf("expected content mismatch after recovery, got %q", result.SubStatus)
	}
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
