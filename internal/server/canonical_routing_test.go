package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func assertRedirectLocation(t *testing.T, r http.Handler, sessionCookie, path, expected string) {
	t.Helper()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", path, nil)
	if sessionCookie != "" {
		req.Header.Set("Cookie", "atria_session="+sessionCookie)
	}
	r.ServeHTTP(w, req)
	if w.Code != http.StatusFound {
		t.Fatalf("%s expected 302, got %d body=%s", path, w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); loc != expected {
		t.Fatalf("%s expected Location %s, got %s", path, expected, loc)
	}
}

func TestRootRedirectsToAppDashboardWhenAuthenticated(t *testing.T) {
	r, _ := setupTestRouter(t)
	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)
	assertRedirectLocation(t, r, sessionCookie, "/", "/app/#/dashboard")
}

func TestRootRedirectsToLoginWhenUnauthenticated(t *testing.T) {
	r, _ := setupTestRouter(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusFound || w.Header().Get("Location") != "/login" {
		t.Fatalf("unauthenticated / expected redirect to /login, status=%d location=%s", w.Code, w.Header().Get("Location"))
	}
}

func TestLegacyDashboardRedirectsToHashDashboard(t *testing.T) {
	r, _ := setupTestRouter(t)
	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)
	assertRedirectLocation(t, r, sessionCookie, "/dashboard", "/app/#/dashboard")
}

func TestLegacyAccountsRedirectsToHashAccounts(t *testing.T) {
	r, _ := setupTestRouter(t)
	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)
	assertRedirectLocation(t, r, sessionCookie, "/accounts", "/app/#/accounts")
}

func TestLegacyChatsRedirectsToHashChats(t *testing.T) {
	r, _ := setupTestRouter(t)
	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)
	assertRedirectLocation(t, r, sessionCookie, "/chats", "/app/#/chats")
}

func TestLegacySettingsRedirectsToHashSettings(t *testing.T) {
	r, _ := setupTestRouter(t)
	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)
	assertRedirectLocation(t, r, sessionCookie, "/settings", "/app/#/settings")
}

func TestAPIRoutesNotAffectedBySPARedirect(t *testing.T) {
	r, _ := setupTestRouter(t)
	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/me", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)
	if w.Code == http.StatusFound || strings.Contains(w.Body.String(), `id="app"`) {
		t.Fatalf("/api/me should return JSON, status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestLoginRouteNotAffectedBySPARedirect(t *testing.T) {
	r, _ := setupTestRouter(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/login", nil)
	r.ServeHTTP(w, req)
	if w.Code == http.StatusFound && strings.Contains(w.Header().Get("Location"), "/app/#/") {
		t.Fatalf("/login should not redirect to SPA, location=%s", w.Header().Get("Location"))
	}
}

func TestInitRouteNotAffectedBySPARedirect(t *testing.T) {
	r, _ := setupTestRouter(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/init", nil)
	r.ServeHTTP(w, req)
	if w.Code == http.StatusFound && strings.Contains(w.Header().Get("Location"), "/app/#/") {
		t.Fatalf("/init should not redirect to SPA, location=%s", w.Header().Get("Location"))
	}
}

func TestHealthzNotAffectedBySPARedirect(t *testing.T) {
	r, _ := setupTestRouter(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/healthz", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), `id="app"`) {
		t.Fatalf("/healthz should return health JSON, status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestAppDefaultRouteRedirectsToDashboard(t *testing.T) {
	content := readFileContent(t, "frontend/src/router/index.ts")
	if !strings.Contains(content, "path: '/'") || !strings.Contains(content, "redirect: '/dashboard'") {
		t.Fatal("Vue hash root must redirect to /dashboard")
	}
}

func TestAppHashRootRedirectsToDashboard(t *testing.T) {
	content := readFileContent(t, "frontend/src/router/index.ts")
	if !strings.Contains(content, "createWebHashHistory('/app/')") || !strings.Contains(content, "redirect: '/dashboard'") {
		t.Fatal("/app/#/ must be handled by hash router and redirect to dashboard")
	}
}

func TestSidebarLinksUseHashRoutes(t *testing.T) {
	content := readFileContent(t, "frontend/src/components/Sidebar.vue")
	if !strings.Contains(content, "router.push(path)") {
		t.Fatal("Sidebar must use Vue router push")
	}
	for _, old := range []string{`href="/"`, `href="/accounts"`, `href="/chats"`, `href="/settings"`} {
		if strings.Contains(content, old) {
			t.Fatalf("Sidebar must not contain legacy link %s", old)
		}
	}
}

func TestNoAppAppRouteGenerated(t *testing.T) {
	r, _ := setupTestRouter(t)
	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)
	assertRedirectLocation(t, r, sessionCookie, "/app/app/accounts", "/app/#/accounts")

	for _, path := range []string{"frontend/src/main.ts", "frontend/src/router/index.ts", "frontend/src/components/Sidebar.vue"} {
		if strings.Contains(readFileContent(t, path), "/app/app") {
			t.Fatalf("%s must not generate /app/app URLs", path)
		}
	}
}

func TestNoLegacyAccountsHashRouteGenerated(t *testing.T) {
	content := readFileContent(t, "frontend/src/main.ts")
	if strings.Contains(content, "loc.hash") || strings.Contains(content, "accounts#/dashboard") {
		t.Fatal("history-style canonicalization must not append an existing hash")
	}
}

func TestDashboardUsesVueAppShell(t *testing.T) {
	app := readFileContent(t, "frontend/src/App.vue")
	router := readFileContent(t, "frontend/src/router/index.ts")
	if !strings.Contains(app, "AppShell") || !strings.Contains(router, "DashboardView.vue") {
		t.Fatal("Dashboard must render inside Vue AppShell")
	}
}

func TestDashboardAndAccountsShareAppShellLayout(t *testing.T) {
	app := readFileContent(t, "frontend/src/App.vue")
	router := readFileContent(t, "frontend/src/router/index.ts")
	if strings.Count(app, "AppShell") == 0 ||
		!strings.Contains(router, "DashboardView.vue") ||
		!strings.Contains(router, "AccountsView.vue") {
		t.Fatal("Dashboard and Accounts must share the AppShell layout")
	}
}
