package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// adminAPIAuthBypass 是鉴权中间件的豁免白名单.
//
// 这些接口是"所有已登录管理员"的基础能力, 前端登录后必须首先拉取:
//   - /admin/rbac/menu           : 渲染侧边栏 (只返回角色允许的菜单, 本身即受限)
//   - /admin/rbac/me/permissions : 拿到自身权限 key 集合供前端按钮级判定
//
// 若将它们也纳入 RBAC 校验, 则新建角色首次登录时会直接 403, 形成死锁 —
// 因为用户此时还没有任何权限, 连"查自己有啥权限"都被拦住.
//
// key 为 "METHOD /admin/<bizPath>".
var adminAPIAuthBypass = map[string]struct{}{
	"GET /admin/rbac/menu":           {},
	"GET /admin/rbac/me/permissions": {},
}

// RequireAPIPermission 基于当前请求的 method + 路由模板动态校验 RBAC API 权限.
//
// 必须挂载在 adminAuth 之后, 期望上下文中存在:
//   - ContextKeyIsSuperAdmin    (bool)     : 超管直接放行
//   - ContextKeyUserPermissions ([]string) : 包含 "*" 或 "api:<METHOD>:<path>" 的权限 key 集合
//
// 路径约定与 route_sync.go 保持一致: admin_apis.path 存剥离 "/api/v1" 后的业务路径,
// 因此这里取 c.FullPath() 并剥离前缀后再拼键, 保证与启动同步结果可对齐.
func RequireAPIPermission() gin.HandlerFunc {
	return func(c *gin.Context) {
		// FullPath 返回的是 Gin 匹配到的路由模板, 如 "/api/v1/admin/users/:id".
		// 若请求未命中任何路由 (404), FullPath 为空, 交给 Gin NoRoute 处理.
		fullPath := c.FullPath()
		if fullPath == "" {
			c.Next()
			return
		}

		method := strings.ToUpper(c.Request.Method)
		bizPath := strings.TrimPrefix(fullPath, "/api/v1")

		// 白名单: 所有已登录管理员共享的基础能力.
		if _, ok := adminAPIAuthBypass[method+" "+bizPath]; ok {
			c.Next()
			return
		}

		// 超级管理员直接放行.
		if isSuper, exists := c.Get(string(ContextKeyIsSuperAdmin)); exists {
			if super, ok := isSuper.(bool); ok && super {
				c.Next()
				return
			}
		}

		perms, exists := c.Get(string(ContextKeyUserPermissions))
		if !exists {
			AbortWithError(c, 403, "FORBIDDEN", "No permissions found")
			return
		}
		permKeys, ok := perms.([]string)
		if !ok {
			AbortWithError(c, 403, "FORBIDDEN", "Invalid permissions")
			return
		}

		target := "api:" + method + ":" + bizPath
		for _, k := range permKeys {
			if k == "*" || k == target {
				c.Next()
				return
			}
		}

		AbortWithError(c, 403, "FORBIDDEN", "Insufficient API permissions")
	}
}

// RequirePermission 创建一个检查特定权限 key 的中间件
func RequirePermission(permissionKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 超级管理员直接放行
		if isSuper, exists := c.Get(string(ContextKeyIsSuperAdmin)); exists {
			if super, ok := isSuper.(bool); ok && super {
				c.Next()
				return
			}
		}

		// 检查权限列表
		perms, exists := c.Get(string(ContextKeyUserPermissions))
		if !exists {
			AbortWithError(c, 403, "FORBIDDEN", "No permissions found")
			return
		}

		permKeys, ok := perms.([]string)
		if !ok {
			AbortWithError(c, 403, "FORBIDDEN", "Invalid permissions")
			return
		}

		for _, k := range permKeys {
			if k == "*" || k == permissionKey {
				c.Next()
				return
			}
		}

		AbortWithError(c, 403, "FORBIDDEN", "Insufficient permissions")
	}
}

// GetPermissionsFromContext 从 gin.Context 获取用户权限列表
func GetPermissionsFromContext(c *gin.Context) ([]string, bool) {
	perms, exists := c.Get(string(ContextKeyUserPermissions))
	if !exists {
		return nil, false
	}
	permKeys, ok := perms.([]string)
	return permKeys, ok
}

// IsSuperAdminFromContext 从 gin.Context 获取是否超级管理员
func IsSuperAdminFromContext(c *gin.Context) bool {
	if isSuper, exists := c.Get(string(ContextKeyIsSuperAdmin)); exists {
		if super, ok := isSuper.(bool); ok && super {
			return true
		}
	}
	return false
}

// matchAPIPath 检查请求路径是否匹配 API 权限路径模式
// 支持精确匹配和通配符匹配 (如 /admin/users/*)
func matchAPIPath(pattern, actual string) bool {
	if pattern == actual {
		return true
	}
	// 通配符匹配: /admin/users/* 匹配 /admin/users/123
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		return strings.HasPrefix(actual, prefix+"/") || actual == prefix
	}
	return false
}
