package server

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/handler/admin"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// adminPathPrefix 是管理员路由统一前缀, 扫描器据此过滤出需要同步的路由.
// admin_apis.path 存储剥离 /api/v1 后的业务路径, 如 "/admin/users/:id",
// 与 permission_key "api:METHOD:/admin/users/:id" 的命名约定保持一致.
const adminPathPrefix = "/api/v1/admin/"

// syncAdminAPIsTimeout 保护启动流程不被 DB 慢查询卡住.
const syncAdminAPIsTimeout = 10 * time.Second

// scannerGroupMap 将 /admin/<segments>... 映射为中文 group 名, 供管理后台展示.
//
// 与左侧菜单 (admin_menus.name) 对齐是硬要求: 凡是菜单中存在同名功能的 API,
// group 文案必须与菜单名称逐字一致, 避免运营在 "角色分配" 弹窗看到分组名和
// 侧边栏菜单对不上. 匹配优先级:
//  1. 两段前缀 "seg1/seg2" (如 "rbac/menus") 精细覆盖 RBAC 子模块;
//  2. 单段 "seg1" 覆盖绝大多数业务模块;
//  3. 均未命中则留空, 管理员可在 API 管理页手工补充.
//
// 启动同步仅在 "新值非空" 时覆盖 (RBACRepository.UpsertAPIs), 因此管理员的手工
// 命名若想稳定保留, 需确保映射表未对该路径预设值.
var scannerGroupMap = map[string]string{
	// 两段前缀 (RBAC 子模块与菜单 62/63 对齐, 单独归类).
	"rbac/menus":      "菜单管理",   // 菜单 id=62 name
	"rbac/user-menus": "菜单管理",   // 菜单 id=62 name
	"rbac/apis":       "API 管理", // 菜单 id=63 name

	// 单段 (与菜单 admin_menus.name 一一对齐, 条目顺序按菜单 sort_order).
	"dashboard":            "管理仪表盘", // 菜单 id=2
	"users":                "用户管理",  // 菜单 route=/admin/users
	"agents":               "合伙人管理", // 菜单 route=/admin/agents
	"accounts":             "账号管理",  // 菜单 id=21
	"groups":               "分组管理",  // 菜单 id=22
	"channels":             "渠道管理",  // 菜单 route=/admin/channels
	"suppliers":            "供应商考察", // 菜单 route=/admin/suppliers
	"subscriptions":        "订阅管理",  // 菜单 id=31
	"redeem-codes":         "卡密管理",  // 菜单 id=32 (菜单 route=/admin/redeem, API 路径 /admin/redeem-codes)
	"promo-codes":          "优惠码管理", // 菜单 id=33
	"topup":                "充值订单",  // 菜单 id=34 (API 路径 /admin/topup/orders)
	"invoice":              "开票管理",  // 菜单 id=35 (API 路径 /admin/invoice/requests)
	"announcements":        "公告管理",  // 菜单 id=41
	"changelog":            "更新日志管理",
	"feedbacks":            "反馈管理", // 菜单 id=42
	"finance-transactions": "财务记账", // 菜单 route=/admin/finance-transactions
	"proxies":              "代理管理", // 菜单 id=51
	"usage":                "使用记录", // 菜单 id=52
	"ops":                  "运维监控", // 菜单 id=53
	"settings":             "系统设置", // 菜单 id=54
	"rbac":                 "角色管理", // 菜单 id=61 (/admin/rbac/roles/*, /admin/rbac/menu, /admin/rbac/me/*, /admin/rbac/users/:id/roles)

	// 左侧菜单中无独立入口, 保留独立分组名仅便于运营识别, 未来若新增菜单需同步更新此处文案.
	"api-keys":                 "API Key",
	"openai":                   "OpenAI OAuth",
	"gemini":                   "Gemini OAuth",
	"antigravity":              "Antigravity OAuth",
	"backups":                  "数据备份",
	"data-management":          "数据管理",
	"user-attributes":          "用户属性",
	"tls-fingerprint-profiles": "TLS 指纹",
	"error-passthrough-rules":  "错误透传",
	"scheduled-test-plans":     "定时测试",
	"system":                   "系统",
	"totp":                     "二次验证",
	"balance-alert":            "余额预警",
}

