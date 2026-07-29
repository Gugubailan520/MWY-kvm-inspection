#!/usr/bin/env bash
# 在 Linux 构建主机上执行。Agent 依赖 libpcap，必须先安装：
#   CentOS/RHEL:  yum install -y libpcap-devel gcc
#   Debian/Ubuntu: apt-get install -y libpcap-dev gcc
set -euo pipefail

VERSION=${VERSION:-0.1.0}
OUT=${OUT:-bin}
mkdir -p "$OUT"

echo "==> building server (linux/amd64)"
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" \
    -o "$OUT/server" ./Server/cmd/server

echo "==> building agent (linux/amd64, requires libpcap)"
GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -trimpath -ldflags "-s -w" \
    -o "$OUT/agent" ./Agent/cmd/agent

echo "==> done: $OUT/server, $OUT/agent"
ls -lh "$OUT"
