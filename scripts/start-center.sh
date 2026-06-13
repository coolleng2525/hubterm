#!/bin/bash
# HubTerm 中心服务启动脚本
# 用法: ./scripts/start-center.sh [--config path/to/config.yaml]

set -e

cd "$(dirname "$0")/.."

export JWT_SECRET="***"
export ADMIN_PASSWORD="***"

CONFIG_ARG=""
if [ "$1" = "--config" ] && [ -n "$2" ]; then
    CONFIG_ARG="--config $2"
    shift 2
fi

echo "🚀 Starting HubTerm Center on :8080..."
cd cmd/center
go run main.go $CONFIG_ARG
