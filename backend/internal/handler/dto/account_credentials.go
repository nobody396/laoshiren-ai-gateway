package dto

import "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/accountcredentials"

// RedactAccountCredentials returns a response-safe copy of account credentials.
// Runtime and persistence paths must continue to use service.Account.Credentials.
func RedactAccountCredentials(credentials map[string]any) map[string]any {
	return accountcredentials.RedactForResponse(credentials)
}
