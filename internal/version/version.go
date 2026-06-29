// Package version 暴露构建版本信息;由 ldflags 注入,缺省为 dev。
package version

import "fmt"

var Version = "dev"

// Info 返回格式化的版本信息字符串。
func Info() string {
	return fmt.Sprintf("Atria %s", Version)
}

// Short 返回简短版本号。
func Short() string {
	return Version
}
