#!/bin/bash
# HubTerm 中心服务启动脚本
# 用法: ./scripts/start-center.sh [--build] [--config path/to/config.yaml]

set -e

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
BIN="$ROOT/hubterm-center"

yaml_value() {
  local key="$1" file="$2"
  grep -E "^[[:space:]]*${key}:" "$file" | head -1 | sed -E 's/^[^:]*:[[:space:]]*"?([^"]*)"?.*/\1/'
}

FORCE_BUILD=false
if [[ "${1:-}" == "--build" ]]; then
  FORCE_BUILD=true
  shift
fi

if [[ "$FORCE_BUILD" == true || ! -x "$BIN" ]]; then
  echo "🔨 Building hubterm-center..."
  CGO_ENABLED=0 go build -o "$BIN" ./cmd/center
fi

CONFIG_ARG=""
if [[ "$1" == "--config" && -n "${2:-}" ]]; then
  CONFIG_ARG="--config $2"
  shift 2
elif [[ -f "$ROOT/config.yaml" ]]; then
  CONFIG_ARG="--config $ROOT/config.yaml"
fi

PORT=8080
if [[ -f "$ROOT/config.yaml" ]]; then
  [[ -z "${JWT_SECRET:-}" ]] && export JWT_SECRET="$(yaml_value jwt_secret "$ROOT/config.yaml")"
  [[ -z "${ADMIN_PASSWORD:-}" ]] && export ADMIN_PASSWORD="$(yaml_value admin_password "$ROOT/config.yaml")"
  PORT="$(yaml_value port "$ROOT/config.yaml")"
  PORT="${PORT:-8080}"
fi

echo "🚀 Starting HubTerm Center on :${PORT}..."
exec "$BIN" $CONFIG_ARG