// ScanAdminAPIs 从 Gin 引擎的路由表中抽取全部 /api/v1/admin/* 路由,
// 生成 admin_apis 资源条目列表.
//
// 路径转换: "/api/v1/admin/users/:id" → "/admin/users/:id"
// Group:    优先两段前缀 (如 "rbac/menus") 查 scannerGroupMap; 未命中回退单段 (如 "users");
//
//	两级都没命中则留空, 等待管理员到 API 管理页手工补齐. 与左侧菜单 (admin_menus.name) 对齐是硬要求.
//
// Description: 优先查 scannerDescMap 取中文简介; 未命中则回退 "<METHOD> <path>" 占位,
//
//	等待管理员到 API 管理页手工补齐或后续迭代补入映射表.
func ScanAdminAPIs(engine *gin.Engine) []*service.AdminAPI {
	if engine == nil {
		return nil
	}
	routes := engine.Routes()
	out := make([]*service.AdminAPI, 0, len(routes))
	seen := make(map[string]struct{}, len(routes))
	for _, r := range routes {
		if !strings.HasPrefix(r.Path, adminPathPrefix) {
			continue
		}
		// 剥离 /api/v1 前缀, 得到业务路径.
		bizPath := strings.TrimPrefix(r.Path, "/api/v1")
		method := strings.ToUpper(r.Method)

		key := method + " " + bizPath
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		segment := firstSegment(bizPath[len("/admin/"):])
		group := scannerGroupMap[firstTwoSegments(bizPath[len("/admin/"):])]
		if group == "" {
			group = scannerGroupMap[segment]
		}

		description := lookupScannerDesc(method, bizPath)
		if description == "" {
			// 映射表未命中: 保持旧的占位格式, 便于管理员识别并手工补写.
			description = method + " " + bizPath
		}

		out = append(out, &service.AdminAPI{
			Group:       group,
			Path:        bizPath,
			Method:      method,
			Description: description,
			Status:      service.ResourceStatusActive,
		})
	}
	return out
}

// firstSegment 返回 path 中第一个 '/' 之前的片段.
// 例: "users/:id/balance" → "users"; "rbac" → "rbac".
func firstSegment(p string) string {
	if i := strings.IndexByte(p, '/'); i >= 0 {
		return p[:i]
	}
	return p
}

// firstTwoSegments 返回 path 前两段, 用 '/' 连接; 不足两段返回空串.
// 例: "rbac/menus/:id" → "rbac/menus"; "users/:id" → "users/:id"; "users" → "".
// 专用于 scannerGroupMap 的精细分组查找 (如 rbac 子模块).
func firstTwoSegments(p string) string {
	first := strings.IndexByte(p, '/')
	if first < 0 {
		return ""
	}
	second := strings.IndexByte(p[first+1:], '/')
	if second < 0 {
		return p
	}
	return p[:first+1+second]
}

// SyncAdminAPIsOptions 控制启动同步的行为开关.
type SyncAdminAPIsOptions struct {
	// Prune 为 true 时, 将删除 DB 中本次路由扫描未出现的孤儿记录.
	// 默认 false (保守): 避免启动时误删管理员手工新增的 API 条目.
	// 启用前请确保路由扫描结果稳定, 或使用手动 /admin/rbac/apis/sync?prune=true 空跑一次验证.
	Prune bool
}

// SyncAdminAPIsOnStartup 在启动阶段扫描路由表并同步到 admin_apis 表.
//
// 行为:
//  1. 扫描 engine 的全部 /api/v1/admin/* 路由
//  2. 通过 rbacService.SyncAPIsWithOptions 幂等写入 (不存在则插入; 已存在仅当新值非空时覆盖 group/description)
//  3. 可选 prune: 若 opts[0].Prune=true, 删除本次未出现的孤儿记录 (admin_role_apis 会随外键 CASCADE 清理)
//  4. 将快照注入 handler/admin 包, 供管理员手动触发 /admin/rbac/apis/sync 时复用
//
// 本函数失败不应阻止启动, 因此只记录日志; 管理员可随时通过管理端 "同步 API" 按钮重试.
func SyncAdminAPIsOnStartup(engine *gin.Engine, rbacService *service.RBACService, opts ...SyncAdminAPIsOptions) {
	if engine == nil || rbacService == nil {
		return
	}
	var opt SyncAdminAPIsOptions
	if len(opts) > 0 {
		opt = opts[0]
	}
	items := ScanAdminAPIs(engine)
	if len(items) == 0 {
		slog.Warn("rbac: admin route scanner found no routes, skipping sync")
		return
	}
	// 注入快照供 manual sync 复用.
	admin.SetRouteSnapshot(items)

	ctx, cancel := context.WithTimeout(context.Background(), syncAdminAPIsTimeout)
	defer cancel()
	res, err := rbacService.SyncAPIsWithOptions(ctx, items, service.SyncAPIsOptions{Prune: opt.Prune})
	if err != nil {
		slog.Error("rbac: failed to sync admin APIs on startup",
			"err", err, "count", len(items), "prune", opt.Prune)
		return
	}
	slog.Info("rbac: synced admin APIs from route scanner",
		"count", res.Synced,
		"created", res.Created,
		"updated", res.Updated,
		"pruned", res.Pruned,
		"prune_enabled", opt.Prune)
}
