#!/bin/bash
# Atria 全流程自动化测试脚本
# 在 tmp/ 沙箱中模拟完整发布链路
# 不访问真实 Telegram 网络，不需要 GitHub token，不需要 root
# 注释中文优先

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
TMP_DIR="$PROJECT_DIR/tmp"
REPORTS_DIR="$TMP_DIR/reports"
REPORT_FILE="$REPORTS_DIR/full_check_report.md"
RELEASE_MOCK_DIR="$TMP_DIR/release-mock"
INSTALL_ROOT="$TMP_DIR/install-root"
HOME_DIR="$TMP_DIR/home"

# 清理
cleanup() {
    if [ "${ATRIA_FULL_CHECK_KEEP_TMP:-0}" != "1" ]; then
        rm -rf "$RELEASE_MOCK_DIR" "$INSTALL_ROOT" "$HOME_DIR"
    fi
}
trap cleanup EXIT

# 初始化
mkdir -p "$REPORTS_DIR" "$RELEASE_MOCK_DIR" "$INSTALL_ROOT" "$HOME_DIR"
export HOME="$HOME_DIR"

PASS_COUNT=0
FAIL_COUNT=0
SKIP_COUNT=0

log_pass() { echo -e "\033[0;32m[PASS]\033[0m $*"; PASS_COUNT=$((PASS_COUNT + 1)); }
log_fail() { echo -e "\033[0;31m[FAIL]\033[0m $*"; FAIL_COUNT=$((FAIL_COUNT + 1)); }
log_skip() { echo -e "\033[1;33m[SKIP]\033[0m $*"; SKIP_COUNT=$((SKIP_COUNT + 1)); }
log_info() { echo -e "\033[0;34m[信息]\033[0m $*"; }

# 报告初始化
cat > "$REPORT_FILE" <<EOF
# Atria 全流程自动化测试报告

- 执行时间: $(date -u +%Y-%m-%dT%H:%M:%SZ)
- 项目目录: $PROJECT_DIR
- 临时目录: $TMP_DIR

## 测试结果

EOF

echo "=========================================="
echo "Atria 全流程自动化测试"
echo "=========================================="
echo "项目目录: $PROJECT_DIR"
echo "临时目录: $TMP_DIR"
echo ""

# ===== 步骤 1: gofmt =====
log_info "[步骤 1] gofmt 检查..."
cd "$PROJECT_DIR"
if [ -z "$(gofmt -l .)" ]; then
    log_pass "gofmt 检查通过"
    echo "- [x] gofmt: 通过" >> "$REPORT_FILE"
else
    log_fail "gofmt 检查失败"
    echo "- [ ] gofmt: 失败" >> "$REPORT_FILE"
fi

# ===== 步骤 2: go mod tidy =====
log_info "[步骤 2] go mod tidy 检查..."
if go mod tidy 2>&1; then
    log_pass "go mod tidy 通过"
    echo "- [x] go mod tidy: 通过" >> "$REPORT_FILE"
else
    log_fail "go mod tidy 失败"
    echo "- [ ] go mod tidy: 失败" >> "$REPORT_FILE"
fi

# ===== 步骤 3: go test =====
log_info "[步骤 3] 运行测试..."
if go test ./... -count=1 > "$TMP_DIR/test_output.txt" 2>&1; then
    log_pass "go test 通过"
    echo "- [x] go test: 通过" >> "$REPORT_FILE"
else
    log_fail "go test 失败"
    echo "- [ ] go test: 失败" >> "$REPORT_FILE"
    cat "$TMP_DIR/test_output.txt"
fi

# ===== 步骤 4: go build =====
log_info "[步骤 4] 构建..."
if go build -o "$TMP_DIR/atria" ./cmd/atria 2>&1; then
    log_pass "go build 通过"
    echo "- [x] go build: 通过" >> "$REPORT_FILE"
else
    log_fail "go build 失败"
    echo "- [ ] go build: 失败" >> "$REPORT_FILE"
fi

# ===== 步骤 5: smoke test =====
log_info "[步骤 5] Smoke test..."
if bash "$SCRIPT_DIR/smoke.sh" > "$TMP_DIR/smoke_output.txt" 2>&1; then
    log_pass "smoke test 通过"
    echo "- [x] smoke test: 通过" >> "$REPORT_FILE"
else
    log_fail "smoke test 失败"
    echo "- [ ] smoke test: 失败" >> "$REPORT_FILE"
    cat "$TMP_DIR/smoke_output.txt"
fi

# ===== 步骤 6: 多平台构建 =====
log_info "[步骤 6] 多平台构建..."
export OUTPUT_DIR="$RELEASE_MOCK_DIR"
export VERSION="v0.1.0-alpha-test"
export COMMIT="test-commit"
export BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

if bash "$SCRIPT_DIR/build_release.sh" > "$TMP_DIR/build_output.txt" 2>&1; then
    log_pass "多平台构建通过"
    echo "- [x] 多平台构建: 通过" >> "$REPORT_FILE"
