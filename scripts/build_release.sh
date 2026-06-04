#!/bin/bash
# Atria 多平台构建脚本
# 构建所有目标平台的二进制并打包
# 注释中文优先

set -euo pipefail

# 版本信息
VERSION="${VERSION:-0.1.0-dev}"
COMMIT="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")}"
BUILD_DATE="${BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"

# 输出目录
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
OUTPUT_DIR="${OUTPUT_DIR:-$PROJECT_DIR/tmp/dist}"

# 构建目标
TARGETS=(
    "linux/amd64"
    "linux/arm64"
    "windows/amd64"
    "windows/arm64"
    "darwin/amd64"
    "darwin/arm64"
)

echo "=========================================="
echo "Atria 构建发布包"
echo "=========================================="
echo "版本: $VERSION"
echo "Commit: $COMMIT"
echo "构建时间: $BUILD_DATE"
echo "输出目录: $OUTPUT_DIR"
echo ""

# 清理并创建输出目录
rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"

# 构建函数
build_target() {
    local goos="$1"
    local goarch="$2"
    local suffix=""
    local archive_ext="tar.gz"

    if [ "$goos" = "windows" ]; then
        suffix=".exe"
        archive_ext="zip"
    fi

    local binary_name="atria${suffix}"
    local archive_name="atria_${goos}_${goarch}.${archive_ext}"
    local build_dir="$OUTPUT_DIR/build_${goos}_${goarch}"

    echo "构建 ${goos}/${goarch}..."

    # 创建构建目录
    mkdir -p "$build_dir"

    # 构建二进制（必须在项目根目录执行）
    cd "$PROJECT_DIR"
    CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build \
        -ldflags "-s -w \
            -X github.com/user/atria/internal/version.Version=$VERSION \
            -X github.com/user/atria/internal/version.Commit=$COMMIT \
            -X github.com/user/atria/internal/version.BuildDate=$BUILD_DATE" \
        -o "$build_dir/$binary_name" \
        ./cmd/atria

    # 复制文档
    cp "$PROJECT_DIR/README.md" "$build_dir/"
    cp "$PROJECT_DIR/LICENSE" "$build_dir/"

    # 创建压缩包
    # 使用绝对路径确保 cd 后仍能找到输出文件
    local archive_path="$OUTPUT_DIR/$archive_name"

    if [ "$archive_ext" = "zip" ]; then
        # Windows zip：cd 到构建目录后打包
        if command -v zip >/dev/null 2>&1; then
            (
                cd "$build_dir"
                zip -qr "$archive_path" .
            )
        else
            # zip 不可用时回退为 tar.gz
            echo "  警告: zip 命令不可用，使用 tar.gz 替代"
            archive_name="atria_${goos}_${goarch}.tar.gz"
            archive_path="$OUTPUT_DIR/$archive_name"
            tar -czf "$archive_path" -C "$build_dir" .
        fi
    else
        # Linux/macOS tar.gz
        tar -czf "$archive_path" -C "$build_dir" .
    fi

    # 清理构建目录
    rm -rf "$build_dir"

    echo "  完成: $archive_name"
}

# 构建所有目标
for target in "${TARGETS[@]}"; do
    IFS='/' read -r goos goarch <<< "$target"
    build_target "$goos" "$goarch"
done

# 生成 checksums.txt
echo ""
echo "生成 checksums.txt..."
cd "$OUTPUT_DIR"
for f in *.tar.gz *.zip; do
    [ -f "$f" ] && sha256sum "$f"
done > checksums.txt

echo ""
echo "=========================================="
echo "构建完成"
echo "=========================================="
echo "产物目录: $OUTPUT_DIR"
echo ""
ls -la "$OUTPUT_DIR"/
