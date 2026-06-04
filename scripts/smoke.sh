#!/bin/bash
# Atria 本地 Smoke Test 脚本
# 用途：验证基础启动和公开页面可访问
# 不访问 Telegram 网络，不使用真实 API 凭据

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
TEMP_DIR=$(mktemp -d)
PORT=19876
SERVER_PID=""

# 清理函数
cleanup() {
    echo "[清理] 停止服务器..."
    if [ -n "$SERVER_PID" ]; then
        kill "$SERVER_PID" 2>/dev/null || true
        wait "$SERVER_PID" 2>/dev/null || true
    fi
    echo "[清理] 删除临时目录 $TEMP_DIR"
    rm -rf "$TEMP_DIR"
    echo "[清理] 完成"
}

trap cleanup EXIT

echo "=========================================="
echo "Atria Smoke Test"
echo "=========================================="
echo "项目目录: $PROJECT_DIR"
echo "临时目录: $TEMP_DIR"
echo "端口: $PORT"
echo ""

# 编译
echo "[步骤 1] 编译..."
cd "$PROJECT_DIR"
go build -o "$TEMP_DIR/atria" ./cmd/atria 2>&1
echo "[步骤 1] 编译成功"

# 启动服务器（使用临时数据目录）
export ATRIA_HOST="127.0.0.1"
export ATRIA_PORT="$PORT"
export ATRIA_DATA_DIR="$TEMP_DIR/data"
export ATRIA_DB_DRIVER="sqlite"
export ATRIA_DB_DSN="$TEMP_DIR/data/atria.db"
export ATRIA_SESSION_DIR="$TEMP_DIR/data/sessions"
export ATRIA_LOG_DIR="$TEMP_DIR/data/logs"

echo "[步骤 2] 启动服务器..."
"$TEMP_DIR/atria" serve &
SERVER_PID=$!
sleep 3

# 检查进程是否还在运行
if ! kill -0 "$SERVER_PID" 2>/dev/null; then
    echo "[失败] 服务器未正常启动"
    exit 1
fi
echo "[步骤 2] 服务器已启动 (PID: $SERVER_PID)"

# 测试 /healthz
echo "[步骤 3] 测试 /healthz..."
HEALTHZ=$(curl -s "http://127.0.0.1:$PORT/healthz")
echo "  响应: $HEALTHZ"
if echo "$HEALTHZ" | grep -q '"status":"ok"'; then
    echo "[步骤 3] /healthz 正常"
else
    echo "[失败] /healthz 响应异常"
    exit 1
fi

# 测试 /init（未初始化时应返回 200）
echo "[步骤 4] 测试 /init..."
INIT_CODE=$(curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:$PORT/init")
echo "  状态码: $INIT_CODE"
if [ "$INIT_CODE" = "200" ]; then
    echo "[步骤 4] /init 正常（未初始化，显示初始化页面）"
else
    echo "[失败] /init 期望 200，实际 $INIT_CODE"
    exit 1
fi

# 测试 /（未初始化时应重定向到 /init）
echo "[步骤 5] 测试 /（未初始化重定向）..."
ROOT_CODE=$(curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:$PORT/")
ROOT_LOCATION=$(curl -s -o /dev/null -w "%{redirect_url}" "http://127.0.0.1:$PORT/")
echo "  状态码: $ROOT_CODE"
echo "  重定向: $ROOT_LOCATION"
if [ "$ROOT_CODE" = "302" ] || [ "$ROOT_CODE" = "303" ]; then
    echo "[步骤 5] / 正确重定向"
else
    echo "[失败] / 期望重定向，实际 $ROOT_CODE"
    exit 1
fi

# 测试 /login（未初始化时应重定向到 /init）
echo "[步骤 6] 测试 /login（未初始化重定向）..."
LOGIN_CODE=$(curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:$PORT/login")
echo "  状态码: $LOGIN_CODE"
if [ "$LOGIN_CODE" = "302" ] || [ "$LOGIN_CODE" = "303" ]; then
    echo "[步骤 6] /login 正确重定向"
else
    echo "[失败] /login 期望重定向，实际 $LOGIN_CODE"
    exit 1
fi

# 测试 /healthz JSON 包含版本信息
echo "[步骤 7] 检查版本信息..."
if echo "$HEALTHZ" | grep -q '"version"'; then
    echo "[步骤 7] 版本信息存在"
else
    echo "[失败] /healthz 缺少版本信息"
    exit 1
fi

# 检查数据目录是否创建
echo "[步骤 8] 检查数据目录..."
if [ -d "$TEMP_DIR/data" ]; then
    echo "[步骤 8] data 目录已创建"
else
    echo "[失败] data 目录未创建"
    exit 1
fi

if [ -f "$TEMP_DIR/data/atria.db" ]; then
    echo "[步骤 8] SQLite 数据库已创建"
else
    echo "[失败] SQLite 数据库未创建"
    exit 1
fi

if [ -f "$TEMP_DIR/data/secret.key" ]; then
    echo "[步骤 8] secret.key 已创建"
else
    echo "[失败] secret.key 未创建"
    exit 1
fi

# 检查 secret.key 权限
echo "[步骤 9] 检查 secret.key 权限..."
SECRET_PERMS=$(stat -c "%a" "$TEMP_DIR/data/secret.key" 2>/dev/null || stat -f "%Lp" "$TEMP_DIR/data/secret.key" 2>/dev/null || echo "unknown")
echo "  权限: $SECRET_PERMS"
if [ "$SECRET_PERMS" = "600" ]; then
    echo "[步骤 9] secret.key 权限正确 (0600)"
else
    echo "[警告] secret.key 权限为 $SECRET_PERMS，建议 0600"
fi

echo ""
echo "=========================================="
echo "Smoke Test 全部通过"
echo "=========================================="
