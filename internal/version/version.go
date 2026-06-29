// Package version 提供 Atria 的版本信息。
// 后续 GitHub Actions 可通过 ldflags 注入实际值。
package version

import "fmt"

// 版本信息变量，构建时可通过 ldflags 覆盖：
//
//	go build -ldflags "-X github.com/user/atria/internal/version.Version=v1.0.0
//	  -X github.com/user/atria/internal/version.Commit=abc123
//	  -X github.com/user/atria/internal/version.BuildDate=2024-01-01"
var (
	// Version 是语义化版本号
	Version = "0.1.0-dev"

	// Commit 是 Git 提交哈希
	Commit = "unknown"

	// BuildDate 是构建日期
	BuildDate = "unknown"
)

// Info 返回格式化的版本信息字符串。
func Info() string {
	return fmt.Sprintf("Atria %s (commit: %s, built: %s)", Version, Commit, BuildDate)
}

// Short 返回简短版本号。
func Short() string {
	return Version
}
