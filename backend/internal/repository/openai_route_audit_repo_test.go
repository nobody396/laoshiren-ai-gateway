package repository

import (
	"strings"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestBuildOpenAIRouteShadowWhereIncludesEveryAuditFilter(t *testing.T) {
	start := time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	groupID := int64(7)
	version := 3
	evaluated := true
	diverged := false
	emergency := true

	where, args := buildOpenAIRouteShadowWhere(&service.OpenAIRouteShadowDecisionFilter{
		StartTime:       &start,
		EndTime:         &end,
		GroupID:         &groupID,
		Model:           "gpt-5.6-sol",
		PolicyVersion:   &version,
		Reason:          "shadow_selected",
		RequestID:       "request-1",
		ClientRequestID: "client-1",
		Evaluated:       &evaluated,
		Diverged:        &diverged,
		Emergency:       &emergency,
	}, "d")

	require.Len(t, args, 11)
	for _, column := range []string{
		"d.created_at >=", "d.created_at <", "d.group_id =", "d.model =",
		"d.policy_version =", "d.reason =", "d.request_id =", "d.client_request_id =",
		"d.evaluated =", "d.diverged =", "d.emergency =",
	} {
		require.True(t, strings.Contains(where, column), "missing %s in %s", column, where)
	}
}

func TestNormalizeOpenAIRouteShadowFilterClampsPagination(t *testing.T) {
	filter := normalizeOpenAIRouteShadowFilter(&service.OpenAIRouteShadowDecisionFilter{Page: -1, PageSize: 500})
	require.Equal(t, 1, filter.Page)
	require.Equal(t, 200, filter.PageSize)
}
