package server

import (
	"bytes"
	"html/template"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// errorPageData holds the data passed to the error page template.
type errorPageData struct {
	Status  int
	Title   string
	Message string
}

// errorPageTmpl is the parsed HTML template for error pages.
// Compiled once at package init time; html/template auto-escapes all fields.
var errorPageTmpl = template.Must(template.New("error").Parse(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}} - Atria</title>
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
        <div class="error-code">{{.Status}}</div>
        <div class="error-title">{{.Title}}</div>
        <div class="error-desc">{{.Message}}</div>
        <a href="/">返回首页</a>
    </div>
</body>
</html>`))

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

// errorPageHTML 生成简单的错误页面 HTML。
func errorPageHTML(status int, title string, message string) string {
	var buf bytes.Buffer
	_ = errorPageTmpl.Execute(&buf, errorPageData{
		Status:  status,
		Title:   title,
		Message: message,
	})
	return buf.String()
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
