package service

import (
	"context"
	"errors"
	"testing"
)

func geminiBridgeTestAccount(id int64, modelMapping map[string]any) Account {
	credentials := map[string]any{
		"api_key":   "sk-gemini-test",
		"base_url":  "https://hk2.pomoai.xyz",
		"pool_mode": true,
	}
	if modelMapping != nil {
		credentials["model_mapping"] = modelMapping
	}
	return Account{
		ID:          id,
		Name:        "gemini-bridge",
		Status:      StatusActive,
		Schedulable: true,
		Platform:    PlatformGemini,
		Type:        AccountTypeAPIKey,
		Credentials: credentials,
		Concurrency: 1,
		AccountGroups: []AccountGroup{
			{AccountID: id, GroupID: 57},
		},
	}
}

func TestGeminiChatBridgeAccountEligible(t *testing.T) {
	mapped := map[string]any{"gemini-3.1-pro": "gemini-3.1-pro"}

	cases := []struct {
		name     string
		account  *Account
		model    string
		excluded map[int64]struct{}
		want     bool
	}{
		{
			name:    "gemini apikey account is eligible",
			account: ptrAccount(geminiBridgeTestAccount(1, mapped)),
			model:   "gemini-3.1-pro",
			want:    true,
		},
		{
			name:    "model not in mapping is rejected",
			account: ptrAccount(geminiBridgeTestAccount(1, mapped)),
			model:   "gemini-3.7-flash",
			want:    false,
		},
		{
			name:    "account without mapping allows any model",
			account: ptrAccount(geminiBridgeTestAccount(1, nil)),
			model:   "gemini-3.7-flash",
			want:    true,
		},
		{
			name: "gemini oauth account is not bridge-eligible",
			account: &Account{
				ID: 2, Status: StatusActive, Schedulable: true,
				Platform: PlatformGemini, Type: AccountTypeOAuth,
			},
			model: "gemini-3.1-pro",
			want:  false,
		},
		{
			name:    "openai account is not bridge-eligible",
			account: ptrAccount(testOpenAIAccount(3, nil)),
			model:   "gemini-3.1-pro",
			want:    false,
		},
		{
			name:     "excluded account is rejected",
			account:  ptrAccount(geminiBridgeTestAccount(4, mapped)),
			model:    "gemini-3.1-pro",
			excluded: map[int64]struct{}{4: {}},
			want:     false,
		},
		{
			name: "unschedulable account is rejected",
			account: &Account{
				ID: 5, Status: StatusActive, Schedulable: false,
				Platform: PlatformGemini, Type: AccountTypeAPIKey,
			},
			model: "gemini-3.1-pro",
			want:  false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := geminiChatBridgeAccountEligible(tc.account, tc.model, tc.excluded); got != tc.want {
				t.Fatalf("geminiChatBridgeAccountEligible = %v, want %v", got, tc.want)
			}
		})
	}
}

func ptrAccount(acc Account) *Account { return &acc }

func TestSelectGeminiAccountWithScheduler_SelectsApikeyAccount(t *testing.T) {
	groupID := int64(57)
	repo := &groupAccountsStubRepo{
		groupAccounts: []Account{
			geminiBridgeTestAccount(11, map[string]any{"gemini-3.1-pro": "gemini-3.1-pro"}),
		},
	}
	svc := &OpenAIGatewayService{accountRepo: repo, cache: &stubGatewayCache{}}

	selection, decision, err := svc.SelectGeminiAccountWithScheduler(
		context.Background(), &groupID, "", "gemini-3.1-pro", nil,
	)
	if err != nil {
		t.Fatalf("SelectGeminiAccountWithScheduler error: %v", err)
	}
	if selection == nil || selection.Account == nil || selection.Account.ID != 11 {
		t.Fatalf("expected account 11 selected, got %+v", selection)
	}
	if !selection.Acquired {
		t.Fatalf("expected concurrency slot acquired")
	}
	if decision.CandidateCount != 1 {
		t.Fatalf("CandidateCount = %d, want 1", decision.CandidateCount)
	}
}

func TestSelectGeminiAccountWithScheduler_UnknownModelIsModelNotSupported(t *testing.T) {
	groupID := int64(57)
	repo := &groupAccountsStubRepo{
		groupAccounts: []Account{
			geminiBridgeTestAccount(11, map[string]any{"gemini-3.1-pro": "gemini-3.1-pro"}),
		},
	}
	svc := &OpenAIGatewayService{accountRepo: repo, cache: &stubGatewayCache{}}

	_, _, err := svc.SelectGeminiAccountWithScheduler(
		context.Background(), &groupID, "", "gpt-5", nil,
	)
	var modelErr *ModelNotSupportedError
	if !errors.As(err, &modelErr) {
		t.Fatalf("expected *ModelNotSupportedError, got %T: %v", err, err)
	}
	if modelErr.Platform != PlatformGemini {
		t.Fatalf("Platform = %q, want %q", modelErr.Platform, PlatformGemini)
	}
}

