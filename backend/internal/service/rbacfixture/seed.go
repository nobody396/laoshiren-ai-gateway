package rbacfixture

import (
	"context"
	"database/sql"
	"fmt"
)

// Execer 抽象 SQL 执行能力, 兼容 *sql.DB / *sql.Tx / *sql.Conn.
// 调用方自行负责事务的 Begin/Commit/Rollback.
type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// upsertMenuSQL 按 permission_key 做 UPSERT, 幂等.
// admin_menus.permission_key 是 UNIQUE 约束(见 migration 118), 因此可作冲突键.
// 20260505 扁平化 (migration 120): 移除 parent_id 列.
const upsertMenuSQL = `
INSERT INTO admin_menus
    (id, name, type, path, component, icon,
     permission_key, sort_order, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (permission_key) DO UPDATE SET
    name        = EXCLUDED.name,
    type        = EXCLUDED.type,
    path        = EXCLUDED.path,
    component   = EXCLUDED.component,
    icon        = EXCLUDED.icon,
    sort_order  = EXCLUDED.sort_order,
    status      = EXCLUDED.status,
    updated_at  = now()
`

// upsertAPISQL 按 (method, path) 做 UPSERT, 幂等.
// admin_apis 在 (method, path) 上有唯一索引(见 migration 118).
const upsertAPISQL = `
INSERT INTO admin_apis
    (id, "group", path, method, description, sort_order, status)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (method, path) DO UPDATE SET
    "group"      = EXCLUDED."group",
    description  = EXCLUDED.description,
    sort_order   = EXCLUDED.sort_order,
    status       = EXCLUDED.status,
    updated_at   = now()
`

// 重置主键序列, 避免 fixture 写入的大 id 与自增冲突.
const resetMenuSequenceSQL = `
SELECT setval('admin_menus_id_seq',
    (SELECT COALESCE(MAX(id), 1) FROM admin_menus), true)
`

const resetAPISequenceSQL = `
SELECT setval('admin_apis_id_seq',
    (SELECT COALESCE(MAX(id), 1) FROM admin_apis), true)
`

// SeedMenus 把基线菜单幂等写入 admin_menus 表.
//
// 20260505 扁平化后 admin_menus 不再有 parent_id 自引用约束，无顺序依赖.
func SeedMenus(ctx context.Context, exec Execer) error {
	for _, m := range BaselineMenus() {
		if _, err := exec.ExecContext(ctx, upsertMenuSQL,
			m.ID, m.Name, m.Type, m.Path, m.Component, m.Icon,
			m.PermissionKey, m.SortOrder, m.Status,
		); err != nil {
			return fmt.Errorf("seed menu %q: %w", m.PermissionKey, err)
		}
	}
	return nil
}

// SeedAPIs 把基线 API 资源幂等写入 admin_apis 表.
func SeedAPIs(ctx context.Context, exec Execer) error {
	for _, a := range BaselineAPIs() {
		if _, err := exec.ExecContext(ctx, upsertAPISQL,
			a.ID, a.Group, a.Path, a.Method, a.Description, a.SortOrder, a.Status,
		); err != nil {
			return fmt.Errorf("seed api %s %s: %w", a.Method, a.Path, err)
		}
	}
	return nil
}

// SeedAll 顺序写入菜单 + API fixture. 适合集成测试一次性初始化.
func SeedAll(ctx context.Context, exec Execer) error {
	if err := SeedMenus(ctx, exec); err != nil {
		return err
	}
	if err := SeedAPIs(ctx, exec); err != nil {
		return err
	}
	return nil
}

// SeedAllAndResetSequence 在 [SeedAll] 之后重置主键序列.
// 推荐用于一次性导入脚本; 集成测试使用事务回滚隔离时一般无需调用.
func SeedAllAndResetSequence(ctx context.Context, exec Execer) error {
	if err := SeedAll(ctx, exec); err != nil {
		return err
	}
	if _, err := exec.ExecContext(ctx, resetMenuSequenceSQL); err != nil {
		return fmt.Errorf("reset admin_menus_id_seq: %w", err)
	}
	if _, err := exec.ExecContext(ctx, resetAPISequenceSQL); err != nil {
		return fmt.Errorf("reset admin_apis_id_seq: %w", err)
	}
	return nil
}
