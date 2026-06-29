#!/usr/bin/env bash
# Atria 本地构建脚本
# 构建单二进制，前端先构建再由 go embed 嵌入
set -euo pipefail
cd "$(dirname "$0")/.."

VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null | sed 's/^v//')}"
VERSION="${VERSION:-dev}"
LDFLAGS="-s -w -X github.com/user/atria/internal/version.Version=${VERSION}"

# 构建前端
npm --prefix frontend install
npm --prefix frontend run build
touch web/static/dist/.gitkeep

mkdir -p dist
CGO_ENABLED=0 go build -trimpath -ldflags "${LDFLAGS}" -o dist/atria ./cmd/atria
echo "built dist/atria (version=${VERSION})"
