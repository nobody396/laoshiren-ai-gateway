package service

import (
	"context"
	"errors"
	"net/http"
)

// buildOpenAIAuthenticationHeaders mirrors the upstream non-agent-identity
// branch. Live account capability filtering excludes agent-identity accounts.
func (s *OpenAIGatewayService) buildOpenAIAuthenticationHeaders(_ context.Context, account *Account, token string) (http.Header, error) {
	if account == nil {
		return nil, errors.New("account is nil")
	}
	headers := make(http.Header)
	headers.Set("Authorization", "Bearer "+token)
	return headers, nil
}

func setOpenAIChatGPTAccountHeaders(headers http.Header, account *Account) {
	if headers == nil || account == nil || !account.IsOpenAIOAuth() {
		return
	}
	if chatgptAccountID := account.GetChatGPTAccountID(); chatgptAccountID != "" {
		headers.Set("chatgpt-account-id", chatgptAccountID)
	}
	if account.IsChatGPTAccountFedRAMP() {
		headers.Set("x-openai-fedramp", "true")
	} else {
		headers.Del("x-openai-fedramp")
	}
}

// The local baseline has no credential-shadow account type, so the account
// selected for Live is already the credential-bearing account.
func resolveAndSetOpenAIChatGPTAccountHeaders(_ context.Context, _ AccountRepository, headers http.Header, account *Account) error {
	setOpenAIChatGPTAccountHeaders(headers, account)
	return nil
}
