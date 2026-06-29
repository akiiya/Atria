package server

import (
	"fmt"

	"github.com/user/atria/internal/config"
	"github.com/user/atria/internal/version"
)

// ViewData 是统一的页面数据结构。
type ViewData struct {
	// 页面标题
	Title string

	// 当前激活的导航项
	ActiveNav string

	// 版本号
	Version string

	// Git Commit
	Commit string

	// 构建日期
	BuildDate string

	// CSRF Token
	CSRFToken string

	// 系统是否已初始化
	IsInitialized bool

	// 当前是否已登录
	IsAuthenticated bool

	// 当前管理员用户名
	CurrentAdminUsername string

	// 当前选中的 API 凭据 ID
	CurrentCredentialID uint

	// 当前选中的 API 凭据名称
	CurrentCredentialName string

	// 当前选中的 API 凭据脱敏显示
	CurrentCredentialMasked string

	// 凭据切换器列表
	CredentialsForSwitcher any

	// Flash 消息
	Flash string

	// 错误消息
	Error string

	// 系统信息
	DatabaseDriver string
	DatabaseDSN    string
	DataDir        string
	SessionDir     string
	LogDir         string
	ListenAddr     string

	// 页面自定义数据
	Data any
}

// 页面标题映射。
var pageTitles = map[string]string{
	"dashboard":   "仪表盘",
	"init":        "初始化管理员",
	"login":       "管理员登录",
	"settings":    "系统设置",
	"credentials": "API 凭据",
	"accounts":    "账号会话",
	"audit":       "审计日志",
	"security":    "安全说明",
	"403":         "访问被拒绝",
	"404":         "页面未找到",
	"500":         "服务器错误",
}

// NewViewData 创建 ViewData 实例。
func NewViewData(cfg *config.Config, activeNav string) ViewData {
	title, ok := pageTitles[activeNav]
	if !ok {
		title = activeNav
	}

	return ViewData{
		Title:                   title,
		ActiveNav:               activeNav,
		Version:                 version.Short(),
		Commit:                  "",
		BuildDate:               "",
		CurrentCredentialName:   "",
		CurrentCredentialMasked: "未选择凭据",
		DatabaseDriver:          cfg.DatabaseDriver,
		DatabaseDSN:             cfg.MaskedDSN(),
		DataDir:                 cfg.DataDir,
		SessionDir:              cfg.SessionDir,
		LogDir:                  cfg.LogDir,
		ListenAddr:              cfg.ListenAddr(),
	}
}

// ToMap 将 ViewData 转换为 gin.H（用于模板渲染）。
func (v ViewData) ToMap() map[string]any {
	return map[string]any{
		"Title":                   v.Title,
		"ActivePage":              v.ActiveNav,
		"ActiveNav":               v.ActiveNav,
		"Version":                 v.Version,
		"Commit":                  v.Commit,
		"BuildDate":               v.BuildDate,
		"CSRFToken":               v.CSRFToken,
		"IsInitialized":           v.IsInitialized,
		"IsAuthenticated":         v.IsAuthenticated,
		"CurrentAdminUsername":    v.CurrentAdminUsername,
		"CurrentCredentialName":   v.CurrentCredentialName,
		"CurrentCredentialMasked": v.CurrentCredentialMasked,
		"CredentialsForSwitcher":  v.CredentialsForSwitcher,
		"Flash":                   v.Flash,
		"Error":                   v.Error,
		"DatabaseDriver":          v.DatabaseDriver,
		"DatabaseDSN":             v.DatabaseDSN,
		"DataDir":                 v.DataDir,
		"SessionDir":              v.SessionDir,
		"LogDir":                  v.LogDir,
		"ListenAddr":              v.ListenAddr,
		"Data":                    v.Data,
	}
}

// WithCSRF 设置 CSRF Token。
func (v ViewData) WithCSRF(token string) ViewData {
	v.CSRFToken = token
	return v
}

// WithFlash 设置 Flash 消息。
func (v ViewData) WithFlash(msg string) ViewData {
	v.Flash = msg
	return v
}

// WithError 设置错误消息。
func (v ViewData) WithError(msg string) ViewData {
	v.Error = msg
	return v
}

// WithData 设置页面自定义数据。
func (v ViewData) WithData(data any) ViewData {
	v.Data = data
	return v
}

// FormatTitle 返回格式化的页面标题。
func (v ViewData) FormatTitle() string {
	return fmt.Sprintf("%s - Atria", v.Title)
}
