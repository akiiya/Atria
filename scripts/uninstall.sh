#!/bin/bash
# Atria 卸载脚本
# 注释中文优先

set -euo pipefail

# ===== 配置 =====
INSTALL_DIR="${ATRIA_INSTALL_DIR:-/usr/local/bin}"
DATA_DIR="${ATRIA_DATA_DIR:-/var/lib/atria}"
LOG_DIR="${ATRIA_LOG_DIR:-/var/log/atria}"
SERVICE_USER="${ATRIA_SERVICE_USER:-atria}"
SKIP_SYSTEMD="${ATRIA_SKIP_SYSTEMD:-0}"

# Try-Run 模式
DRY_RUN="${ATRIA_UNINSTALL_DRY_RUN:-0}"
INSTALL_ROOT="${ATRIA_INSTALL_ROOT:-}"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[信息]${NC} $*"; }
log_warn() { echo -e "${YELLOW}[警告]${NC} $*"; }
log_error() { echo -e "${RED}[错误]${NC} $*"; }

# ===== 主流程 =====
main() {
    echo "=========================================="
    echo "Atria 卸载脚本"
    echo "=========================================="

    # Try-Run 模式调整路径
    if [ -n "$INSTALL_ROOT" ]; then
        INSTALL_DIR="$INSTALL_ROOT/usr/local/bin"
        DATA_DIR="$INSTALL_ROOT/var/lib/atria"
        LOG_DIR="$INSTALL_ROOT/var/log/atria"
        DRY_RUN=1
    fi

    if [ "$DRY_RUN" = "1" ]; then
        log_info "[Try-Run] 模拟卸载，不写入真实系统路径"
    fi

    # 停止并禁用 systemd 服务
    if [ "$SKIP_SYSTEMD" != "1" ]; then
        log_info "停止 Atria 服务..."
        if [ "$DRY_RUN" = "1" ]; then
            log_info "[Try-Run] 跳过 systemctl stop"
        else
            systemctl stop atria 2>/dev/null || true
        fi

        log_info "禁用 Atria 服务..."
        if [ "$DRY_RUN" = "1" ]; then
            log_info "[Try-Run] 跳过 systemctl disable"
        else
            systemctl disable atria 2>/dev/null || true
        fi

        log_info "删除 service 文件..."
        if [ "$DRY_RUN" = "1" ]; then
            log_info "[Try-Run] 跳过删除 /etc/systemd/system/atria.service"
        else
            rm -f /etc/systemd/system/atria.service
            systemctl daemon-reload
        fi
    fi

    # 删除二进制
    log_info "删除二进制..."
    if [ -f "$INSTALL_DIR/atria" ]; then
        if [ "$DRY_RUN" = "1" ]; then
            log_info "[Try-Run] 跳过删除 $INSTALL_DIR/atria"
        else
            rm -f "$INSTALL_DIR/atria"
        fi
    fi

    # 删除备份
    if [ -f "$INSTALL_DIR/atria.bak" ]; then
        if [ "$DRY_RUN" = "1" ]; then
            log_info "[Try-Run] 跳过删除 $INSTALL_DIR/atria.bak"
        else
            rm -f "$INSTALL_DIR/atria.bak"
        fi
    fi

    # 提示数据目录
    echo ""
    echo "=========================================="
    echo "卸载完成"
    echo "=========================================="
    echo ""
    echo "以下目录未删除（保留用户数据）："
    echo "  数据目录: $DATA_DIR"
    echo "  日志目录: $LOG_DIR"
    echo ""
    echo "如需彻底删除数据，请手动执行："
    echo "  rm -rf $DATA_DIR"
    echo "  rm -rf $LOG_DIR"
    echo ""

    if [ "$DRY_RUN" = "1" ]; then
        log_info "[Try-Run] 模拟卸载完成"
    fi
}

main "$@"
