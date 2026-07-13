package server

import (
	"strings"
	"testing"
)

func TestRenderError(t *testing.T) {
	t.Run("basic output", func(t *testing.T) {
		html := errorPageHTML(404, "Page Not Found", "The page you requested does not exist")
		if !strings.Contains(html, "404") {
			t.Error("expected error code 404")
		}
		if !strings.Contains(html, "Page Not Found") {
			t.Error("expected title 'Page Not Found'")
		}
		if !strings.Contains(html, "The page you requested does not exist") {
			t.Error("expected message to appear verbatim")
		}
	})

	t.Run("XSS prevention", func(t *testing.T) {
		html := errorPageHTML(500, "<script>alert(1)</script>", `"><img src=x onerror=alert(1)>`)
		// html/template 转义 < > "，script 标签不应作为 HTML 出现
		if strings.Contains(html, "<script>alert(1)</script>") {
			t.Error("title script tag must be escaped")
		}
		if strings.Contains(html, "<img src=x") {
			t.Error("img tag must be escaped, not rendered as HTML")
		}
		if !strings.Contains(html, "&lt;script&gt;") {
			t.Error("expected escaped <script> in output")
		}
	})

	t.Run("status code placement", func(t *testing.T) {
		html := errorPageHTML(403, "Forbidden", "Access denied")
		if !strings.Contains(html, ">403<") {
			t.Error("expected 403 in error-code div")
		}
	})
}
