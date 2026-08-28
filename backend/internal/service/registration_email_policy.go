package service

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// NormalizeEmailForAliasDedup returns the mailbox identity used only for
// duplicate detection. Persisted/login/send addresses remain unchanged.
// Local policy intentionally folds aliases only for the Gmail family, whose
// dot and plus semantics are explicit and stable.
func NormalizeEmailForAliasDedup(email string) string {
	if normalized := NormalizeRegistrationEmailAddress(email); normalized != "" {
		return normalized
	}
	return strings.ToLower(strings.TrimSpace(email))
}

type emailAliasLookupRepository interface {
	ExistsByEmailAlias(ctx context.Context, email string, excludeUserID int64) (bool, error)
}

func emailAliasExistsForAnotherUser(ctx context.Context, repo UserRepository, email string, excludeUserID int64) (bool, error) {
	lookup, ok := repo.(emailAliasLookupRepository)
	if !ok {
		return false, nil
	}
	return lookup.ExistsByEmailAlias(ctx, email, excludeUserID)
}

var registrationEmailDomainPattern = regexp.MustCompile(
	`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`,
)

// RegistrationEmailSuffix extracts normalized suffix in "@domain" form.
func RegistrationEmailSuffix(email string) string {
	_, domain, ok := splitEmailForPolicy(email)
	if !ok {
		return ""
	}
	return "@" + domain
}

// NormalizeRegistrationEmailAddress 返回邀请唯一性检查使用的收件箱身份。
// 只对明确支持的 Gmail 语义折叠点号和 plus alias，其他域保持原样。
func NormalizeRegistrationEmailAddress(email string) string {
	local, domain, ok := splitEmailForPolicy(email)
	if !ok {
		return ""
	}
	domain = strings.TrimRight(domain, ".")
	if domain == "gmail.com" || domain == "googlemail.com" {
		if plusIndex := strings.IndexByte(local, '+'); plusIndex > 0 {
			local = local[:plusIndex]
		}
		if dotStripped := strings.ReplaceAll(local, ".", ""); dotStripped != "" {
			local = dotStripped
		}
		domain = "gmail.com"
	}
	if local == "" || domain == "" {
		return ""
	}
	return local + "@" + domain
}

// IsRegistrationEmailSuffixAllowed checks whether an email is allowed by suffix whitelist.
// Empty whitelist means allow all.
func IsRegistrationEmailSuffixAllowed(email string, whitelist []string) bool {
	if len(whitelist) == 0 {
		return true
	}
	suffix := RegistrationEmailSuffix(email)
	if suffix == "" {
		return false
	}
	for _, allowed := range whitelist {
		if suffix == allowed {
			return true
		}
	}
	return false
}

// NormalizeRegistrationEmailSuffixWhitelist normalizes and validates suffix whitelist items.
func NormalizeRegistrationEmailSuffixWhitelist(raw []string) ([]string, error) {
	return normalizeRegistrationEmailSuffixWhitelist(raw, true)
}

// ParseRegistrationEmailSuffixWhitelist parses persisted JSON into normalized suffixes.
// Invalid entries are ignored to keep old misconfigurations from breaking runtime reads.
func ParseRegistrationEmailSuffixWhitelist(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	var items []string
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return []string{}
	}
	normalized, _ := normalizeRegistrationEmailSuffixWhitelist(items, false)
	if len(normalized) == 0 {
		return []string{}
	}
	return normalized
}

func normalizeRegistrationEmailSuffixWhitelist(raw []string, strict bool) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		normalized, err := normalizeRegistrationEmailSuffix(item)
		if err != nil {
			if strict {
				return nil, err
			}
			continue
		}
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}

	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func normalizeRegistrationEmailSuffix(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return "", nil
	}

	domain := value
	if strings.Contains(value, "@") {
		if !strings.HasPrefix(value, "@") || strings.Count(value, "@") != 1 {
			return "", fmt.Errorf("invalid email suffix: %q", raw)
		}
		domain = strings.TrimPrefix(value, "@")
	}

	if domain == "" || strings.Contains(domain, "@") || !registrationEmailDomainPattern.MatchString(domain) {
		return "", fmt.Errorf("invalid email suffix: %q", raw)
	}

	return "@" + domain, nil
}

func splitEmailForPolicy(raw string) (local string, domain string, ok bool) {
	email := strings.ToLower(strings.TrimSpace(raw))
	local, domain, found := strings.Cut(email, "@")
	if !found || local == "" || domain == "" || strings.Contains(domain, "@") {
		return "", "", false
	}
	return local, domain, true
}
