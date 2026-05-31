// Package rbacfixture 提供 RBAC 测试 fixture (菜单 + API).
//
// 扁平化后的数据结构:
//   - BaselineMenus(): 20 条菜单 (type 固定为 menu，不再有父子层级)，与当前后台导航保持一致
//   - BaselineAPIs(): admin API 记录, 每条对应一个 admin 路由; group 由对应菜单名决定
//
// 本包为测试辅助包, 仅被测试引用.
package rbacfixture

import (
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

// newBaselineMenu 构造扁平菜单条目 (type 固定 menu).
func newBaselineMenu(id int64, name, path, icon, key string, sortOrder int) *service.AdminMenu {
	return &service.AdminMenu{
		ID:            id,
		Name:          name,
		Type:          service.MenuTypeMenu,
		Path:          path,
		Icon:          icon,
		PermissionKey: key,
		SortOrder:     sortOrder,
		Status:        service.ResourceStatusActive,
	}
}

// BaselineMenus 返回 20 条基线菜单.
//
// 20260505 扁平化: migration 120 已删除 directory、button 两种类型与 parent_id 字段。
// 保留原有子菜单 id (2/11/21-23/31-35/41-42/51-54/61-63)，新增当前项目特有菜单使用空出的 id。
func BaselineMenus() []*service.AdminMenu {
	return []*service.AdminMenu{
		// 仪表盘
		newBaselineMenu(2, "管理仪表盘", "/admin/dashboard", "dashboard", "admin:dashboard", 1),
		// 用户管理
		newBaselineMenu(11, "用户管理", "/admin/users", "users", "admin:users", 2),
		newBaselineMenu(12, "代理商管理", "/admin/agents", "agent", "admin:agents", 3),
		// 渠道管理
		newBaselineMenu(21, "账号管理", "/admin/accounts", "globe", "admin:accounts", 4),
		newBaselineMenu(22, "分组管理", "/admin/groups", "folder", "admin:groups", 5),
		newBaselineMenu(23, "渠道管理", "/admin/channels", "channel", "admin:channels", 6),
		// 财务管理
		newBaselineMenu(31, "订阅管理", "/admin/subscriptions", "credit-card", "admin:subscriptions", 7),
		newBaselineMenu(32, "卡密管理", "/admin/redeem", "ticket", "admin:redeem", 8),
		newBaselineMenu(33, "优惠码管理", "/admin/promo-codes", "gift", "admin:promo-codes", 9),
		newBaselineMenu(34, "充值订单", "/admin/topup-orders", "credit-card", "admin:topup-orders", 10),
		newBaselineMenu(35, "开票管理", "/admin/invoice-requests", "ticket", "admin:invoice-requests", 11),
		// 内容管理
		newBaselineMenu(41, "公告管理", "/admin/announcements", "bell", "admin:announcements", 12),
		newBaselineMenu(42, "反馈管理", "/admin/feedbacks", "feedback", "admin:feedbacks", 13),
		// 系统管理
		newBaselineMenu(51, "代理管理", "/admin/proxies", "server", "admin:proxies", 14),
		newBaselineMenu(52, "使用记录", "/admin/usage", "chart", "admin:usage", 15),
		newBaselineMenu(53, "运维监控", "/admin/ops", "chart", "admin:ops", 16),
		newBaselineMenu(54, "系统设置", "/admin/settings", "cog", "admin:settings", 17),
		// 权限管理
		newBaselineMenu(61, "角色管理", "/admin/roles", "shield", "admin:roles", 18),
		newBaselineMenu(62, "菜单管理", "/admin/menus", "menu", "admin:menus", 19),
		newBaselineMenu(63, "API 管理", "/admin/apis", "api", "admin:apis", 20),
	}
}

// menuGroupName 将菜单 id 映射为 API group 名称, 用于 admin_apis.group 字段.
var menuGroupName = map[int64]string{
	2:  "仪表盘",
	11: "用户管理",
	12: "代理商管理",
	21: "账号管理",
	22: "分组管理",
	23: "渠道管理",
	31: "订阅管理",
	32: "卡密管理",
	33: "优惠码管理",
	34: "充值订单",
	35: "开票管理",
	41: "公告管理",
	42: "反馈管理",
	51: "代理管理",
	52: "使用记录",
	53: "运维监控",
	54: "系统设置",
	61: "角色管理",
	62: "菜单管理",
	63: "API 管理",
}

// BaselineAPIs 按 routes/admin.go 的全部 admin 路由生成 API 资源.
// id 从 101 开始自增.
func BaselineAPIs() []*service.AdminAPI {
	out := make([]*service.AdminAPI, 0, 320)
	nextID := int64(100)
	add := func(method, path string, parentMenuID int64, sortOrder int) {
		nextID++
		group := menuGroupName[parentMenuID]
		out = append(out, &service.AdminAPI{
			ID:          nextID,
			Group:       group,
			Path:        path,
			Method:      method,
			Description: method + " " + path,
			SortOrder:   sortOrder,
			Status:      service.ResourceStatusActive,
		})
	}

	// ===== Dashboard (parent=2 管理仪表盘) =====
	add("GET", "/admin/dashboard/snapshot-v2", 2, 1)
	add("GET", "/admin/dashboard/stats", 2, 2)
	add("GET", "/admin/dashboard/realtime", 2, 3)
	add("GET", "/admin/dashboard/trend", 2, 4)
	add("GET", "/admin/dashboard/models", 2, 5)
	add("GET", "/admin/dashboard/groups", 2, 6)
	add("GET", "/admin/dashboard/api-keys-trend", 2, 7)
	add("GET", "/admin/dashboard/users-trend", 2, 8)
	add("GET", "/admin/dashboard/users-ranking", 2, 9)
	add("POST", "/admin/dashboard/users-usage", 2, 10)
	add("POST", "/admin/dashboard/api-keys-usage", 2, 11)
	add("GET", "/admin/dashboard/user-breakdown", 2, 12)
	add("POST", "/admin/dashboard/aggregation/backfill", 2, 13)

	// ===== User Management (parent=11 用户列表) =====
	add("GET", "/admin/users", 11, 1)
	add("GET", "/admin/users/:id", 11, 2)
	add("POST", "/admin/users", 11, 3)
	add("PUT", "/admin/users/:id", 11, 4)
	add("DELETE", "/admin/users/:id", 11, 5)
	add("POST", "/admin/users/:id/balance", 11, 6)
	add("GET", "/admin/users/:id/api-keys", 11, 7)
	add("GET", "/admin/users/:id/usage", 11, 8)
	add("GET", "/admin/users/:id/balance-history", 11, 9)
	add("POST", "/admin/users/:id/replace-group", 11, 10)
	add("GET", "/admin/users/:id/attributes", 11, 11)
	add("PUT", "/admin/users/:id/attributes", 11, 12)

	// ===== Agent Management (parent=12 代理商管理) =====
	add("GET", "/admin/agents", 12, 1)
	add("GET", "/admin/agents/rates", 12, 2)
	add("PUT", "/admin/agents/rates", 12, 3)
	add("GET", "/admin/agents/:id", 12, 4)
	add("GET", "/admin/agents/:id/users", 12, 5)
	add("POST", "/admin/agents/:id/users/bind", 12, 6)
	add("GET", "/admin/agents/:id/commissions", 12, 7)
	add("GET", "/admin/agents/:id/settlements", 12, 8)
	add("POST", "/admin/agents/:id/settlements", 12, 9)
	add("GET", "/admin/agents/:id/rate", 12, 10)
	add("PUT", "/admin/agents/:id/rate", 12, 11)

	// ===== Groups (parent=22 分组管理) =====
	add("GET", "/admin/groups", 22, 1)
	add("GET", "/admin/groups/all", 22, 2)
	add("GET", "/admin/groups/usage-summary", 22, 3)
	add("GET", "/admin/groups/capacity-summary", 22, 4)
	add("PUT", "/admin/groups/sort-order", 22, 5)
	add("GET", "/admin/groups/:id", 22, 6)
	add("POST", "/admin/groups", 22, 7)
	add("PUT", "/admin/groups/:id", 22, 8)
	add("DELETE", "/admin/groups/:id", 22, 9)
	add("GET", "/admin/groups/:id/stats", 22, 10)
	add("GET", "/admin/groups/:id/rate-multipliers", 22, 11)
	add("PUT", "/admin/groups/:id/rate-multipliers", 22, 12)
	add("DELETE", "/admin/groups/:id/rate-multipliers", 22, 13)
	add("GET", "/admin/groups/:id/api-keys", 22, 14)

	// ===== Accounts (parent=21 账号管理) =====
	add("GET", "/admin/accounts", 21, 1)
	add("GET", "/admin/accounts/:id", 21, 2)
	add("POST", "/admin/accounts", 21, 3)
	add("POST", "/admin/accounts/check-mixed-channel", 21, 4)
	add("POST", "/admin/accounts/sync/crs", 21, 5)
	add("POST", "/admin/accounts/sync/crs/preview", 21, 6)
	add("PUT", "/admin/accounts/:id", 21, 7)
	add("DELETE", "/admin/accounts/:id", 21, 8)
	add("POST", "/admin/accounts/:id/test", 21, 9)
	add("POST", "/admin/accounts/:id/recover-state", 21, 10)
	add("POST", "/admin/accounts/:id/refresh", 21, 11)
	add("POST", "/admin/accounts/:id/set-privacy", 21, 12)
	add("POST", "/admin/accounts/:id/refresh-tier", 21, 13)
	add("GET", "/admin/accounts/:id/stats", 21, 14)
	add("POST", "/admin/accounts/:id/clear-error", 21, 15)
	add("GET", "/admin/accounts/:id/usage", 21, 16)
	add("GET", "/admin/accounts/:id/today-stats", 21, 17)
	add("POST", "/admin/accounts/today-stats/batch", 21, 18)
	add("POST", "/admin/accounts/:id/clear-rate-limit", 21, 19)
	add("POST", "/admin/accounts/:id/reset-quota", 21, 20)
	add("GET", "/admin/accounts/:id/temp-unschedulable", 21, 21)
	add("DELETE", "/admin/accounts/:id/temp-unschedulable", 21, 22)
	add("POST", "/admin/accounts/:id/schedulable", 21, 23)
	add("GET", "/admin/accounts/:id/models", 21, 24)
	add("POST", "/admin/accounts/batch", 21, 25)
	add("GET", "/admin/accounts/data", 21, 26)
	add("POST", "/admin/accounts/data", 21, 27)
	add("POST", "/admin/accounts/batch-update-credentials", 21, 28)
	add("POST", "/admin/accounts/batch-refresh-tier", 21, 29)
	add("POST", "/admin/accounts/bulk-update", 21, 30)
	add("POST", "/admin/accounts/batch-clear-error", 21, 31)
	add("POST", "/admin/accounts/batch-refresh", 21, 32)
	add("GET", "/admin/accounts/antigravity/default-model-mapping", 21, 33)
	add("POST", "/admin/accounts/generate-auth-url", 21, 34)
	add("POST", "/admin/accounts/generate-setup-token-url", 21, 35)
	add("POST", "/admin/accounts/exchange-code", 21, 36)
	add("POST", "/admin/accounts/exchange-setup-token-code", 21, 37)
	add("POST", "/admin/accounts/cookie-auth", 21, 38)
	add("POST", "/admin/accounts/setup-token-cookie-auth", 21, 39)

	// ===== Announcements (parent=41) =====
	add("GET", "/admin/announcements", 41, 1)
	add("POST", "/admin/announcements", 41, 2)
	add("GET", "/admin/announcements/:id", 41, 3)
	add("PUT", "/admin/announcements/:id", 41, 4)
	add("DELETE", "/admin/announcements/:id", 41, 5)
	add("GET", "/admin/announcements/:id/read-status", 41, 6)

	// ===== Feedbacks (parent=42) =====
	add("GET", "/admin/feedbacks", 42, 1)
	add("GET", "/admin/feedbacks/:id", 42, 2)
	add("POST", "/admin/feedbacks/:id/replies", 42, 3)
	add("PUT", "/admin/feedbacks/:id/status", 42, 4)
	add("PUT", "/admin/feedbacks/:id/priority", 42, 5)
	add("PUT", "/admin/feedbacks/batch-status", 42, 6)
	add("DELETE", "/admin/feedbacks/:id", 42, 7)
	add("POST", "/admin/feedbacks/batch-delete", 42, 8)

	// ===== OpenAI OAuth (parent=21) =====
	add("POST", "/admin/openai/generate-auth-url", 21, 40)
	add("POST", "/admin/openai/exchange-code", 21, 41)
	add("POST", "/admin/openai/refresh-token", 21, 42)
	add("POST", "/admin/openai/accounts/:id/refresh", 21, 43)
	add("POST", "/admin/openai/create-from-oauth", 21, 44)

	// ===== Gemini OAuth (parent=21) =====
	add("POST", "/admin/gemini/oauth/auth-url", 21, 45)
	add("POST", "/admin/gemini/oauth/exchange-code", 21, 46)
	add("GET", "/admin/gemini/oauth/capabilities", 21, 47)

	// ===== Antigravity OAuth (parent=21) =====
	add("POST", "/admin/antigravity/oauth/auth-url", 21, 48)
	add("POST", "/admin/antigravity/oauth/exchange-code", 21, 49)
	add("POST", "/admin/antigravity/oauth/refresh-token", 21, 50)

	// ===== Proxies (parent=51 代理管理) =====
	add("GET", "/admin/proxies", 51, 1)
	add("GET", "/admin/proxies/all", 51, 2)
	add("GET", "/admin/proxies/data", 51, 3)
	add("POST", "/admin/proxies/data", 51, 4)
	add("GET", "/admin/proxies/:id", 51, 5)
	add("POST", "/admin/proxies", 51, 6)
	add("PUT", "/admin/proxies/:id", 51, 7)
	add("DELETE", "/admin/proxies/:id", 51, 8)
	add("POST", "/admin/proxies/:id/test", 51, 9)
	add("POST", "/admin/proxies/:id/quality-check", 51, 10)
	add("GET", "/admin/proxies/:id/stats", 51, 11)
	add("GET", "/admin/proxies/:id/accounts", 51, 12)
	add("POST", "/admin/proxies/batch-delete", 51, 13)
	add("POST", "/admin/proxies/batch", 51, 14)

	// ===== RedeemCodes (parent=32 卡密管理) =====
	add("GET", "/admin/redeem-codes", 32, 1)
	add("GET", "/admin/redeem-codes/stats", 32, 2)
	add("GET", "/admin/redeem-codes/export", 32, 3)
	add("GET", "/admin/redeem-codes/billing", 32, 4)
	add("GET", "/admin/redeem-codes/:id", 32, 5)
	add("POST", "/admin/redeem-codes/create-and-redeem", 32, 6)
	add("POST", "/admin/redeem-codes/generate", 32, 7)
	add("DELETE", "/admin/redeem-codes/:id", 32, 8)
	add("POST", "/admin/redeem-codes/batch-delete", 32, 9)
	add("POST", "/admin/redeem-codes/:id/expire", 32, 10)

	// ===== PromoCodes (parent=33) =====
	add("GET", "/admin/promo-codes", 33, 1)
	add("GET", "/admin/promo-codes/:id", 33, 2)
	add("POST", "/admin/promo-codes", 33, 3)
	add("PUT", "/admin/promo-codes/:id", 33, 4)
	add("DELETE", "/admin/promo-codes/:id", 33, 5)
	add("GET", "/admin/promo-codes/:id/usages", 33, 6)

	// ===== Settings (parent=54 系统设置) =====
	add("GET", "/admin/settings", 54, 1)
	add("PUT", "/admin/settings", 54, 2)
	add("POST", "/admin/settings/test-smtp", 54, 3)
	add("POST", "/admin/settings/send-test-email", 54, 4)
	add("GET", "/admin/settings/admin-api-key", 54, 5)
	add("POST", "/admin/settings/admin-api-key/regenerate", 54, 6)
	add("DELETE", "/admin/settings/admin-api-key", 54, 7)
	add("GET", "/admin/settings/overload-cooldown", 54, 8)
	add("PUT", "/admin/settings/overload-cooldown", 54, 9)
	add("GET", "/admin/settings/stream-timeout", 54, 10)
	add("PUT", "/admin/settings/stream-timeout", 54, 11)
	add("GET", "/admin/settings/rectifier", 54, 12)
	add("PUT", "/admin/settings/rectifier", 54, 13)
	add("GET", "/admin/settings/beta-policy", 54, 14)
	add("PUT", "/admin/settings/beta-policy", 54, 15)
	add("GET", "/admin/settings/openai-fast-policy", 54, 16)
	add("PUT", "/admin/settings/openai-fast-policy", 54, 17)
	add("GET", "/admin/settings/web-search-emulation", 54, 18)
	add("PUT", "/admin/settings/web-search-emulation", 54, 19)
	add("POST", "/admin/settings/web-search-emulation/test", 54, 20)
	add("POST", "/admin/settings/web-search-emulation/reset-usage", 54, 21)

	// ===== DataManagement (parent=54) =====
	add("GET", "/admin/data-management/agent/health", 54, 20)
	add("GET", "/admin/data-management/config", 54, 21)
	add("PUT", "/admin/data-management/config", 54, 22)
	add("GET", "/admin/data-management/sources/:source_type/profiles", 54, 23)
	add("POST", "/admin/data-management/sources/:source_type/profiles", 54, 24)
	add("PUT", "/admin/data-management/sources/:source_type/profiles/:profile_id", 54, 25)
	add("DELETE", "/admin/data-management/sources/:source_type/profiles/:profile_id", 54, 26)
	add("POST", "/admin/data-management/sources/:source_type/profiles/:profile_id/activate", 54, 27)
	add("POST", "/admin/data-management/s3/test", 54, 28)
	add("GET", "/admin/data-management/s3/profiles", 54, 29)
	add("POST", "/admin/data-management/s3/profiles", 54, 30)
	add("PUT", "/admin/data-management/s3/profiles/:profile_id", 54, 31)
	add("DELETE", "/admin/data-management/s3/profiles/:profile_id", 54, 32)
	add("POST", "/admin/data-management/s3/profiles/:profile_id/activate", 54, 33)
	add("POST", "/admin/data-management/backups", 54, 34)
	add("GET", "/admin/data-management/backups", 54, 35)
	add("GET", "/admin/data-management/backups/:job_id", 54, 36)

	// ===== Backups (parent=54) =====
	add("GET", "/admin/backups/s3-config", 54, 40)
	add("PUT", "/admin/backups/s3-config", 54, 41)
	add("POST", "/admin/backups/s3-config/test", 54, 42)
	add("GET", "/admin/backups/schedule", 54, 43)
	add("PUT", "/admin/backups/schedule", 54, 44)
	add("POST", "/admin/backups", 54, 45)
	add("GET", "/admin/backups", 54, 46)
	add("GET", "/admin/backups/:id", 54, 47)
	add("DELETE", "/admin/backups/:id", 54, 48)
	add("GET", "/admin/backups/:id/download-url", 54, 49)
	add("POST", "/admin/backups/:id/restore", 54, 50)

	// ===== Ops (parent=53 运维监控) =====
	add("GET", "/admin/ops/concurrency", 53, 1)
	add("GET", "/admin/ops/user-concurrency", 53, 2)
	add("GET", "/admin/ops/account-availability", 53, 3)
	add("GET", "/admin/ops/realtime-traffic", 53, 4)
	add("GET", "/admin/ops/alert-rules", 53, 5)
	add("POST", "/admin/ops/alert-rules", 53, 6)
	add("PUT", "/admin/ops/alert-rules/:id", 53, 7)
	add("DELETE", "/admin/ops/alert-rules/:id", 53, 8)
	add("GET", "/admin/ops/alert-events", 53, 9)
	add("GET", "/admin/ops/alert-events/:id", 53, 10)
	add("PUT", "/admin/ops/alert-events/:id/status", 53, 11)
	add("POST", "/admin/ops/alert-silences", 53, 12)
	add("GET", "/admin/ops/email-notification/config", 53, 13)
	add("PUT", "/admin/ops/email-notification/config", 53, 14)
	add("GET", "/admin/ops/webhook-notification/config", 53, 15)
	add("PUT", "/admin/ops/webhook-notification/config", 53, 16)
	add("POST", "/admin/ops/webhook-notification/test", 53, 17)
	add("GET", "/admin/ops/runtime/alert", 53, 18)
	add("PUT", "/admin/ops/runtime/alert", 53, 19)
	add("GET", "/admin/ops/runtime/logging", 53, 20)
	add("PUT", "/admin/ops/runtime/logging", 53, 21)
	add("POST", "/admin/ops/runtime/logging/reset", 53, 22)
	add("GET", "/admin/ops/advanced-settings", 53, 23)
	add("PUT", "/admin/ops/advanced-settings", 53, 24)
	add("GET", "/admin/ops/settings/metric-thresholds", 53, 25)
	add("PUT", "/admin/ops/settings/metric-thresholds", 53, 26)
	add("GET", "/admin/ops/ws/qps", 53, 27)
	add("GET", "/admin/ops/errors", 53, 28)
	add("GET", "/admin/ops/errors/:id", 53, 29)
	add("GET", "/admin/ops/errors/:id/retries", 53, 30)
	add("POST", "/admin/ops/errors/:id/retry", 53, 31)
	add("PUT", "/admin/ops/errors/:id/resolve", 53, 32)
	add("GET", "/admin/ops/request-errors", 53, 33)
	add("GET", "/admin/ops/request-errors/:id", 53, 34)
	add("GET", "/admin/ops/request-errors/:id/upstream-errors", 53, 35)
	add("POST", "/admin/ops/request-errors/:id/retry-client", 53, 36)
	add("POST", "/admin/ops/request-errors/:id/upstream-errors/:idx/retry", 53, 37)
	add("PUT", "/admin/ops/request-errors/:id/resolve", 53, 38)
	add("GET", "/admin/ops/upstream-errors", 53, 39)
	add("GET", "/admin/ops/upstream-errors/:id", 53, 40)
	add("POST", "/admin/ops/upstream-errors/:id/retry", 53, 41)
	add("PUT", "/admin/ops/upstream-errors/:id/resolve", 53, 42)
	add("GET", "/admin/ops/requests", 53, 43)
	add("GET", "/admin/ops/system-logs", 53, 44)
	add("POST", "/admin/ops/system-logs/cleanup", 53, 45)
	add("GET", "/admin/ops/system-logs/health", 53, 46)
	add("GET", "/admin/ops/dashboard/snapshot-v2", 53, 47)
	add("GET", "/admin/ops/dashboard/overview", 53, 48)
	add("GET", "/admin/ops/dashboard/throughput-trend", 53, 49)
	add("GET", "/admin/ops/dashboard/latency-histogram", 53, 50)
	add("GET", "/admin/ops/dashboard/error-trend", 53, 51)
	add("GET", "/admin/ops/dashboard/error-distribution", 53, 52)
	add("GET", "/admin/ops/dashboard/openai-token-stats", 53, 53)

	// ===== System (系统管理 - 扁平化后原 parent=50 目录已删, 这几条 group 为空) =====
	add("GET", "/admin/system/version", 50, 1)
	add("GET", "/admin/system/check-updates", 50, 2)
	add("POST", "/admin/system/update", 50, 3)
	add("POST", "/admin/system/rollback", 50, 4)
	add("POST", "/admin/system/restart", 50, 5)

	// ===== Subscriptions (parent=31 订阅管理) =====
	add("GET", "/admin/subscriptions", 31, 1)
	add("GET", "/admin/subscriptions/:id", 31, 2)
	add("GET", "/admin/subscriptions/:id/progress", 31, 3)
	add("POST", "/admin/subscriptions/assign", 31, 4)
	add("POST", "/admin/subscriptions/bulk-assign", 31, 5)
	add("POST", "/admin/subscriptions/:id/extend", 31, 6)
	add("POST", "/admin/subscriptions/:id/reset-quota", 31, 7)
	add("DELETE", "/admin/subscriptions/:id", 31, 8)
	add("GET", "/admin/groups/:id/subscriptions", 31, 9)
	add("GET", "/admin/users/:id/subscriptions", 31, 10)

	// ===== Usage (parent=52 使用记录) =====
	add("GET", "/admin/usage", 52, 1)
	add("GET", "/admin/usage/stats", 52, 2)
	add("GET", "/admin/usage/search-users", 52, 3)
	add("GET", "/admin/usage/search-api-keys", 52, 4)
	add("GET", "/admin/usage/cleanup-tasks", 52, 5)
	add("POST", "/admin/usage/cleanup-tasks", 52, 6)
	add("POST", "/admin/usage/cleanup-tasks/:id/cancel", 52, 7)

	// ===== UserAttribute (parent=11 用户列表) =====
	add("GET", "/admin/user-attributes", 11, 20)
	add("POST", "/admin/user-attributes", 11, 21)
	add("POST", "/admin/user-attributes/batch", 11, 22)
	add("PUT", "/admin/user-attributes/reorder", 11, 23)
	add("PUT", "/admin/user-attributes/:id", 11, 24)
	add("DELETE", "/admin/user-attributes/:id", 11, 25)

	// ===== ErrorPassthrough (parent=54) =====
	add("GET", "/admin/error-passthrough-rules", 54, 60)
	add("GET", "/admin/error-passthrough-rules/:id", 54, 61)
	add("POST", "/admin/error-passthrough-rules", 54, 62)
	add("PUT", "/admin/error-passthrough-rules/:id", 54, 63)
	add("DELETE", "/admin/error-passthrough-rules/:id", 54, 64)

	// ===== TLSFingerprintProfile (parent=54) =====
	add("GET", "/admin/tls-fingerprint-profiles", 54, 70)
	add("GET", "/admin/tls-fingerprint-profiles/:id", 54, 71)
	add("POST", "/admin/tls-fingerprint-profiles", 54, 72)
	add("PUT", "/admin/tls-fingerprint-profiles/:id", 54, 73)
	add("DELETE", "/admin/tls-fingerprint-profiles/:id", 54, 74)

	// ===== AdminAPIKey (parent=54) =====
	add("PUT", "/admin/api-keys/:id", 54, 80)

	// ===== ScheduledTest (parent=21 账号管理) =====
	add("POST", "/admin/scheduled-test-plans", 21, 60)
	add("PUT", "/admin/scheduled-test-plans/:id", 21, 61)
	add("DELETE", "/admin/scheduled-test-plans/:id", 21, 62)
	add("GET", "/admin/scheduled-test-plans/:id/results", 21, 63)
	add("GET", "/admin/accounts/:id/scheduled-test-plans", 21, 64)

	// ===== Channel (parent=23 渠道列表) =====
	add("GET", "/admin/channels", 23, 1)
	add("GET", "/admin/channels/model-pricing", 23, 2)
	add("GET", "/admin/channels/:id", 23, 3)
	add("POST", "/admin/channels", 23, 4)
	add("PUT", "/admin/channels/:id", 23, 5)
	add("DELETE", "/admin/channels/:id", 23, 6)

	// ===== Invoice (parent=34 充值订单 / 35 开票管理) =====
	add("GET", "/admin/topup/orders", 34, 1)
	add("GET", "/admin/invoice/requests", 35, 1)
	add("POST", "/admin/invoice/requests/export", 35, 2)
	add("POST", "/admin/invoice/requests/:id/complete", 35, 3)
	add("POST", "/admin/invoice/requests/:id/reject", 35, 4)

	// ===== RBAC (parent=61 角色管理 / 62 菜单管理 / 63 API 管理) =====
	add("GET", "/admin/rbac/menu", 61, 1)
	add("GET", "/admin/rbac/me/permissions", 61, 2)
	// 角色 CRUD + 关联
	add("GET", "/admin/rbac/roles", 61, 10)
	add("POST", "/admin/rbac/roles", 61, 11)
	add("PUT", "/admin/rbac/roles/:id", 61, 12)
	add("DELETE", "/admin/rbac/roles/:id", 61, 13)
	add("GET", "/admin/rbac/roles/:id/menus", 61, 14)
	add("PUT", "/admin/rbac/roles/:id/menus", 61, 15)
	add("GET", "/admin/rbac/roles/:id/apis", 61, 16)
	add("PUT", "/admin/rbac/roles/:id/apis", 61, 17)
	add("GET", "/admin/rbac/users/:id/roles", 61, 20)
	add("PUT", "/admin/rbac/users/:id/roles", 61, 21)
	// 菜单管理 CRUD
	add("GET", "/admin/rbac/menus", 62, 1)
	add("GET", "/admin/rbac/menus/tree", 62, 2)
	add("POST", "/admin/rbac/menus", 62, 3)
	add("PUT", "/admin/rbac/menus/:id", 62, 4)
	add("DELETE", "/admin/rbac/menus/:id", 62, 5)
	add("GET", "/admin/rbac/user-menus/visibility", 62, 6)
	add("PUT", "/admin/rbac/user-menus/visibility", 62, 7)
	// API 管理 CRUD
	add("GET", "/admin/rbac/apis", 63, 1)
	add("GET", "/admin/rbac/apis/all", 63, 2)
	add("GET", "/admin/rbac/apis/groups", 63, 3)
	add("POST", "/admin/rbac/apis", 63, 4)
	add("PUT", "/admin/rbac/apis/:id", 63, 5)
	add("DELETE", "/admin/rbac/apis/:id", 63, 6)
	add("DELETE", "/admin/rbac/apis", 63, 7)
	add("POST", "/admin/rbac/apis/sync", 63, 8)

	return out
}

// FullFixture 返回完整的菜单 + API fixture 数据, 供 seed 调用.
//
// 注意: 返回 2 个切片而非合并, 因为菜单与 API 写入不同表.
func FullFixture() (menus []*service.AdminMenu, apis []*service.AdminAPI) {
	return BaselineMenus(), BaselineAPIs()
}
