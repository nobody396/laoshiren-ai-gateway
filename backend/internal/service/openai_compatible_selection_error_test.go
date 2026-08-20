package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

// groupAccountsStubRepo 提供分组内账号列表的可控桩实现，
// 用于验证零候选失败时“结构性不可服务”与“暂时性不可用”的区分。
type groupAccountsStubRepo struct {
	AccountRepository
	groupAccounts    []Account
	listByGroupCalls int
}

func (r *groupAccountsStubRepo) ListByGroup(ctx context.Context, groupID int64) ([]Account, error) {
	r.listByGroupCalls++
	return append([]Account(nil), r.groupAccounts...), nil
}

func (r *groupAccountsStubRepo) GetByID(ctx context.Context, id int64) (*Account, error) {
	for i := range r.groupAccounts {
		if r.groupAccounts[i].ID == id {
			acc := r.groupAccounts[i]
			return &acc, nil
		}
	}
	return nil, errors.New("account not found")
}

func (r *groupAccountsStubRepo) ListSchedulableByGroupIDAndPlatform(ctx context.Context, groupID int64, platform string) ([]Account, error) {
	var result []Account
	for _, acc := range r.groupAccounts {
		if acc.Platform == platform {
			result = append(result, acc)
		}
	}
	return result, nil
}

func (r *groupAccountsStubRepo) ListSchedulableUngroupedByPlatform(ctx context.Context, platform string) ([]Account, error) {
	return nil, nil
}

func selectOpenAICompatibleForTest(svc *OpenAIGatewayService, platform string, groupID *int64, model string) error {
	_, _, err := svc.SelectOpenAICompatibleAccountWithSchedulerForRouting(
		context.Background(),
		platform,
		groupID,
		"",
		"",
		model,
		nil,
		OpenAIUpstreamTransportAny,
		false,
		false,
		"/v1/chat/completions",
	)
	return err
}

func TestSelectOpenAICompatible_GroupWithoutPlatformAccountsIsStructural(t *testing.T) {
	groupID := int64(57)
	repo := &groupAccountsStubRepo{
		groupAccounts: []Account{
			{ID: 1, Platform: PlatformGemini, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1},
		},
	}
	svc := &OpenAIGatewayService{accountRepo: repo, cache: &stubGatewayCache{}}

	err := selectOpenAICompatibleForTest(svc, PlatformOpenAI, &groupID, "gemini-3.1-pro")
	var noServableErr *NoServableAccountsError
	if !errors.As(err, &noServableErr) {
		t.Fatalf("expected *NoServableAccountsError, got %T: %v", err, err)
	}
	if !errors.Is(err, ErrNoAvailableAccounts) {
		t.Fatalf("NoServableAccountsError should keep ErrNoAvailableAccounts semantics, got %v", err)
	}
	if noServableErr.Platform != PlatformOpenAI {
		t.Fatalf("Platform = %q, want %q", noServableErr.Platform, PlatformOpenAI)
	}
}

func TestSelectOpenAICompatible_TransientStarvationStaysOriginalError(t *testing.T) {
	groupID := int64(8)
	resetAt := time.Now().Add(1 * time.Hour)
	repo := &groupAccountsStubRepo{
		groupAccounts: []Account{
			{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, RateLimitResetAt: &resetAt},
		},
	}
	svc := &OpenAIGatewayService{accountRepo: repo, cache: &stubGatewayCache{}}

	err := selectOpenAICompatibleForTest(svc, PlatformOpenAI, &groupID, "gpt-5")
	if err == nil {
		t.Fatalf("expected error")
	}
	var noServableErr *NoServableAccountsError
	if errors.As(err, &noServableErr) {
		t.Fatalf("rate-limited accounts must not be reclassified as structural, got %v", err)
	}
	if !errors.Is(err, ErrNoAvailableAccounts) {
		t.Fatalf("expected ErrNoAvailableAccounts semantics, got %v", err)
	}
}

func TestSelectOpenAICompatible_ModelNotSupportedPassthrough(t *testing.T) {
	groupID := int64(9)
	repo := &groupAccountsStubRepo{
		groupAccounts: []Account{
			{
				ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1,
				Credentials: map[string]any{"model_mapping": map[string]any{"gpt-5": "gpt-5"}},
			},
		},
	}
	svc := &OpenAIGatewayService{accountRepo: repo, cache: &stubGatewayCache{}}

	err := selectOpenAICompatibleForTest(svc, PlatformOpenAI, &groupID, "gpt-5.6-sol")
	var modelErr *ModelNotSupportedError
	if !errors.As(err, &modelErr) {
		t.Fatalf("expected *ModelNotSupportedError, got %T: %v", err, err)
	}
	var noServableErr *NoServableAccountsError
	if errors.As(err, &noServableErr) {
		t.Fatalf("ModelNotSupportedError must not be rewrapped, got %v", err)
	}
	if modelErr.RequestedModel != "gpt-5.6-sol" {
		t.Fatalf("RequestedModel = %q, want gpt-5.6-sol", modelErr.RequestedModel)
	}
}

func TestSelectOpenAICompatible_NilGroupKeepsOriginalError(t *testing.T) {
	repo := &groupAccountsStubRepo{}
	svc := &OpenAIGatewayService{accountRepo: repo, cache: &stubGatewayCache{}}

	err := selectOpenAICompatibleForTest(svc, PlatformOpenAI, nil, "gpt-5")
	if err == nil {
		t.Fatalf("expected error")
	}
	var noServableErr *NoServableAccountsError
	if errors.As(err, &noServableErr) {
		t.Fatalf("ungrouped keys must keep the original 503-class error, got %v", err)
	}
}

func TestGroupHasAccountsForPlatform_CachesVerdict(t *testing.T) {
	groupID := int64(57)
	repo := &groupAccountsStubRepo{
		groupAccounts: []Account{
			{ID: 1, Platform: PlatformGemini, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true},
		},
	}
	svc := &OpenAIGatewayService{accountRepo: repo}

	if svc.groupHasAccountsForPlatform(context.Background(), &groupID, PlatformGemini) != true {
		t.Fatalf("expected group to have gemini accounts")
	}
	if svc.groupHasAccountsForPlatform(context.Background(), &groupID, PlatformOpenAI) != false {
		t.Fatalf("expected group to have no openai accounts")
	}
	if repo.listByGroupCalls != 2 {
		t.Fatalf("expected 2 ListByGroup calls, got %d", repo.listByGroupCalls)
	}

	// Second round must be served from the in-process verdict cache.
	svc.groupHasAccountsForPlatform(context.Background(), &groupID, PlatformGemini)
	svc.groupHasAccountsForPlatform(context.Background(), &groupID, PlatformOpenAI)
	if repo.listByGroupCalls != 2 {
		t.Fatalf("expected cached verdicts, got %d ListByGroup calls", repo.listByGroupCalls)
	}
}