func TestSelectGeminiAccountWithScheduler_NoGeminiAccountsKeepsNoAvailable(t *testing.T) {
	groupID := int64(57)
	repo := &groupAccountsStubRepo{
		groupAccounts: []Account{
			testOpenAIAccount(21, nil),
		},
	}
	svc := &OpenAIGatewayService{accountRepo: repo, cache: &stubGatewayCache{}}

	_, _, err := svc.SelectGeminiAccountWithScheduler(
		context.Background(), &groupID, "", "gemini-3.1-pro", nil,
	)
	if err == nil {
		t.Fatalf("expected error")
	}
	var modelErr *ModelNotSupportedError
	if errors.As(err, &modelErr) {
		t.Fatalf("must not be ModelNotSupportedError, got %v", err)
	}
	if !errors.Is(err, ErrNoAvailableAccounts) {
		t.Fatalf("expected ErrNoAvailableAccounts semantics, got %v", err)
	}
}

func TestSelectOpenAICompatible_GeminiChatCompletionsRouteUsesGeminiScheduler(t *testing.T) {
	groupID := int64(57)
	repo := &groupAccountsStubRepo{
		groupAccounts: []Account{
			geminiBridgeTestAccount(11, map[string]any{"gemini-3.1-pro": "gemini-3.1-pro"}),
		},
	}
	svc := &OpenAIGatewayService{accountRepo: repo, cache: &stubGatewayCache{}}

	selection, _, err := svc.SelectOpenAICompatibleAccountWithSchedulerForRouting(
		context.Background(),
		PlatformGemini,
		&groupID,
		"",
		"",
		"gemini-3.1-pro",
		nil,
		OpenAIUpstreamTransportAny,
		false,
		false,
		"/v1/chat/completions",
	)
	if err != nil {
		t.Fatalf("expected gemini account selected, got error: %v", err)
	}
	if selection == nil || selection.Account == nil || selection.Account.ID != 11 {
		t.Fatalf("expected gemini account 11, got %+v", selection)
	}
}

func TestSelectOpenAICompatible_GeminiNonChatRouteKeepsOpenAISemantics(t *testing.T) {
	groupID := int64(57)
	repo := &groupAccountsStubRepo{
		groupAccounts: []Account{
			geminiBridgeTestAccount(11, map[string]any{"gemini-3.1-pro": "gemini-3.1-pro"}),
		},
	}
	svc := &OpenAIGatewayService{accountRepo: repo, cache: &stubGatewayCache{}}

	// /v1/responses（无 routeEndpoints 参数）不能把 Gemini 账号选进来：
	// 该端点没有 Gemini 桥接实现，必须保持原有零候选失败语义。
	selection, _, err := svc.SelectOpenAICompatibleAccountWithSchedulerForRouting(
		context.Background(),
		PlatformGemini,
		&groupID,
		"",
		"",
		"gemini-3.1-pro",
		nil,
		OpenAIUpstreamTransportAny,
		false,
		false,
	)
	if err == nil {
		t.Fatalf("expected error, got selection %+v", selection)
	}
	var noServableErr *NoServableAccountsError
	if !errors.As(err, &noServableErr) {
		t.Fatalf("expected *NoServableAccountsError, got %T: %v", err, err)
	}
}

func TestSelectOpenAICompatible_GeminiUnknownModelSurfacedAsModelNotSupported(t *testing.T) {
	groupID := int64(57)
	repo := &groupAccountsStubRepo{
		groupAccounts: []Account{
			geminiBridgeTestAccount(11, map[string]any{"gemini-3.1-pro": "gemini-3.1-pro"}),
		},
	}
	svc := &OpenAIGatewayService{accountRepo: repo, cache: &stubGatewayCache{}}

	_, _, err := svc.SelectOpenAICompatibleAccountWithSchedulerForRouting(
		context.Background(),
		PlatformGemini,
		&groupID,
		"",
		"",
		"gpt-5",
		nil,
		OpenAIUpstreamTransportAny,
		false,
		false,
		"/v1/chat/completions",
	)
	var modelErr *ModelNotSupportedError
	if !errors.As(err, &modelErr) {
		t.Fatalf("expected *ModelNotSupportedError, got %T: %v", err, err)
	}
}

// TestSelectOpenAICompatible_OpenAIRouteUnchanged 回归：openai 平台请求仍走
// 原有 openai 调度器并选中 openai 账号。
func TestSelectOpenAICompatible_OpenAIRouteUnchanged(t *testing.T) {
	groupID := int64(8)
	repo := &groupAccountsStubRepo{
		groupAccounts: []Account{
			testOpenAIAccount(31, map[string]any{"gpt-5": "gpt-5"}),
			geminiBridgeTestAccount(32, map[string]any{"gemini-3.1-pro": "gemini-3.1-pro"}),
		},
	}
	svc := &OpenAIGatewayService{accountRepo: repo, cache: &stubGatewayCache{}}

	selection, _, err := svc.SelectOpenAICompatibleAccountWithSchedulerForRouting(
		context.Background(),
		PlatformOpenAI,
		&groupID,
		"",
		"",
		"gpt-5",
		nil,
		OpenAIUpstreamTransportAny,
		false,
		false,
		"/v1/chat/completions",
	)
	if err != nil {
		t.Fatalf("expected openai account selected, got error: %v", err)
	}
	if selection == nil || selection.Account == nil || selection.Account.ID != 31 {
		t.Fatalf("expected openai account 31, got %+v", selection)
	}
}