else
    log_fail "多平台构建失败"
    echo "- [ ] 多平台构建: 失败" >> "$REPORT_FILE"
    cat "$TMP_DIR/build_output.txt"
fi

# ===== 步骤 7: 产物完整性检查 =====
log_info "[步骤 7] 产物完整性检查..."
if bash "$SCRIPT_DIR/check_release_artifacts.sh" "$RELEASE_MOCK_DIR" > "$TMP_DIR/artifact_check_output.txt" 2>&1; then
    log_pass "产物完整性检查通过"
    echo "- [x] 产物完整性检查: 通过" >> "$REPORT_FILE"
else
    log_fail "产物完整性检查失败"
    echo "- [ ] 产物完整性检查: 失败" >> "$REPORT_FILE"
    cat "$TMP_DIR/artifact_check_output.txt"
fi

# ===== 步骤 9: install.sh Try-Run =====
log_info "[步骤 9] install.sh Try-Run..."

# install.sh 仅支持 Linux，在其他平台跳过
CURRENT_OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
if [ "$CURRENT_OS" != "linux" ]; then
    log_skip "install.sh 仅支持 Linux（当前: $CURRENT_OS）"
    echo "- [~] install.sh Try-Run: 跳过（仅支持 Linux）" >> "$REPORT_FILE"
else
    # 解压一个平台的构建产物到 mock release 目录
    MOCK_RELEASE="$TMP_DIR/mock-release"
    mkdir -p "$MOCK_RELEASE"
    tar -xzf "$RELEASE_MOCK_DIR/atria_linux_amd64.tar.gz" -C "$MOCK_RELEASE"
    cp "$RELEASE_MOCK_DIR/checksums.txt" "$MOCK_RELEASE/"

    export ATRIA_INSTALL_ROOT="$INSTALL_ROOT"
    export ATRIA_RELEASE_BASE_URL="$MOCK_RELEASE"
    export ATRIA_SKIP_SYSTEMD=1
    export ATRIA_SKIP_USER_CREATE=1
    export ATRIA_VERSION="v0.1.0-alpha-test"

    if bash "$SCRIPT_DIR/install.sh" > "$TMP_DIR/install_output.txt" 2>&1; then
        log_pass "install.sh Try-Run 通过"
        echo "- [x] install.sh Try-Run: 通过" >> "$REPORT_FILE"

        # 验证安装结果
        if [ -f "$INSTALL_ROOT/usr/local/bin/atria" ]; then
            log_pass "安装后二进制存在"
            echo "- [x] 安装后二进制: 存在" >> "$REPORT_FILE"

            # 验证 atria version
            if "$INSTALL_ROOT/usr/local/bin/atria" version > /dev/null 2>&1; then
                log_pass "安装后 atria version 可执行"
                echo "- [x] atria version: 可执行" >> "$REPORT_FILE"
            else
                log_fail "安装后 atria version 不可执行"
                echo "- [ ] atria version: 不可执行" >> "$REPORT_FILE"
            fi
        else
            log_fail "安装后二进制不存在"
            echo "- [ ] 安装后二进制: 不存在" >> "$REPORT_FILE"
        fi
    else
        log_fail "install.sh Try-Run 失败"
        echo "- [ ] install.sh Try-Run: 失败" >> "$REPORT_FILE"
        cat "$TMP_DIR/install_output.txt"
    fi
fi

# ===== 步骤 10: uninstall.sh Try-Run =====
log_info "[步骤 10] uninstall.sh Try-Run..."

export ATRIA_UNINSTALL_DRY_RUN=1

if bash "$SCRIPT_DIR/uninstall.sh" > "$TMP_DIR/uninstall_output.txt" 2>&1; then
    log_pass "uninstall.sh Try-Run 通过"
    echo "- [x] uninstall.sh Try-Run: 通过" >> "$REPORT_FILE"
else
    log_fail "uninstall.sh Try-Run 失败"
    echo "- [ ] uninstall.sh Try-Run: 失败" >> "$REPORT_FILE"
    cat "$TMP_DIR/uninstall_output.txt"
fi

# ===== 汇总 =====
echo ""
echo "=========================================="
echo "测试完成"
echo "=========================================="
echo "通过: $PASS_COUNT"
echo "失败: $FAIL_COUNT"
echo "跳过: $SKIP_COUNT"
echo ""

cat >> "$REPORT_FILE" <<EOF

## 汇总

- 通过: $PASS_COUNT
- 失败: $FAIL_COUNT
- 跳过: $SKIP_COUNT

## 说明

- 本测试不访问真实 Telegram 网络
- 本测试不访问真实 GitHub Release
- 本测试不需要 root 权限
- 所有临时文件在 tmp/ 目录下
- 真实 Telegram 登录验证需要人工执行
EOF

echo "报告已生成: $REPORT_FILE"

# 如果有失败则退出非零
if [ "$FAIL_COUNT" -gt 0 ]; then
    exit 1
fi
