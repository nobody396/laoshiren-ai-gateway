package middleware

import (
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// AgentOrAdmin 合伙人或管理员权限中间件
// 允许 role=agent 或 role=admin 的用户访问
// 必须在 JWTAuth 中间件之后使用
func AgentOrAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, ok := GetUserRoleFromContext(c)
		if !ok {
			AbortWithError(c, 401, "UNAUTHORIZED", "User not found in context")
			return
		}

		if role != service.RoleAgent && role != service.RoleAdmin {
			AbortWithError(c, 403, "FORBIDDEN", "合伙人或管理员权限不足")
			return
		}

		c.Next()
	}
}
