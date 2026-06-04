package updater

import "fmt"

var (
	// ErrUpdateDisabled 表示自更新功能已禁用。
	ErrUpdateDisabled = fmt.Errorf("自更新功能已禁用")

	// ErrUnsupportedPlatform 表示当前平台不支持自更新。
	ErrUnsupportedPlatform = fmt.Errorf("当前平台不支持自更新")

	// ErrDockerEnvironment 表示 Docker 环境不支持容器内自更新。
	ErrDockerEnvironment = fmt.Errorf("Docker 环境不支持容器内自更新，请使用新镜像重建")

	// ErrPermissionDenied 表示权限不足。
	ErrPermissionDenied = fmt.Errorf("权限不足，无法替换二进制文件")

	// ErrNoUpdateAvailable 表示没有可用更新。
	ErrNoUpdateAvailable = fmt.Errorf("当前已是最新版本")

	// ErrChecksumRequired 表示需要 checksum 校验。
	ErrChecksumRequired = fmt.Errorf("checksum 校验是必须的")

	// ErrChecksumMismatch 表示 checksum 不匹配。
	ErrChecksumMismatch = fmt.Errorf("checksum 不匹配")

	// ErrAssetNotFound 表示未找到匹配的产物。
	ErrAssetNotFound = fmt.Errorf("未找到匹配的产物")
)
