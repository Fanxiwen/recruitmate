#!/usr/bin/env bash
# 本地集成验证：迁移 → 种子 → 启动 API → 冒烟测试 → 停止
# 用法：bash scripts/verify.sh
set -euo pipefail
cd "$(dirname "$0")/.."

echo "==> 1/4 执行数据库迁移"
make api-migrate

echo "==> 2/4 写入种子数据"
make api-seed

echo "==> 3/4 启动 API（后台）"
(cd apps/api && go run ./cmd/server > /tmp/recruitmate-api.log 2>&1 & echo $! > /tmp/recruitmate-api.pid)
# 等待就绪
for _ in $(seq 1 30); do
  if curl -sf http://localhost:8080/healthz >/dev/null 2>&1; then break; fi
  sleep 1
done
curl -sf http://localhost:8080/healthz >/dev/null || { echo "API 启动失败，日志："; tail -30 /tmp/recruitmate-api.log; exit 1; }
echo "    API 已就绪"

echo "==> 4/4 冒烟测试"
if bash scripts/smoke.sh; then
  echo "✅ 集成验证通过"
else
  echo "❌ 集成验证失败，API 日志尾部："
  tail -50 /tmp/recruitmate-api.log
  exit 1
fi

if [ -f /tmp/recruitmate-api.pid ]; then kill "$(cat /tmp/recruitmate-api.pid)" 2>/dev/null || true; fi
