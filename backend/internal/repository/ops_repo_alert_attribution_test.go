package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpsRepositoryListErrorLogsIncludesSafeCallerAttribution(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &opsRepository{db: db}
	now := time.Date(2026, 9, 1, 2, 34, 45, 0, time.UTC)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM ops_error_logs e`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	rows := sqlmock.NewRows([]string{
		"id", "created_at", "error_phase", "error_type", "error_owner", "error_source", "severity",
		"effective_status", "client_status", "is_business_limited", "is_count_tokens", "platform", "model",
		"is_retryable", "retry_count", "resolved", "resolved_at", "resolved_by_user_id", "resolved_by_name",
		"resolved_retry_id", "client_request_id", "request_id", "error_message", "user_id", "user_email",
		"api_key_id", "api_key_name", "is_internal", "account_id", "account_name", "group_id", "group_name",
		"client_ip", "request_path", "stream", "inbound_endpoint", "upstream_endpoint", "requested_model",
		"upstream_model", "request_type",
	}).AddRow(
		int64(901), now, "routing", "api_error", "platform", "gateway", "warning",
		403, 403, false, false, "openai", "",
		false, 0, false, nil, nil, "",
		nil, "", "req-visible-1", "This group does not allow /v1/messages dispatch", int64(2), "231798222@qq.com",
		int64(128), "一键安装 · Codex", true, nil, "", int64(6), "GPT 标准线路",
		nil, "/v1/messages", false, "/v1/messages", "", "", "", nil,
	)
	mock.ExpectQuery(`LEFT JOIN api_keys k ON e.api_key_id = k.id`).
		WithArgs(20, 0).
		WillReturnRows(rows)

	result, err := repo.ListErrorLogs(context.Background(), &service.OpsErrorLogFilter{Page: 1, PageSize: 20})

	require.NoError(t, err)
	require.Len(t, result.Errors, 1)
	item := result.Errors[0]
	require.Equal(t, "231798222@qq.com", item.UserEmail)
	require.NotNil(t, item.APIKeyID)
	require.Equal(t, int64(128), *item.APIKeyID)
	require.Equal(t, "一键安装 · Codex", item.APIKeyName)
	require.True(t, item.IsInternal)
	require.Equal(t, "req-visible-1", item.RequestID)
	require.NoError(t, mock.ExpectationsWereMet())
}
