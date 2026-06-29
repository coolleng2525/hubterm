  Prioritizing Tool Usage
  已经为你将版本号提升至  1.1.8  并推送了新的代码和 Tag ( tabby-hubterm-v1.1.8 )。

  这将会触发 GitHub Actions 的两条流水线：

  1. HubTerm-Plugin-Release: 为插件打包。
  2. Package-Build: 构建完整的 Tabby 应用，包括 macOS、Linux 和 Windows 版（现在 macOS 版由于我已经排除了签名环节的报错，所以能够成功编译和打包）。

  如何在 Mac 上使用：
  等待大概 15~25 分钟，等 GitHub Actions 跑完之后，你就可以在  tabby-hubterm-v1.1.8  的 Release 页面看到 macOS 的  .dmg  或  .zip  下载链接。
  下载并拖入“应用程序 (Applications)”文件夹后，由于我们的安装包没有使用苹果开发者签名，首次打开时系统（Gatekeeper）可能会拦截并提示“App
  已损坏，无法打开”或“无法验证开发者”。
  你需要：

  1. 打开 Mac 上的 终端 (Terminal)
  2. 运行以下命令移除系统隔离属性：
    sudo xattr -cr /Applications/Tabby.app
    (注意：如果你的应用名称不是  Tabby.app ，请替换成实际名称)
  3. 之后就可以正常打开并使用该程序了。

▸ Thought for 5s, 253 tokens
  Prioritizing Tool Usage
  推送成功！代码和新的 Tag  tabby-hubterm-v1.1.8  都已经成功推送到 GitHub 仓库中了。



  # 部署二进制
python3 scripts/deploy-agent-t2.py deploy
# 部署并启动
python3 scripts/deploy-agent-t2.py deploy --start
# 启动 / 停止 / 重启 / 状态
python3 scripts/deploy-agent-t2.py start 10.223.40.20 10.223.40.21
python3 scripts/deploy-agent-t2.py stop  10.223.40.20 10.223.40.21
python3 scripts/deploy-agent-t2.py restart 10.223.40.20 10.223.40.21
python3 scripts/deploy-agent-t2.py status 10.223.40.21
python3 scripts/deploy-agent-t2.py status 10.223.40.21