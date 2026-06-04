package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// CheckLatest 从 GitHub 查询最新 Release。
func (u *DefaultUpdater) CheckLatest(ctx context.Context, opts CheckOptions) (*ReleaseInfo, error) {
	u.state.Status = StatusChecking
	u.state.Message = "正在检查更新..."

	var url string
	if opts.CustomCheckURL != "" {
		url = opts.CustomCheckURL
	} else if u.checkURL != "" {
		url = u.checkURL
	} else {
		if opts.Repo == "" {
			opts.Repo = u.repo
		}
		if opts.AllowPrerelease {
			url = fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=10", opts.Repo)
		} else {
			url = fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", opts.Repo)
		}
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		u.state.Status = StatusFailed
		u.state.Error = "创建请求失败"
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Atria-Updater")

	resp, err := client.Do(req)
	if err != nil {
		u.state.Status = StatusFailed
		u.state.Error = "请求失败"
		return nil, fmt.Errorf("请求 GitHub API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		u.state.Status = StatusFailed
		u.state.Error = fmt.Sprintf("GitHub API 返回 %d", resp.StatusCode)
		return nil, fmt.Errorf("GitHub API 返回状态码 %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		u.state.Status = StatusFailed
		u.state.Error = "读取响应失败"
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 根据是否允许预发布选择解析方式
	var release ReleaseInfo
	if opts.AllowPrerelease {
		var releases []ReleaseInfo
		if err := json.Unmarshal(body, &releases); err != nil {
			u.state.Status = StatusFailed
			u.state.Error = "解析响应失败"
			return nil, fmt.Errorf("解析 GitHub 响应失败: %w", err)
		}
		if len(releases) == 0 {
			u.state.Status = StatusUpToDate
			u.state.Message = "没有可用的 Release"
			return nil, fmt.Errorf("没有可用的 Release")
		}
		release = releases[0]
	} else {
		if err := json.Unmarshal(body, &release); err != nil {
			u.state.Status = StatusFailed
			u.state.Error = "解析响应失败"
			return nil, fmt.Errorf("解析 GitHub 响应失败: %w", err)
		}
	}

	// 查找 checksum 资产
	for i, asset := range release.Assets {
		if strings.Contains(asset.Name, "checksum") {
			release.ChecksumAsset = &release.Assets[i]
			break
		}
	}

	// 更新状态
	now := time.Now()
	u.state.LatestVersion = release.TagName
	u.state.CheckedAt = &now
	u.state.Message = ""

	if release.TagName != "" && release.TagName != u.currentVersion {
		u.state.Status = StatusUpdateAvailable
		u.state.Message = fmt.Sprintf("新版本 %s 可用", release.TagName)
	} else {
		u.state.Status = StatusUpToDate
		u.state.Message = "当前已是最新版本"
	}

	return &release, nil
}
