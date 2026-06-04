#!/bin/bash
# Atria 一键安装脚本
# 支持 Linux amd64/arm64
# 注释中文优先

set -euo pipefail

# ===== 配置 =====
REPO="${ATRIA_REPO:-user/atria}"
VERSION="${ATRIA_VERSION:-latest}"
INSTALL_DIR="${ATRIA_INSTALL_DIR:-/usr/local/bin}"
DATA_DIR="${ATRIA_DATA_DIR:-/var/lib/atria}"
LOG_DIR="${ATRIA_LOG_DIR:-/var/log/atria}"
SERVICE_USER="${ATRIA_SERVICE_USER:-atria}"
SKIP_SYSTEMD="${ATRIA_SKIP_SYSTEMD:-0}"
SKIP_USER_CREATE="${ATRIA_SKIP_USER_CREATE:-0}"

# Try-Run 模式
DRY_RUN="${ATRIA_INSTALL_DRY_RUN:-0}"
INSTALL_ROOT="${ATRIA_INSTALL_ROOT:-}"
RELEASE_BASE_URL="${ATRIA_RELEASE_BASE_URL:-}"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[信息]${NC} $*"; }
log_warn() { echo -e "${YELLOW}[警告]${NC} $*"; }
log_error() { echo -e "${RED}[错误]${NC} $*"; }

# ===== 检测平台 =====
detect_platform() {
    local os arch

    os="$(uname -s | tr '[:upper:]' '[:lower:]')"
    arch="$(uname -m)"

    case "$os" in
        linux)  ;;
        *)      log_error "不支持的操作系统: $os（当前仅支持 Linux）"; exit 1 ;;
    esac

    case "$arch" in
        x86_64)  arch="amd64" ;;
        aarch64) arch="arm64" ;;
        arm64)   arch="arm64" ;;
        *)       log_error "不支持的架构: $arch"; exit 1 ;;
    esac

    OS="$os"
    ARCH="$arch"
    PLATFORM="${os}_${arch}"
}

# ===== 获取最新版本 =====
get_latest_version() {
    if [ -n "$RELEASE_BASE_URL" ]; then
        echo "$VERSION"
        return
    fi

    local url="https://api.github.com/repos/${REPO}/releases/latest"
    local version
    version=$(curl -s "$url" | grep '"tag_name"' | cut -d'"' -f4)

    if [ -z "$version" ]; then
        log_error "无法获取最新版本"
        exit 1
    fi

    echo "$version"
}

# ===== 下载文件 =====
download_file() {
    local url="$1"
    local dest="$2"

    if [ -n "$RELEASE_BASE_URL" ]; then
        # 从本地目录复制
        local filename
        filename=$(basename "$url")
        cp "$RELEASE_BASE_URL/$filename" "$dest"
    else
        curl -fsSL "$url" -o "$dest"
    fi
}

