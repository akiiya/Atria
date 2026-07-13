package server

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// contextKeyTLS 是 gin.Context 中 TLS 检测结果的 key。
const contextKeyTLS = "_atria_is_tls"

// tlsDetectMiddleware 检测反向代理 TLS header，自动设置 cookie Secure。
// 如果配置中 CookieSecure 已为 true，则始终使用 Secure。
// 如果配置中 CookieSecure 为 false，则检测 X-Forwarded-Proto: https。
func (s *Server) tlsDetectMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.cfg.CookieSecure {
			// 检测反向代理 TLS header
			proto := c.GetHeader("X-Forwarded-Proto")
			if strings.EqualFold(proto, "https") {
				c.Set(contextKeyTLS, true)
			}
		}
		c.Next()
	}
}

// isTLSCookieSecure 返回当前请求是否应使用 cookie Secure 属性。
func (s *Server) isTLSCookieSecure(c *gin.Context) bool {
	if s.cfg.CookieSecure {
		return true
	}
	if tls, ok := c.Get(contextKeyTLS); ok {
		if secure, ok := tls.(bool); ok {
			return secure
		}
	}
	return false
}
