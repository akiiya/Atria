#!/usr/bin/env bash
# Atria 发布构建脚本
# 串联: go vet → go test → 前端构建 → 多平台打包 → SHA256SUMS
# 不真正发版，只产出 dist/ 下的压缩包
set -euo pipefail
cd "$(dirname "$0")/.."

VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null | sed 's/^v//')}"
VERSION="${VERSION:-dev}"
LDFLAGS="-s -w -X github.com/user/atria/internal/version.Version=${VERSION}"

EXTRA_FILES=(README.md LICENSE)

echo "=========================================="
echo "Atria Release 构建"
echo "版本: ${VERSION}"
echo "=========================================="

go vet ./...
go test ./...

# 构建前端
npm --prefix frontend install
npm --prefix frontend run build
touch web/static/dist/.gitkeep

rm -rf dist && mkdir -p dist

package() {
  local goos="$1" goarch="$2" ext="$3" archive="$4"
  local bin="atria${ext}"
  local stage="dist/stage_${goos}_${goarch}"
  rm -rf "${stage}" && mkdir -p "${stage}"

  echo "构建 ${goos}/${goarch}..."
  CGO_ENABLED=0 GOOS="${goos}" GOARCH="${goarch}" \
    go build -trimpath -ldflags "${LDFLAGS}" -o "${stage}/${bin}" ./cmd/atria

  for f in "${EXTRA_FILES[@]}"; do
    mkdir -p "${stage}/$(dirname "${f}")"
    cp "${f}" "${stage}/${f}"
  done

  case "${archive}" in
    tar.gz) tar -C "${stage}" -czf "dist/atria_${VERSION}_${goos}_${goarch}.tar.gz" . ;;
    zip)
      if command -v zip >/dev/null 2>&1; then
        (cd "${stage}" && zip -qr "../atria_${VERSION}_${goos}_${goarch}.zip" .)
      else
        echo "  警告: zip 不可用，使用 tar.gz 替代"
        tar -C "${stage}" -czf "dist/atria_${VERSION}_${goos}_${goarch}.tar.gz" .
      fi
      ;;
  esac
  rm -rf "${stage}"
  echo "  完成: atria_${VERSION}_${goos}_${goarch}.${archive}"
}

package linux   amd64 ""     tar.gz
package linux   arm64 ""     tar.gz
package windows amd64 .exe   zip
package darwin  amd64 ""     tar.gz
package darwin  arm64 ""     tar.gz

echo ""
echo "生成 SHA256SUMS..."
(cd dist && (sha256sum atria_* 2>/dev/null || shasum -a 256 atria_*) > SHA256SUMS)

echo ""
echo "=========================================="
echo "构建完成: ${VERSION}"
echo "=========================================="
ls -1 dist
