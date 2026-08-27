package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTeamMigrationContainsLocalIsolationAndAttributionContracts(t *testing.T) {
	content, err := FS.ReadFile("219_add_teams.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(content))

	require.Contains(t, sql, "team_memberships_active_user_uq")
	require.Contains(t, sql, "team_memberships_active_owner_uq")
	require.Contains(t, sql, "api_keys_team_id_fkey")
	require.Contains(t, sql, "usage_logs_billing_user_id_fkey")
	require.Contains(t, sql, "usage_logs_actor_user_id_fkey")
	require.Contains(t, sql, "usage_logs_team_id_fkey")
	require.Contains(t, sql, "new.actor_user_id")
	require.Contains(t, sql, "apply_team_member_usage")
	// 本地 Reliability/Customer Tier 继续读取 user_id，因此迁移不能把它改成成员。
	require.NotContains(t, sql, "new.user_id :=")
	require.NotContains(t, sql, "batch_image_jobs")

	require.Contains(t, sql, "prevent_active_team_owner_deletion")
	require.Contains(t, sql, "users_prevent_active_team_owner_soft_delete")
	require.Contains(t, sql, "users_prevent_active_team_owner_hard_delete")
}

func TestTeamDefaultMemberLimitsMigration(t *testing.T) {
	content, err := FS.ReadFile("220_add_team_default_member_limits.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(content))
	require.Contains(t, sql, "default_daily_limit_usd")
	require.Contains(t, sql, "default_weekly_limit_usd")
	require.Contains(t, sql, "default_monthly_limit_usd")
	require.Contains(t, sql, "teams_default_member_limits_check")
}

func TestTeamLifecycleMigration(t *testing.T) {
	content, err := FS.ReadFile("221_harden_team_lifecycle_and_allowance.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(content))
	require.Contains(t, sql, "team_owner_disabled")
	require.Contains(t, sql, "team_owner_transfer_required")
	require.Contains(t, sql, "update team_memberships")
	require.Contains(t, sql, "update api_keys")
	require.NotContains(t, sql, "batch_image_jobs")
}

func TestTeamRBACMigrationRegistersMenuAndAllAdminRoutes(t *testing.T) {
	content, err := FS.ReadFile("222_grant_team_rbac.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(content))
	require.Contains(t, sql, "admin:teams")
	for _, route := range []string{
		"/admin/teams", "/admin/teams/:id", "/admin/teams/:id/members",
		"/admin/teams/:id/usage", "/admin/teams/:id/force-transfer",
	} {
		require.Contains(t, sql, route)
	}
	require.Contains(t, sql, "role.is_super_admin=true")
}
