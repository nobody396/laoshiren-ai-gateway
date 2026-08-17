package service

import (
	"errors"
	"testing"
)

func testOpenAIAccount(id int64, modelMapping map[string]any) Account {
	credentials := map[string]any{}
	if modelMapping != nil {
		credentials["model_mapping"] = modelMapping
	}
	return Account{
		ID:          id,
		Status:      StatusActive,
		Schedulable: true,
		Platform:    PlatformOpenAI,
		Credentials: credentials,
		Extra:       map[string]any{},
		Concurrency: 1,
		Priority:    0,
	}
}

func TestAllOpenAICandidatesModelUnsupported(t *testing.T) {
	gpt5Mapping := map[string]any{"gpt-5": "gpt-5"}
	openAIMapped := testOpenAIAccount(1, gpt5Mapping)
	openAIUnmapped := testOpenAIAccount(2, nil)
	openAIExcluded := testOpenAIAccount(3, nil)
	excludedIDs := map[int64]struct{}{openAIExcluded.ID: {}}

	cases := []struct {
		name          string
		accounts      []Account
		excludedIDs   map[int64]struct{}
		model         string
		wantSupported bool
	}{
		{
			name:          "empty accounts never reports unsupported",
			accounts:      nil,
			model:         "mimo-v2.5-pro",
			wantSupported: false,
		},
		{
			name: "all openai accounts mapped elsewhere -> unsupported",
			accounts: []Account{
				testOpenAIAccount(11, gpt5Mapping),
				testOpenAIAccount(12, gpt5Mapping),
			},
			model:         "mimo-v2.5-pro",
			wantSupported: true,
		},
		{
			name: "retired luna remains unsupported for ordinary text routing",
			accounts: []Account{
				testOpenAIAccount(13, map[string]any{
					"gpt-5.6-sol":   "gpt-5.6-sol",
					"gpt-5.6-terra": "gpt-5.6-terra",
				}),
			},
			model:         "gpt-5.6-luna",
			wantSupported: true,
		},
		{
			name: "one account has no mapping (allow all) -> supported",
			accounts: []Account{
				testOpenAIAccount(21, gpt5Mapping),
				openAIUnmapped,
			},
			model:         "mimo-v2.5-pro",
			wantSupported: false,
		},
		{
			name: "excluded account skipped",
			accounts: []Account{
				openAIMapped,
				openAIExcluded,
			},
			excludedIDs:   excludedIDs,
			model:         "mimo-v2.5-pro",
			wantSupported: true,
		},
		{
			name: "non-openai accounts are not counted",
			accounts: []Account{
				{ID: 31, Status: StatusActive, Schedulable: true, Platform: PlatformGemini, Credentials: map[string]any{"model_mapping": map[string]any{"gemini-2.5-pro": "gemini-2.5-pro"}}},
				openAIMapped,
			},
			model:         "mimo-v2.5-pro",
			wantSupported: true,
		},
		{
			name:          "empty model never reports unsupported",
			accounts:      []Account{openAIMapped, openAIUnmapped},
			model:         "",
			wantSupported: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := allOpenAICandidatesModelUnsupported(tc.accounts, tc.excludedIDs, tc.model); got != tc.wantSupported {
				t.Fatalf("allOpenAICandidatesModelUnsupported = %v, want %v", got, tc.wantSupported)
			}
		})
	}
}

func TestNoAvailableOpenAISelectionError_ModelNotSupported(t *testing.T) {
	accounts := []Account{
		testOpenAIAccount(1, map[string]any{"gpt-5": "gpt-5"}),
	}

	err := noAvailableOpenAISelectionError("mimo-v2.5-pro", false, accounts, nil)
	var modelErr *ModelNotSupportedError
	if !errors.As(err, &modelErr) {
		t.Fatalf("expected *ModelNotSupportedError, got %T: %v", err, err)
	}
	if modelErr.RequestedModel != "mimo-v2.5-pro" {
		t.Fatalf("RequestedModel = %q, want mimo-v2.5-pro", modelErr.RequestedModel)
	}
	if msg := modelErr.Error(); msg != "no available accounts supporting model: mimo-v2.5-pro (model not supported)" {
		t.Fatalf("unexpected error message: %s", msg)
	}
	if !errors.Is(err, ErrNoAvailableAccounts) {
		t.Fatalf("ModelNotSupportedError should keep ErrNoAvailableAccounts semantics for ops log filtering")
	}

	err = noAvailableOpenAISelectionError("gpt-5", false, accounts, nil)
	if errors.As(err, &modelErr) {
		t.Fatalf("did not expect *ModelNotSupportedError for supported model, got %v", err)
	}

	err = noAvailableOpenAISelectionError("mimo-v2.5-pro", true, accounts, nil)
	if !errors.Is(err, ErrNoAvailableCompactAccounts) {
		t.Fatalf("compact-blocked must keep ErrNoAvailableCompactAccounts, got %v", err)
	}
}
