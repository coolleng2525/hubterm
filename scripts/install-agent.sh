#!/bin/bash
# ==============================================================================
# HubTerm Agent 一键安装脚本 (Linux)
# 从 GitHub Releases 下载指定版本的 Agent 二进制程序，并注册为 Systemd 服务运行。
#
# 用法:
#   sudo ./install-agent.sh --center http://192.168.1.55:8080 [--version v1.13.2]
# ==============================================================================

set -e

# 默认设置
VERSION="v1.13.2"
REPO="coolleng2525/hubterm"
CENTER_URL=""
NODE_NAME=""
RUN_USER=$(logname || echo $USER)

show_help() {
    echo "用法: $0 --center <中心地址> [选项]"
    echo "选项:"
    echo "  -c, --center <url>      HubTerm 中心地址 (必填，例如 http://192.168.1.55:8080)"
    echo "  -v, --version <tag>     安装的 Agent 版本号 (默认为 $VERSION)"
    echo "  -n, --name <name>       自定义节点显示名称 (默认为主机 hostname)"
    echo "  -u, --user <user>       运行 Agent 服务的系统用户 (默认为 $RUN_USER)"
    echo "  -h, --help              显示帮助信息"
}

# 解析参数
while [[ $# -gt 0 ]]; do
  case "$1" in
    -c|--center)
      CENTER_URL="$2"
      shift 2
      ;;
    -v|--version)
      VERSION="$2"
      shift 2
      ;;
    -n|--name)
      NODE_NAME="$2"
      shift 2
      ;;
    -u|--user)
      RUN_USER="$2"
      shift 2
      ;;
    -h|--help)
      show_help
      exit 0
      ;;
    *)
      echo "未知参数: $1" >&2
      show_help
      exit 1
      ;;
  esac
done

if [ -z "$CENTER_URL" ]; then
  echo "错误: --center 地址是必填项。" >&2
  show_help
  exit 1
fi

# 检查权限
if [ "$EUID" -ne 0 ]; then
  echo "错误: 必须以 root 权限运行此脚本 (例如使用 sudo)。" >&2
  exit 1
fi

# 检测系统架构
ARCH=$(uname -m)
if [ "$ARCH" = "x86_64" ]; then
    ARCH_SUFFIX="amd64"
elif [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then
    ARCH_SUFFIX="arm64"
else
    echo "不支持的 CPU 架构: $ARCH" >&2
    exit 1
fi

# 计算下载地址
VER_NUM=$(echo "$VERSION" | sed 's/^v//')
FILENAME="hubterm_${VER_NUM}_linux_${ARCH_SUFFIX}.tar.gz"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${FILENAME}"
TMP_DIR="/tmp/hubterm-install"

echo "============================================="
echo "📥 开始安装 HubTerm Agent..."
echo "   - CPU 架构:  Linux/${ARCH_SUFFIX}"
echo "   - 安装版本:  ${VERSION}"
echo "   - 中心地址:  ${CENTER_URL}"
echo "   - 运行用户:  ${RUN_USER}"
echo "============================================="

# 创建临时工作目录
rm -rf "$TMP_DIR" && mkdir -p "$TMP_DIR"
cd "$TMP_DIR"

# 下载归档包
echo "⏳ 正在从 GitHub 下载 ${FILENAME}..."
curl -L -o archive.tar.gz "$DOWNLOAD_URL"
tar -xzf archive.tar.gz

if [ ! -f "hubterm-agent" ]; then
  echo "错误: 归档包中未找到 hubterm-agent 主程序。" >&2
  exit 1
fi

# 部署至运行目录
INSTALL_DIR="/opt/hubterm-agent"
echo "⚙️ 正在部署程序至 ${INSTALL_DIR}..."
mkdir -p "${INSTALL_DIR}/bin" "${INSTALL_DIR}/data"
cp hubterm-agent "${INSTALL_DIR}/bin/hubterm-agent"
chmod +x "${INSTALL_DIR}/bin/hubterm-agent"
chown -R "${RUN_USER}:${RUN_USER}" "${INSTALL_DIR}"

# 写入 Systemd 服务配置文件
SERVICE_FILE="/etc/systemd/system/hubterm-agent.service"
echo "⚙️ 正在生成 Systemd 服务配置文件 ${SERVICE_FILE}..."

NAME_ARG=""
if [ -n "$NODE_NAME" ]; then
  NAME_ARG=" --name ${NODE_NAME}"
fi

tee "$SERVICE_FILE" > /dev/null <<EOF
[Unit]
Description=HubTerm Agent Service
After=network.target

[Service]
Type=simple
User=${RUN_USER}
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/bin/hubterm-agent --center ${CENTER_URL} --data ${INSTALL_DIR}/data${NAME_ARG}
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# 加载配置并启动服务
echo "🚀 启动并注册 hubterm-agent 系统服务..."
systemctl daemon-reload
systemctl enable hubterm-agent
systemctl restart hubterm-agent

sleep 2
systemctl status hubterm-agent --no-pager -l

echo "============================================="
echo "🎉 HubTerm Agent 一键安装完成！"
echo "============================================="

# 清理临时文件
rm -rf "$TMP_DIR"
