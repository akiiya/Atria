package server

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// isHTMLRequest 判断请求是否期望 HTML 响应。
func isHTMLRequest(c *gin.Context) bool {
	accept := c.GetHeader("Accept")
	return strings.Contains(accept, "text/html")
}

// RenderError 返回错误页面或 JSON 错误响应。
//
// HTML 请求返回错误页面；API 请求返回 JSON。
// 不暴露内部错误堆栈。
func RenderError(c *gin.Context, status int, title string, message string) {
	if isHTMLRequest(c) {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.Status(status)
		// 使用内联模板渲染错误页面，避免依赖预解析的模板
		c.Data(status, "text/html; charset=utf-8", []byte(errorPageHTML(status, title, message)))
		return
	}

	c.JSON(status, gin.H{
		"error":  title,
		"status": status,
	})
}

// JSONError 返回统一的 JSON 错误响应。
func JSONError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{
		"error":  message,
		"status": status,
	})
}

// LogAndError 记录内部错误日志并返回用户友好的错误响应。
//
// 内部错误记录到日志；用户只看到通用错误消息。
func LogAndError(c *gin.Context, status int, logMsg string, err error, userMsg string) {
	slog.Error(logMsg, "error", err, "path", c.Request.URL.Path)
	RenderError(c, status, "错误", userMsg)
}

// escapeHTML 对用户可见内容做 HTML 实体转义，防止 XSS。
func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}

// errorPageHTML 生成简单的错误页面 HTML。
func errorPageHTML(status int, title string, message string) string {
	safeTitle := escapeHTML(title)
	safeMessage := escapeHTML(message)
	return `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>` + safeTitle + ` - Atria</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; display: flex; align-items: center; justify-content: center; min-height: 100vh; margin: 0; background: #f5f5f5; color: #333; }
        .error-card { text-align: center; padding: 48px; background: #fff; border-radius: 8px; box-shadow: 0 2px 8px rgba(0,0,0,0.1); }
        .error-code { font-size: 4rem; font-weight: 700; color: #2563eb; margin-bottom: 8px; }
        .error-title { font-size: 1.5rem; font-weight: 600; margin-bottom: 8px; }
        .error-desc { color: #666; margin-bottom: 24px; }
        a { color: #2563eb; text-decoration: none; }
        a:hover { text-decoration: underline; }
    </style>
</head>
<body>
    <div class="error-card">
        <div class="error-code">` + itoa(status) + `</div>
        <div class="error-title">` + safeTitle + `</div>
        <div class="error-desc">` + safeMessage + `</div>
        <a href="/">返回首页</a>
    </div>
</body>
</html>`
}

// itoa 简单的整数转字符串。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

// StatusText 返回 HTTP 状态码的中文描述。
func StatusText(status int) string {
	switch status {
	case http.StatusForbidden:
		return "访问被拒绝"
	case http.StatusNotFound:
		return "页面未找到"
	case http.StatusInternalServerError:
		return "服务器内部错误"
	case http.StatusUnauthorized:
		return "未认证"
	case http.StatusBadRequest:
		return "请求无效"
	default:
		return http.StatusText(status)
	}
}
