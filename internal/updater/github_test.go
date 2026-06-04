package updater

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckLatest_MockSuccess(t *testing.T) {
	release := ReleaseInfo{
		TagName: "v0.2.0",
		Name:    "Release v0.2.0",
		Body:    "Test release",
		Assets: []AssetInfo{
			{Name: "atria_linux_amd64.tar.gz", URL: "http://example.com/atria_linux_amd64.tar.gz", Size: 1000},
			{Name: "checksums.txt", URL: "http://example.com/checksums.txt", Size: 100},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(release)
	}))
	defer server.Close()

	u := NewDefaultUpdater("v0.1.0", "test/repo", server.URL, "", true, nil)

	result, err := u.CheckLatest(context.Background(), CheckOptions{
		CustomCheckURL: server.URL,
	})
	if err != nil {
		t.Fatalf("检查失败: %s", err)
	}

	if result.TagName != "v0.2.0" {
		t.Errorf("版本不匹配，期望 v0.2.0，实际 %s", result.TagName)
	}
}

func TestCheckLatest_PrereleaseAllowed(t *testing.T) {
	releases := []ReleaseInfo{
		{TagName: "v0.2.0-alpha", Prerelease: true},
		{TagName: "v0.1.0", Prerelease: false},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(releases)
	}))
	defer server.Close()

	u := NewDefaultUpdater("v0.1.0", "test/repo", server.URL, "", true, nil)

	result, err := u.CheckLatest(context.Background(), CheckOptions{
		CustomCheckURL:  server.URL,
		AllowPrerelease: true,
	})
	if err != nil {
		t.Fatalf("检查失败: %s", err)
	}

	if result.TagName != "v0.2.0-alpha" {
		t.Errorf("应该返回 prerelease，实际 %s", result.TagName)
	}
}

func TestCheckLatest_NoAssets(t *testing.T) {
	release := ReleaseInfo{
		TagName: "v0.2.0",
		Assets:  []AssetInfo{},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(release)
	}))
	defer server.Close()

	u := NewDefaultUpdater("v0.1.0", "test/repo", server.URL, "", true, nil)

	result, err := u.CheckLatest(context.Background(), CheckOptions{
		CustomCheckURL: server.URL,
	})
	if err != nil {
		t.Fatalf("检查失败: %s", err)
	}

	if len(result.Assets) != 0 {
		t.Error("应该没有资产")
	}
}

func TestCheckLatest_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	u := NewDefaultUpdater("v0.1.0", "test/repo", server.URL, "", true, nil)

	_, err := u.CheckLatest(context.Background(), CheckOptions{
		CustomCheckURL: server.URL,
	})
	if err == nil {
		t.Error("服务器错误应该返回错误")
	}
}

func TestCheckLatest_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{invalid json"))
	}))
	defer server.Close()

	u := NewDefaultUpdater("v0.1.0", "test/repo", server.URL, "", true, nil)

	_, err := u.CheckLatest(context.Background(), CheckOptions{
		CustomCheckURL: server.URL,
	})
	if err == nil {
		t.Error("无效 JSON 应该返回错误")
	}
}

func TestCheckLatest_ChecksumAssetDetected(t *testing.T) {
	release := ReleaseInfo{
		TagName: "v0.2.0",
		Assets: []AssetInfo{
			{Name: "atria_linux_amd64.tar.gz", URL: "http://example.com/atria.tar.gz"},
			{Name: "checksums.txt", URL: "http://example.com/checksums.txt"},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(release)
	}))
	defer server.Close()

	u := NewDefaultUpdater("v0.1.0", "test/repo", server.URL, "", true, nil)

	result, err := u.CheckLatest(context.Background(), CheckOptions{
		CustomCheckURL: server.URL,
	})
	if err != nil {
		t.Fatalf("检查失败: %s", err)
	}

	if result.ChecksumAsset == nil {
		t.Error("应该检测到 checksum 资产")
	}
}

func TestCheckLatest_UpdateStateUpdated(t *testing.T) {
	release := ReleaseInfo{
		TagName: "v0.2.0",
		Assets:  []AssetInfo{{Name: "atria_linux_amd64.tar.gz"}},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(release)
	}))
	defer server.Close()

	u := NewDefaultUpdater("v0.1.0", "test/repo", server.URL, "", true, nil)

	u.CheckLatest(context.Background(), CheckOptions{
		CustomCheckURL: server.URL,
	})

	state := u.GetState()
	if state.Status != StatusUpdateAvailable {
		t.Errorf("状态应为 update_available，实际 %s", state.Status)
	}
	if state.LatestVersion != "v0.2.0" {
		t.Errorf("最新版本应为 v0.2.0，实际 %s", state.LatestVersion)
	}
	if state.CheckedAt == nil {
		t.Error("CheckedAt 应该被设置")
	}
}
