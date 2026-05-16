//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type updateAccountCredentialsRepoStub struct {
	mockAccountRepoForGemini
	account     *Account
	updateCalls int
}

func (r *updateAccountCredentialsRepoStub) GetByID(ctx context.Context, id int64) (*Account, error) {
	return r.account, nil
}

func (r *updateAccountCredentialsRepoStub) Update(ctx context.Context, account *Account) error {
	r.updateCalls++
	r.account = account
	return nil
}

func TestUpdateAccountCredentialsPreservesMissingSensitiveKeysAfterRedactedEdit(t *testing.T) {
	accountID := int64(501)
	repo := &updateAccountCredentialsRepoStub{
		account: &Account{
			ID:       accountID,
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Status:   StatusActive,
			Credentials: map[string]any{
				"api_key":      "C2_EXISTING_API_KEY_SENTINEL",
				"access_token": "C2_EXISTING_ACCESS_TOKEN_SENTINEL",
				"base_url":     "https://old.example.com",
			},
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	updated, err := svc.UpdateAccount(context.Background(), accountID, &UpdateAccountInput{
		Credentials: map[string]any{
			"base_url":                  "https://new.example.com",
			"model_mapping":             map[string]any{"gpt-5": "gpt-5.1"},
			"intercept_warmup_requests": true,
		},
	})

	require.NoError(t, err)
	require.NotNil(t, updated)
	require.Equal(t, 1, repo.updateCalls)
	require.Equal(t, "C2_EXISTING_API_KEY_SENTINEL", repo.account.Credentials["api_key"])
	require.Equal(t, "C2_EXISTING_ACCESS_TOKEN_SENTINEL", repo.account.Credentials["access_token"])
	require.Equal(t, "https://new.example.com", repo.account.Credentials["base_url"])
	require.Equal(t, true, repo.account.Credentials["intercept_warmup_requests"])
	require.Equal(t, map[string]any{"gpt-5": "gpt-5.1"}, repo.account.Credentials["model_mapping"])
}

func TestUpdateAccountCredentialsExplicitSensitiveKeyReplacesExistingValue(t *testing.T) {
	accountID := int64(502)
	repo := &updateAccountCredentialsRepoStub{
		account: &Account{
			ID:       accountID,
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Status:   StatusActive,
			Credentials: map[string]any{
				"api_key":      "C2_OLD_API_KEY_SENTINEL",
				"access_token": "C2_EXISTING_ACCESS_TOKEN_SENTINEL",
				"base_url":     "https://old.example.com",
			},
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	updated, err := svc.UpdateAccount(context.Background(), accountID, &UpdateAccountInput{
		Credentials: map[string]any{
			"api_key":  "C2_NEW_API_KEY_SENTINEL",
			"base_url": "https://new.example.com",
		},
	})

	require.NoError(t, err)
	require.NotNil(t, updated)
	require.Equal(t, 1, repo.updateCalls)
	require.Equal(t, "C2_NEW_API_KEY_SENTINEL", repo.account.Credentials["api_key"])
	require.Equal(t, "C2_EXISTING_ACCESS_TOKEN_SENTINEL", repo.account.Credentials["access_token"])
	require.Equal(t, "https://new.example.com", repo.account.Credentials["base_url"])
}
