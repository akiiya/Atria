#!/bin/bash
# Atria Release 产物检查脚本
# 检查构建产物是否完整、安全
# 注释中文优先

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
DIST_DIR="${1:-$PROJECT_DIR/tmp/dist}"
REPORTS_DIR="$PROJECT_DIR/tmp/reports"
REPORT_FILE="$REPORTS_DIR/artifact_check_report.md"
CHECK_DIR="$PROJECT_DIR/tmp/artifact-check"

# 清理
cleanup() {
    rm -rf "$CHECK_DIR"
}
trap cleanup EXIT

mkdir -p "$REPORTS_DIR" "$CHECK_DIR"

PASS_COUNT=0
FAIL_COUNT=0

log_pass() { echo -e "\033[0;32m[PASS]\033[0m $*"; PASS_COUNT=$((PASS_COUNT + 1)); }
log_fail() { echo -e "\033[0;31m[FAIL]\033[0m $*"; FAIL_COUNT=$((FAIL_COUNT + 1)); }
log_info() { echo -e "\033[0;34m[信息]\033[0m $*"; }

# 报告初始化
cat > "$REPORT_FILE" <<EOF
# Atria Release 产物检查报告

- 执行时间: $(date -u +%Y-%m-%dT%H:%M:%SZ)
- 检查目录: $DIST_DIR

## 检查结果

EOF

echo "=========================================="
echo "Atria Release 产物检查"
echo "=========================================="
echo "检查目录: $DIST_DIR"
echo ""

# 检查目录是否存在
if [ ! -d "$DIST_DIR" ]; then
    log_fail "目录不存在: $DIST_DIR"
    echo "- [ ] 目录存在: 失败" >> "$REPORT_FILE"
    exit 1
fi

# ===== 检查产物存在 =====
log_info "检查产物文件..."

EXPECTED_FILES=(
    "atria_linux_amd64.tar.gz"
    "atria_linux_arm64.tar.gz"
    "atria_darwin_amd64.tar.gz"
    "atria_darwin_arm64.tar.gz"
    "checksums.txt"
)

# Windows 产物可能是 .zip 或 .tar.gz
if ls "$DIST_DIR"/atria_windows_*.zip 2>/dev/null; then
    EXPECTED_FILES+=("atria_windows_amd64.zip" "atria_windows_arm64.zip")
elif ls "$DIST_DIR"/atria_windows_*.tar.gz 2>/dev/null; then
    EXPECTED_FILES+=("atria_windows_amd64.tar.gz" "atria_windows_arm64.tar.gz")
fi

for f in "${EXPECTED_FILES[@]}"; do
    if [ -f "$DIST_DIR/$f" ]; then
        log_pass "产物存在: $f"
        echo "- [x] 产物 $f: 存在" >> "$REPORT_FILE"
    else
        log_fail "产物缺失: $f"
        echo "- [ ] 产物 $f: 缺失" >> "$REPORT_FILE"
    fi
done

# ===== 检查 checksums.txt =====
log_info "校验 checksums.txt..."
cd "$DIST_DIR"
if [ -f checksums.txt ] && sha256sum -c checksums.txt > /dev/null 2>&1; then
    log_pass "checksum 校验通过"
    echo "- [x] checksum 校验: 通过" >> "$REPORT_FILE"
else
    log_fail "checksum 校验失败"
    echo "- [ ] checksum 校验: 失败" >> "$REPORT_FILE"
fi
cd "$PROJECT_DIR"

# ===== 解压并检查内容 =====
log_info "检查产物内容..."

# 敏感文件列表（不应出现在包中）
SENSITIVE_PATTERNS=(
    "data/"
    "tmp/"
    "secret.key"
    "sessions/"
    "logs/"
    "*.db"
    "*.sqlite"
    "*.sqlite3"
    "*.log"
)

# 必须存在的文件
REQUIRED_FILES=("README.md" "LICENSE")

check_archive() {
    local archive="$1"
    local archive_name
    archive_name=$(basename "$archive")
    local extract_dir="$CHECK_DIR/$archive_name"
    mkdir -p "$extract_dir"

    log_info "检查 $archive_name..."

    # 解压
    if [[ "$archive" == *.zip ]]; then
        unzip -q "$archive" -d "$extract_dir" 2>/dev/null || true
    else
        tar -xzf "$archive" -C "$extract_dir" 2>/dev/null || true
    fi

    # 检查必须存在的文件
    for f in "${REQUIRED_FILES[@]}"; do
        if [ -f "$extract_dir/$f" ]; then
            log_pass "  包含: $f"
        else
            log_fail "  缺失: $f"
        fi
    done

    # 检查二进制
    if [ -f "$extract_dir/atria" ] || [ -f "$extract_dir/atria.exe" ]; then
        log_pass "  包含: 二进制文件"
    else
        log_fail "  缺失: 二进制文件"
    fi

    # 检查不应存在的文件
    for pattern in "${SENSITIVE_PATTERNS[@]}"; do
        if find "$extract_dir" -name "$pattern" -o -path "*/$pattern" 2>/dev/null | grep -q .; then
            log_fail "  包含敏感文件: $pattern"
        fi
    done

    # 检查目录结构
    if [ -d "$extract_dir/data" ]; then
        log_fail "  包含 data/ 目录"
    fi
    if [ -d "$extract_dir/tmp" ]; then
        log_fail "  包含 tmp/ 目录"
    fi
    if [ -d "$extract_dir/sessions" ]; then
        log_fail "  包含 sessions/ 目录"
    fi

    # 清理
    rm -rf "$extract_dir"
}

for archive in "$DIST_DIR"/atria_*.tar.gz "$DIST_DIR"/atria_*.zip; do
    [ -f "$archive" ] || continue
    check_archive "$archive"
done

# ===== 汇总 =====
echo ""
echo "=========================================="
echo "检查完成"
echo "=========================================="
echo "通过: $PASS_COUNT"
echo "失败: $FAIL_COUNT"
echo ""

cat >> "$REPORT_FILE" <<EOF

## 汇总

- 通过: $PASS_COUNT
- 失败: $FAIL_COUNT

## 说明

- 检查产物是否包含敏感文件
- 检查产物是否包含必要文件（README、LICENSE、二进制）
- checksum 校验验证文件完整性
EOF

echo "报告已生成: $REPORT_FILE"

if [ "$FAIL_COUNT" -gt 0 ]; then
    exit 1
fi
