#!/usr/bin/env bash
# build_nas_image.sh 交叉编译 + 在 NAS 上打镜像（NAS 拉不到 golang 基础镜像时的可复现路径）。
#
# 背景：fnOS 的 docker 走 docker.fnnas.com 镜像站，`# syntax=docker/dockerfile:1`
# 会 401（前端镜像拉不到），官方 golang:1.23-alpine 也不在本地缓存里。因此改为：
#   1) 本机 CGO_ENABLED=0 交叉编译 linux/amd64 静态二进制；
#   2) 以**已在 NAS 上的运行时镜像**为基底，只替换 /app/wb2api（Dockerfile.deploy）。
#
# 用法：
#   scripts/build_nas_image.sh wb2api-fork:2026-10-24-expert-task
#
# 环境变量：
#   SSH_HOST   默认 fnos
#   NAS_BUILD  默认 /vol1/1000/wb2api-build（构建上下文，与生产挂载目录分开）
#   BASE_IMAGE 默认 wb2api-fork:2026-10-23-3（现有可运行的运行时镜像）
set -euo pipefail

TAG="${1:?用法: build_nas_image.sh <镜像 tag>}"
SSH_HOST="${SSH_HOST:-fnos}"
NAS_BUILD="${NAS_BUILD:-/vol1/1000/wb2api-build}"
BASE_IMAGE="${BASE_IMAGE:-wb2api-fork:2026-10-23-3}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$(mktemp -d)/wb2api.bin"

echo "[1/4] 交叉编译 linux/amd64"
(cd "$ROOT" && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags="-s -w" -o "$OUT" ./cmd/server)

echo "[2/4] 同步源码与二进制到 $SSH_HOST:$NAS_BUILD"
ssh "$SSH_HOST" "mkdir -p '$NAS_BUILD'"
rsync -a --delete \
  --exclude '.git' --exclude 'auths' --exclude 'data' --exclude 'config.json' \
  --exclude 'wb2api' --exclude 'wb2api.bin' --exclude 'Dockerfile.deploy' \
  "$ROOT"/ "$SSH_HOST:$NAS_BUILD/"
scp -q "$OUT" "$SSH_HOST:$NAS_BUILD/wb2api.bin"

echo "[3/4] 生成 Dockerfile.deploy 并构建（classic builder，避免 BuildKit 前端拉取）"
ssh "$SSH_HOST" "cat > '$NAS_BUILD/Dockerfile.deploy' <<'EOF'
FROM $BASE_IMAGE
USER root
COPY wb2api.bin /app/wb2api
RUN chmod 755 /app/wb2api
USER app
EOF
cd '$NAS_BUILD' && DOCKER_BUILDKIT=0 docker build -f Dockerfile.deploy -t '$TAG' ."

echo "[4/4] 完成：$TAG"
echo "上线：把生产目录 docker-compose.yml 的 image 改成 $TAG 后 docker compose up -d"
rm -f "$OUT"
