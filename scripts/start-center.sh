#!/bin/bash
# HubTerm 中心服务启动脚本
# 用法:
#   ./scripts/start-center.sh [--local] [--build] [--config path/to/config.yaml]
#   ./scripts/start-center.sh --docker [--build] [--config path/to/config.yaml]

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
BIN="$ROOT/hubterm-center"

MODE="${HUBTERM_START_MODE:-local}"
FORCE_BUILD=false
CONFIG_FILE=""

yaml_value() {
  local key="$1" file="$2"
  grep -E "^[[:space:]]*${key}:" "$file" | head -1 | sed -E 's/^[^:]*:[[:space:]]*"?([^"]*)"?.*/\1/'
}

usage() {
  echo "用法: $0 [--local|--docker] [--build] [--config path/to/config.yaml]" >&2
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --local)
      MODE="local"
      shift
      ;;
    --docker)
      MODE="docker"
      shift
      ;;
    --build)
      FORCE_BUILD=true
      shift
      ;;
    --config)
      if [[ -z "${2:-}" ]]; then
        usage
        exit 1
      fi
      CONFIG_FILE="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      usage
      exit 1
      ;;
  esac
done

if [[ -z "$CONFIG_FILE" && -f "$ROOT/config.yaml" ]]; then
  CONFIG_FILE="$ROOT/config.yaml"
fi

load_config_env() {
  if [[ -f "$ROOT/.env" ]]; then
    set -a
    # shellcheck disable=SC1091
    source "$ROOT/.env"
    set +a
  fi

  if [[ -n "$CONFIG_FILE" && -f "$CONFIG_FILE" ]]; then
    [[ -z "${JWT_SECRET:-}" ]] && export JWT_SECRET="$(yaml_value jwt_secret "$CONFIG_FILE")"
    [[ -z "${ADMIN_PASSWORD:-}" ]] && export ADMIN_PASSWORD="$(yaml_value admin_password "$CONFIG_FILE")"
    [[ -z "${PORT:-}" ]] && export PORT="$(yaml_value port "$CONFIG_FILE")"
  fi

  PORT="${PORT:-8080}"
  export PORT
}

require_center_env() {
  if [[ -z "${JWT_SECRET:-}" ]]; then
    echo "错误: 未设置 JWT_SECRET。请在 .env、config.yaml 或环境变量中配置。" >&2
    exit 1
  fi
  if [[ -z "${ADMIN_PASSWORD:-}" ]]; then
    echo "错误: 未设置 ADMIN_PASSWORD。请在 .env、config.yaml 或环境变量中配置。" >&2
    exit 1
  fi
  if ! [[ "$PORT" =~ ^[0-9]+$ ]]; then
    echo "错误: 无效的 PORT: $PORT" >&2
    exit 1
  fi
  export JWT_SECRET ADMIN_PASSWORD PORT
}

wait_or_kill() {
  local label="$1"
  shift
  local pids=("$@")
  local pid

  [[ ${#pids[@]} -eq 0 ]] && return

  echo "🛑 Stopping existing ${label}: ${pids[*]}"
  kill "${pids[@]}" 2>/dev/null || true

  for _ in {1..20}; do
    local alive=()
    for pid in "${pids[@]}"; do
      if kill -0 "$pid" 2>/dev/null; then
        alive+=("$pid")
      fi
    done
    if [[ ${#alive[@]} -eq 0 ]]; then
      return
    fi
    sleep 0.2
  done

  echo "⚠️  Force stopping existing ${label}: ${pids[*]}"
  kill -9 "${pids[@]}" 2>/dev/null || true
}

stop_local_center() {
  local pids=()
  local pid

  if ! command -v pgrep >/dev/null 2>&1; then
    return
  fi

  while IFS= read -r pid; do
    [[ -n "$pid" && "$pid" != "$$" ]] && pids+=("$pid")
  done < <(pgrep -f "hubterm-center" || true)

  wait_or_kill "HubTerm Center process" "${pids[@]}"
}

docker_compose() {
  export DOCKER_BUILDKIT=1
  export COMPOSE_DOCKER_CLI_BUILD=1
  if docker compose version &>/dev/null; then
    docker compose "$@"
  elif command -v docker-compose >/dev/null 2>&1; then
    docker-compose "$@"
  else
    echo "错误: 未找到 docker compose，请先安装 Docker。" >&2
    exit 1
  fi
}

docker_available() {
  command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1
}

stop_docker_center() {
  local required="${1:-false}"
  local containers=()
  local container

  if ! docker_available; then
    if [[ "$required" == true ]]; then
      echo "错误: Docker 不可用，请确认 Docker 已安装并正在运行。" >&2
      exit 1
    fi
    return
  fi

  docker_compose down --remove-orphans || true

  while IFS= read -r container; do
    [[ -n "$container" ]] && containers+=("$container")
  done < <(docker ps --filter "publish=${PORT}" -q 2>/dev/null || true)

  if [[ ${#containers[@]} -gt 0 ]]; then
    echo "🛑 Stopping containers publishing :${PORT}: ${containers[*]}"
    docker stop "${containers[@]}" >/dev/null
  fi
}

start_local() {
  if [[ "$FORCE_BUILD" == true || ! -x "$BIN" ]]; then
    echo "🔨 Building hubterm-center..."
    CGO_ENABLED=0 go build -o "$BIN" ./cmd/center
  fi

  local args=()
  if [[ -n "$CONFIG_FILE" ]]; then
    args+=(--config "$CONFIG_FILE")
  fi

  stop_local_center
  stop_docker_center false

  echo "🚀 Starting HubTerm Center locally on :${PORT}..."
  exec "$BIN" "${args[@]}"
}

start_docker() {
  require_center_env
  stop_local_center
  stop_docker_center true

  echo "🚀 Starting HubTerm Center with Docker on :${PORT}..."
  if [[ "$FORCE_BUILD" == true ]]; then
    docker_compose up -d --build
  else
    docker_compose up -d
  fi
  docker_compose ps
}

load_config_env

case "$MODE" in
  local)
    start_local
    ;;
  docker)
    start_docker
    ;;
  *)
    echo "错误: 无效启动模式: $MODE" >&2
    usage
    exit 1
    ;;
esac
