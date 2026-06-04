package auth

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"

	"github.com/gin-gonic/gin"
)

// GenerateCSRFToken 生成 CSRF token。
//
// 使用 crypto/rand 生成 32 字节随机数，base64 URL 安全编码。
func GenerateCSRFToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// CSRFConfig 是 CSRF 中间件的配置。
type CSRFConfig struct {
	// 是否启用 CSRF 保护
	Enabled bool

	// 读取 token 的 Header 名称
	HeaderName string

	// 读取 token 的表单字段名称
	FieldName string

	// 获取 token 的函数（从 Session 或其他存储中）
	GetToken func(c *gin.Context) string

	// 保存 token 的函数
	SetToken func(c *gin.Context, token string)
}

// CSRFMiddleware 返回 CSRF 校验中间件。
//
// 规则：
//   - GET / HEAD / OPTIONS 不校验
//   - POST / PUT / PATCH / DELETE 需要校验
//   - token 从 Header 或 Form 字段读取
//   - 校验失败返回 403
func CSRFMiddleware(cfg CSRFConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !cfg.Enabled {
			c.Next()
			return
		}

		// 安全方法不需要校验
		switch c.Request.Method {
		case "GET", "HEAD", "OPTIONS":
			c.Next()
			return
		}

		// 从 Header 或 Form 读取 token
		token := c.GetHeader(cfg.HeaderName)
		if token == "" {
			token = c.PostForm(cfg.FieldName)
		}

		// 获取预期的 token
		expected := cfg.GetToken(c)

		if expected == "" || token != expected {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "CSRF token 无效",
			})
			return
		}

		c.Next()
	}
}
