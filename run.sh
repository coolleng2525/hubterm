#!/bin/bash
# HubTerm 一键部署脚本
#
# 用法:
#   ./run.sh              # 构建并启动 (docker compose up -d --build)
#   ./run.sh stop         # 停止服务
#   ./run.sh restart      # 重启服务
#   ./run.sh logs         # 查看日志
#   ./run.sh status       # 查看运行状态
#
# 配置优先级: 环境变量 > .env > config.yaml

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

yaml_value() {
  local key="$1" file="$2"
  grep -E "^[[:space:]]*${key}:" "$file" | head -1 | sed -E 's/^[^:]*:[[:space:]]*"?([^"]*)"?.*/\1/'
}

load_env() {
  if [[ -f .env ]]; then
    set -a
    # shellcheck disable=SC1091
    source .env
    set +a
  fi

  if [[ -f config.yaml ]]; then
    if [[ -z "${JWT_SECRET:-}" ]]; then
      JWT_SECRET="$(yaml_value jwt_secret config.yaml)"
      export JWT_SECRET
    fi
    if [[ -z "${ADMIN_PASSWORD:-}" ]]; then
      ADMIN_PASSWORD="$(yaml_value admin_password config.yaml)"
      export ADMIN_PASSWORD
    fi
    if [[ -z "${PORT:-}" ]]; then
      PORT="$(yaml_value port config.yaml)"
    fi
  fi

  PORT="${PORT:-8080}"
  export PORT
}

require_env() {
  if [[ -z "${JWT_SECRET:-}" ]]; then
    echo "错误: 未设置 JWT_SECRET。请在 .env、config.yaml 或环境变量中配置。" >&2
    exit 1
  fi
  if [[ -z "${ADMIN_PASSWORD:-}" ]]; then
    echo "错误: 未设置 ADMIN_PASSWORD。请在 .env、config.yaml 或环境变量中配置。" >&2
    exit 1
  fi
  export JWT_SECRET ADMIN_PASSWORD

  if ! [[ "$PORT" =~ ^[0-9]+$ ]]; then
    echo "错误: 无效的 PORT: $PORT" >&2
    exit 1
  fi
}

docker_compose() {
  export DOCKER_BUILDKIT=1
  export COMPOSE_DOCKER_CLI_BUILD=1
  if docker compose version &>/dev/null; then
    docker compose "$@"
  elif command -v docker-compose &>/dev/null; then
    docker-compose "$@"
  else
    echo "错误: 未找到 docker compose，请先安装 Docker。" >&2
    exit 1
  fi
}

cmd="${1:-up}"

load_env

case "$cmd" in
  up|start|deploy)
    require_env
    echo "🚀 构建并启动 HubTerm..."
    docker_compose up -d --build
    echo ""
    echo "✅ HubTerm 已启动: http://localhost:${PORT}"
    docker_compose ps
    ;;
  stop|down)
    docker_compose down
    echo "✅ HubTerm 已停止"
    ;;
  restart)
    require_env
    docker_compose down
    docker_compose up -d --build
    echo "✅ HubTerm 已重启: http://localhost:${PORT}"
    ;;
  logs)
    docker_compose logs -f --tail=100
    ;;
  status|ps)
    docker_compose ps
    ;;
  *)
    echo "用法: $0 [up|stop|restart|logs|status]" >&2
    exit 1
    ;;
esac
