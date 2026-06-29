#!/bin/bash
# HubTerm 节点代理启动脚本
# 用法: ./scripts/start-agent.sh [--build] [--center URL] [--name NAME] [--data DIR]

set -e

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
BIN="$ROOT/hubterm-agent"

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
  echo "🔨 Building hubterm-agent..."
  CGO_ENABLED=0 go build -o "$BIN" ./cmd/agent
fi

CENTER_URL="${HUBTERM_CENTER_URL:-}"
DATA_DIR="$ROOT/data/agent"
NAME=""
DOMAIN=""
EXTRA_ARGS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --center)
      CENTER_URL="$2"
      shift 2
      ;;
    --name)
      NAME="$2"
      shift 2
      ;;
    --data)
      DATA_DIR="$2"
      shift 2
      ;;
    --domain)
      DOMAIN="$2"
      shift 2
      ;;
    *)
      EXTRA_ARGS+=("$1")
      shift
      ;;
  esac
done

if [[ -z "$CENTER_URL" && -n "$DOMAIN" ]]; then
  :
elif [[ -z "$CENTER_URL" && -f "$ROOT/config.yaml" ]]; then
  PORT="$(yaml_value port "$ROOT/config.yaml")"
  PORT="${PORT:-8080}"
  CENTER_URL="http://127.0.0.1:${PORT}"
fi

if [[ -z "$CENTER_URL" && -z "$DOMAIN" ]]; then
  echo "错误: 请指定中心地址 (--center URL、--domain、HUBTERM_CENTER_URL 或 config.yaml)" >&2
  exit 1
fi

mkdir -p "$DATA_DIR"

ARGS=(--data "$DATA_DIR")
[[ -n "$CENTER_URL" ]] && ARGS+=(--center "$CENTER_URL")
[[ -n "$NAME" ]] && ARGS+=(--name "$NAME")
[[ -n "$DOMAIN" ]] && ARGS+=(--domain "$DOMAIN")
ARGS+=("${EXTRA_ARGS[@]}")

echo "🚀 Starting HubTerm Agent -> ${CENTER_URL:-domain:$DOMAIN}..."
exec "$BIN" "${ARGS[@]}"
