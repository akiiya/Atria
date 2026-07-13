package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/user/atria/internal/config"

	"github.com/gin-gonic/gin"
)

func TestTLSDetectMiddleware_XForwardedProto(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{CookieSecure: false}
	s := &Server{cfg: cfg}

	r := gin.New()
	r.Use(s.tlsDetectMiddleware())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"secure": s.isTLSCookieSecure(c)})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	r.ServeHTTP(w, req)

	if w.Body.String() != `{"secure":true}` {
		t.Errorf("X-Forwarded-Proto: https 应导致 secure=true，实际=%s", w.Body.String())
	}
}

func TestTLSDetectMiddleware_NoHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{CookieSecure: false}
	s := &Server{cfg: cfg}

	r := gin.New()
	r.Use(s.tlsDetectMiddleware())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"secure": s.isTLSCookieSecure(c)})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Body.String() != `{"secure":false}` {
		t.Errorf("无 header 时应 secure=false，实际=%s", w.Body.String())
	}
}

func TestTLSDetectMiddleware_ConfigSecureTrue(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{CookieSecure: true}
	s := &Server{cfg: cfg}

	r := gin.New()
	r.Use(s.tlsDetectMiddleware())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"secure": s.isTLSCookieSecure(c)})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Body.String() != `{"secure":true}` {
		t.Errorf("config Secure=true 时应始终 secure=true，实际=%s", w.Body.String())
	}
}

func TestTLSDetectMiddleware_HTTPSProto(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{CookieSecure: false}
	s := &Server{cfg: cfg}

	r := gin.New()
	r.Use(s.tlsDetectMiddleware())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"secure": s.isTLSCookieSecure(c)})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Forwarded-Proto", "HTTPS") // 大写
	r.ServeHTTP(w, req)

	if w.Body.String() != `{"secure":true}` {
		t.Errorf("大写 HTTPS 也应生效，实际=%s", w.Body.String())
	}
}

func TestTLSDetectMiddleware_HTTPProto(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{CookieSecure: false}
	s := &Server{cfg: cfg}

	r := gin.New()
	r.Use(s.tlsDetectMiddleware())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"secure": s.isTLSCookieSecure(c)})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Forwarded-Proto", "http")
	r.ServeHTTP(w, req)

	if w.Body.String() != `{"secure":false}` {
		t.Errorf("http 不应设置 secure，实际=%s", w.Body.String())
	}
}
