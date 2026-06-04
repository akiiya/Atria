package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequireAuth 返回认证中间件。
//
// 功能：
//   - 从 Cookie 读取 token
//   - 解密并校验过期时间
//   - 成功后将 AdminID、Username、CurrentCredentialID 写入 gin.Context
//   - 失败重定向到 /login
func RequireAuth(key []byte, cookieName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(cookieName)
		if err != nil || token == "" {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		claims, err := DecodeSessionToken(key, token)
		if err != nil {
			// 清除无效 Cookie
			c.SetCookie(cookieName, "", -1, "/", "", false, true)
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		// 将用户信息写入上下文
		c.Set(ContextKeyAdminID, claims.AdminID)
		c.Set(ContextKeyUsername, claims.Username)
		c.Set(ContextKeyCredentialID, claims.CurrentCredentialID)

		c.Next()
	}
}

// GetAdminID 从 gin.Context 获取当前管理员 ID。
func GetAdminID(c *gin.Context) uint {
	if id, exists := c.Get(ContextKeyAdminID); exists {
		if adminID, ok := id.(uint); ok {
			return adminID
		}
	}
	return 0
}

// GetUsername 从 gin.Context 获取当前管理员用户名。
func GetUsername(c *gin.Context) string {
	if username, exists := c.Get(ContextKeyUsername); exists {
		if name, ok := username.(string); ok {
			return name
		}
	}
	return ""
}

// GetCredentialID 从 gin.Context 获取当前凭据 ID。
func GetCredentialID(c *gin.Context) uint {
	if id, exists := c.Get(ContextKeyCredentialID); exists {
		if credID, ok := id.(uint); ok {
			return credID
		}
	}
	return 0
}
