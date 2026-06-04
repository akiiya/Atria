package updater

import (
	"testing"
)

func TestSelectAsset_LinuxAmd64(t *testing.T) {
	u := NewDefaultUpdater("v0.1.0", "test/repo", "", "", true, nil)
	release := ReleaseInfo{
		Assets: []AssetInfo{
			{Name: "atria_linux_amd64.tar.gz"},
			{Name: "atria_linux_arm64.tar.gz"},
		},
	}

	asset, err := u.SelectAsset(release, "linux", "amd64")
	if err != nil {
		t.Fatalf("选择失败: %s", err)
	}
	if asset.Name != "atria_linux_amd64.tar.gz" {
		t.Errorf("期望 atria_linux_amd64.tar.gz，实际 %s", asset.Name)
	}
}

func TestSelectAsset_LinuxArm64(t *testing.T) {
	u := NewDefaultUpdater("v0.1.0", "test/repo", "", "", true, nil)
	release := ReleaseInfo{
		Assets: []AssetInfo{
			{Name: "atria_linux_amd64.tar.gz"},
			{Name: "atria_linux_arm64.tar.gz"},
		},
	}

	asset, err := u.SelectAsset(release, "linux", "arm64")
	if err != nil {
		t.Fatalf("选择失败: %s", err)
	}
	if asset.Name != "atria_linux_arm64.tar.gz" {
		t.Errorf("期望 atria_linux_arm64.tar.gz，实际 %s", asset.Name)
	}
}

func TestSelectAsset_WindowsAmd64(t *testing.T) {
	u := NewDefaultUpdater("v0.1.0", "test/repo", "", "", true, nil)
	release := ReleaseInfo{
		Assets: []AssetInfo{
			{Name: "atria_windows_amd64.zip"},
			{Name: "atria_windows_arm64.zip"},
		},
	}

	asset, err := u.SelectAsset(release, "windows", "amd64")
	if err != nil {
		t.Fatalf("选择失败: %s", err)
	}
	if asset.Name != "atria_windows_amd64.zip" {
		t.Errorf("期望 atria_windows_amd64.zip，实际 %s", asset.Name)
	}
}

func TestSelectAsset_DarwinAmd64(t *testing.T) {
	u := NewDefaultUpdater("v0.1.0", "test/repo", "", "", true, nil)
	release := ReleaseInfo{
		Assets: []AssetInfo{
			{Name: "atria_darwin_amd64.tar.gz"},
			{Name: "atria_darwin_arm64.tar.gz"},
		},
	}

	asset, err := u.SelectAsset(release, "darwin", "amd64")
	if err != nil {
		t.Fatalf("选择失败: %s", err)
	}
	if asset.Name != "atria_darwin_amd64.tar.gz" {
		t.Errorf("期望 atria_darwin_amd64.tar.gz，实际 %s", asset.Name)
	}
}

func TestSelectAsset_DarwinArm64(t *testing.T) {
	u := NewDefaultUpdater("v0.1.0", "test/repo", "", "", true, nil)
	release := ReleaseInfo{
		Assets: []AssetInfo{
			{Name: "atria_darwin_amd64.tar.gz"},
			{Name: "atria_darwin_arm64.tar.gz"},
		},
	}

	asset, err := u.SelectAsset(release, "darwin", "arm64")
	if err != nil {
		t.Fatalf("选择失败: %s", err)
	}
	if asset.Name != "atria_darwin_arm64.tar.gz" {
		t.Errorf("期望 atria_darwin_arm64.tar.gz，实际 %s", asset.Name)
	}
}

func TestSelectAsset_NoMatch(t *testing.T) {
	u := NewDefaultUpdater("v0.1.0", "test/repo", "", "", true, nil)
	release := ReleaseInfo{
		Assets: []AssetInfo{
			{Name: "atria_linux_amd64.tar.gz"},
		},
	}

	_, err := u.SelectAsset(release, "freebsd", "amd64")
	if err == nil {
		t.Error("不匹配的平台应该返回错误")
	}
}

func TestSelectAsset_ChecksumsNotSelected(t *testing.T) {
	u := NewDefaultUpdater("v0.1.0", "test/repo", "", "", true, nil)
	release := ReleaseInfo{
		Assets: []AssetInfo{
			{Name: "checksums.txt"},
			{Name: "atria_linux_amd64.tar.gz"},
		},
	}

	asset, err := u.SelectAsset(release, "linux", "amd64")
	if err != nil {
		t.Fatalf("选择失败: %s", err)
	}
	if asset.Name == "checksums.txt" {
		t.Error("不应该选择 checksums.txt")
	}
}

func TestGetPlatformAssetName(t *testing.T) {
	tests := []struct {
		os       string
		arch     string
		expected string
	}{
		{"linux", "amd64", "atria_linux_amd64.tar.gz"},
		{"linux", "arm64", "atria_linux_arm64.tar.gz"},
		{"windows", "amd64", "atria_windows_amd64.zip"},
		{"darwin", "amd64", "atria_darwin_amd64.tar.gz"},
	}

	for _, tt := range tests {
		result := GetPlatformAssetName(tt.os, tt.arch)
		if result != tt.expected {
			t.Errorf("GetPlatformAssetName(%s, %s) = %s, 期望 %s", tt.os, tt.arch, result, tt.expected)
		}
	}
}

func TestGetState_InitialState(t *testing.T) {
	u := NewDefaultUpdater("v0.1.0", "test/repo", "", "", true, nil)
	state := u.GetState()

	if state.Status != StatusIdle {
		t.Errorf("初始状态应为 idle，实际 %s", state.Status)
	}
	if state.CurrentVersion != "v0.1.0" {
		t.Errorf("当前版本应为 v0.1.0，实际 %s", state.CurrentVersion)
	}
}

func TestSetState(t *testing.T) {
	u := NewDefaultUpdater("v0.1.0", "test/repo", "", "", true, nil)

	newState := UpdateState{
		Status:         StatusUpdateAvailable,
		CurrentVersion: "v0.1.0",
		LatestVersion:  "v0.2.0",
		Message:        "有新版本可用",
	}
	u.SetState(newState)

	state := u.GetState()
	if state.Status != StatusUpdateAvailable {
		t.Errorf("状态应为 update_available，实际 %s", state.Status)
	}
	if state.LatestVersion != "v0.2.0" {
		t.Errorf("最新版本应为 v0.2.0，实际 %s", state.LatestVersion)
	}
}
