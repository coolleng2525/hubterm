#!/bin/bash
set -e

# ============================================
# Tabby HubTerm 一键安装脚本
# 自动检测芯片、下载 DMG、ad-hoc 签名、安装
#
# 用法:
#   ./install-tabby-hubterm.sh                          # 使用默认版本
#   ./install-tabby-hubterm.sh tabby-hubterm-v1.1.10    # 指定版本 tag
# ============================================

REPO="coolleng2525/tabby"
TAG="${1:-tabby-hubterm-v1.1.9}"

# 从 tag 提取版本号 (tabby-hubterm-v1.2.3 → 1.2.3)
VERSION=$(echo "$TAG" | sed 's/.*-v//')

# 1. 检测芯片架构
ARCH=$(uname -m)
if [ "$ARCH" = "arm64" ]; then
    DMG="tabby-${VERSION}-macos-arm64.dmg"
elif [ "$ARCH" = "x86_64" ]; then
    DMG="tabby-${VERSION}-macos-x86_64.dmg"
else
    echo "不支持的架构: $ARCH"
    exit 1
fi

DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${TAG}/${DMG}"
DMG_PATH="/tmp/${DMG}"

echo "架构: $ARCH"
echo "下载: $DOWNLOAD_URL"

# 2. 下载 DMG
curl -L -o "$DMG_PATH" "$DOWNLOAD_URL"
echo "下载完成"

# 3. 挂载 DMG
VOLUME=$(hdiutil attach "$DMG_PATH" -nobrowse | grep /Volumes/ | awk '{print $3}')
echo "已挂载: $VOLUME"

# 4. 复制 app 到用户 Applications
rm -rf ~/Applications/Tabby.app
cp -R "$VOLUME/Tabby.app" ~/Applications/

# 5. 卸载 DMG
hdiutil detach "$VOLUME" -quiet

# 6. ad-hoc 签名（修复 macOS 26 代码签名无效崩溃）
codesign --force --deep --sign - ~/Applications/Tabby.app
echo "ad-hoc 签名完成"

# 7. 移除隔离属性
xattr -dr com.apple.quarantine ~/Applications/Tabby.app 2>/dev/null || true

# 8. 清理
rm -f "$DMG_PATH"

# 9. 启动
open ~/Applications/Tabby.app
echo "Tabby HubTerm 安装完成，已启动"