# ===== 主流程 =====
main() {
    echo "=========================================="
    echo "Atria 安装脚本"
    echo "=========================================="

    # 创建临时目录（必须在 trap 之前初始化，避免 unbound variable）
    local tmp_dir
    tmp_dir=$(mktemp -d)
    trap 'rm -rf "$tmp_dir"' EXIT

    # 检测平台
    detect_platform
    log_info "检测到平台: $PLATFORM"

    # 获取版本
    if [ "$VERSION" = "latest" ]; then
        VERSION=$(get_latest_version)
    fi
    log_info "安装版本: $VERSION"

    # 构建下载 URL
    local archive_name="atria_${PLATFORM}.tar.gz"
    if [ "$OS" = "windows" ]; then
        archive_name="atria_${PLATFORM}.zip"
    fi

    local base_url="https://github.com/${REPO}/releases/download/${VERSION}"
    if [ -n "$RELEASE_BASE_URL" ]; then
        base_url="$RELEASE_BASE_URL"
    fi

    local download_url="$base_url/$archive_name"
    local checksum_url="$base_url/checksums.txt"

    # Try-Run 模式调整路径
    if [ -n "$INSTALL_ROOT" ]; then
        INSTALL_DIR="$INSTALL_ROOT/usr/local/bin"
        DATA_DIR="$INSTALL_ROOT/var/lib/atria"
        LOG_DIR="$INSTALL_ROOT/var/log/atria"
        DRY_RUN=1
    fi

    if [ "$DRY_RUN" = "1" ]; then
        log_info "[Try-Run] 模拟安装，不写入真实系统路径"
    fi

    # 下载
    log_info "下载 $archive_name..."
    download_file "$download_url" "$tmp_dir/$archive_name"

    log_info "下载 checksums.txt..."
    download_file "$checksum_url" "$tmp_dir/checksums.txt"

    # 校验 checksum
    log_info "校验 checksum..."
    local expected
    expected=$(grep "$archive_name" "$tmp_dir/checksums.txt" | cut -d' ' -f1)
    local actual
    actual=$(sha256sum "$tmp_dir/$archive_name" | cut -d' ' -f1)

    if [ "$expected" != "$actual" ]; then
        log_error "checksum 校验失败"
        log_error "期望: $expected"
        log_error "实际: $actual"
        exit 1
    fi
    log_info "checksum 校验通过"

    # 解压
    log_info "解压..."
    mkdir -p "$tmp_dir/extracted"
    tar -xzf "$tmp_dir/$archive_name" -C "$tmp_dir/extracted"

    # 创建系统用户
    if [ "$SKIP_USER_CREATE" != "1" ] && [ "$DRY_RUN" != "1" ]; then
        if ! id "$SERVICE_USER" >/dev/null 2>&1; then
            log_info "创建系统用户 $SERVICE_USER..."
            useradd --system --no-create-home --shell /bin/false "$SERVICE_USER" 2>/dev/null || true
        fi
    fi

    # 安装二进制
    log_info "安装二进制到 $INSTALL_DIR/atria..."
    mkdir -p "$INSTALL_DIR"

    # 备份旧二进制
    if [ -f "$INSTALL_DIR/atria" ]; then
        log_info "备份旧二进制..."
        cp "$INSTALL_DIR/atria" "$INSTALL_DIR/atria.bak"
    fi

    cp "$tmp_dir/extracted/atria" "$INSTALL_DIR/atria"
    chmod +x "$INSTALL_DIR/atria"

    # 创建数据目录
    log_info "创建数据目录..."
    mkdir -p "$DATA_DIR/sessions"
    mkdir -p "$LOG_DIR"

    # 设置目录权限
    if [ "$SKIP_USER_CREATE" != "1" ] && [ "$DRY_RUN" != "1" ]; then
        chown -R "$SERVICE_USER:$SERVICE_USER" "$DATA_DIR" "$LOG_DIR" 2>/dev/null || true
    fi

    # 创建 systemd service
    if [ "$SKIP_SYSTEMD" != "1" ]; then
        log_info "创建 systemd service..."

        local service_content="[Unit]
Description=Atria MTProto Session Manager
After=network.target

[Service]
Type=simple
User=$SERVICE_USER
ExecStart=$INSTALL_DIR/atria serve
WorkingDirectory=$DATA_DIR
Restart=on-failure
RestartSec=5
Environment=ATRIA_HOST=127.0.0.1
Environment=ATRIA_PORT=8080
Environment=ATRIA_DATA_DIR=$DATA_DIR
Environment=ATRIA_SESSION_DIR=$DATA_DIR/sessions
Environment=ATRIA_LOG_DIR=$LOG_DIR

[Install]
WantedBy=multi-user.target"

        if [ "$DRY_RUN" = "1" ]; then
            local service_file="$INSTALL_ROOT/etc/systemd/system/atria.service"
            mkdir -p "$(dirname "$service_file")"
            echo "$service_content" > "$service_file"
            log_info "[Try-Run] service 文件已写入: $service_file"
        else
            echo "$service_content" > /etc/systemd/system/atria.service
            systemctl daemon-reload
            systemctl enable --now atria
            log_info "服务已启动"
        fi
    fi

    # 完成
    echo ""
    echo "=========================================="
    echo "安装完成"
    echo "=========================================="
    echo ""
    echo "访问地址: http://127.0.0.1:8080"
    echo ""
    echo "数据目录: $DATA_DIR"
    echo "日志目录: $LOG_DIR"
    echo ""
    echo "重要提醒："
    echo "  1. 请备份 $DATA_DIR/secret.key"
    echo "  2. 如需公网访问，请使用反向代理和 HTTPS"
    echo "  3. 不要提交 data 目录或 secret.key 到代码仓库"
    echo ""

    if [ "$DRY_RUN" = "1" ]; then
        log_info "[Try-Run] 模拟安装完成，文件位于: $INSTALL_ROOT"
    fi
}

main "$@"
