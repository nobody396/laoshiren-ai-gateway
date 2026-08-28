package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUserEmailAliasDedupIndexMigration(t *testing.T) {
	content, err := FS.ReadFile("224_users_email_alias_dedup_index_notx.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(content))
	require.Contains(t, sql, "create unique index concurrently if not exists idx_users_email_alias_dedup")
	require.Contains(t, sql, "gmail.com")
	require.Contains(t, sql, "googlemail.com")
	require.Contains(t, sql, "strpos(split_part(lower(btrim(email)), '@', 1), '+') > 1")
	require.Contains(t, sql, "where deleted_at is null")
}
