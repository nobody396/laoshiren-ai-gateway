//go:build unit

package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"strings"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type grokCredentialCASResult int64

func (r grokCredentialCASResult) LastInsertId() (int64, error) { return 0, nil }
func (r grokCredentialCASResult) RowsAffected() (int64, error) { return int64(r), nil }

type grokCredentialCASExecutor struct {
	queries []string
	args    [][]any
	result  driver.Result
}

func (e *grokCredentialCASExecutor) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	e.queries = append(e.queries, query)
	e.args = append(e.args, args)
	return e.result, nil
}

func (e *grokCredentialCASExecutor) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	return nil, nil
}

func normalizeGrokCredentialCASSQL(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func TestAccountRepositoryGrokCredentialCASMutationsPublishAtomicOutbox(t *testing.T) {
	proxyID := int64(29)
	snapshot := service.GrokCredentialMutationSnapshot{
		CredentialsJSON: `{"access_token":"old","refresh_token":"attempted","_token_version":9}`,
		ProxyID:         &proxyID,
	}

	t.Run("refresh success", func(t *testing.T) {
		exec := &grokCredentialCASExecutor{result: grokCredentialCASResult(1)}
		repo := newAccountRepositoryWithSQL(nil, exec, nil)
		applied, err := repo.UpdateGrokOAuthCredentialsIfUnchanged(
			context.Background(), 42,
			map[string]any{"refresh_token": "attempted", "_token_version": int64(9)},
			&proxyID,
			map[string]any{"refresh_token": "rotated", "_token_version": int64(10)},
		)
		require.NoError(t, err)
		require.True(t, applied)
		require.Len(t, exec.queries, 1)
		query := normalizeGrokCredentialCASSQL(exec.queries[0])
		require.Contains(t, query, "credentials = $5::jsonb")
		require.Contains(t, query, "proxy_id IS NOT DISTINCT FROM $6")
		require.Contains(t, query, "INSERT INTO scheduler_outbox")
		require.Len(t, exec.args[0], 7)
	})

	t.Run("permanent request failure", func(t *testing.T) {
		exec := &grokCredentialCASExecutor{result: grokCredentialCASResult(1)}
		repo := newAccountRepositoryWithSQL(nil, exec, nil)
		applied, err := repo.SetGrokCredentialErrorIfMatch(
			context.Background(), 42, snapshot, string(service.GrokCredentialReasonRevoked),
		)
		require.NoError(t, err)
		require.True(t, applied)
		query := normalizeGrokCredentialCASSQL(exec.queries[0])
		require.Contains(t, query, "a.credentials = $7::jsonb")
		require.Contains(t, query, "a.proxy_id IS NOT DISTINCT FROM $8")
		require.Contains(t, query, "INSERT INTO scheduler_outbox")
		require.Len(t, exec.args[0], 10)
	})

	t.Run("transient request failure", func(t *testing.T) {
		exec := &grokCredentialCASExecutor{result: grokCredentialCASResult(1)}
		repo := newAccountRepositoryWithSQL(nil, exec, nil)
		applied, err := repo.SetGrokCredentialTempUnschedulableIfMatch(
			context.Background(), 42, snapshot, time.Now().Add(time.Minute), "temporary",
		)
		require.NoError(t, err)
		require.True(t, applied)
		query := normalizeGrokCredentialCASSQL(exec.queries[0])
		require.Contains(t, query, "a.credentials = $7::jsonb")
		require.Contains(t, query, "a.proxy_id IS NOT DISTINCT FROM $8")
		require.Contains(t, query, "INSERT INTO scheduler_outbox")
		require.Len(t, exec.args[0], 9)
	})
}
