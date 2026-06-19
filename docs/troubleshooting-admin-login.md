# Admin 登录故障：修改环境变量后仍返回 401

## 现象

- `GET /api/health` 返回 200，数据库状态正常。
- 使用部署时配置的 admin 密码调用 `POST /api/auth/login`，接口返回 401。
- 修改 `ADMIN_PASSWORD` 并重启旧版本后，登录结果不变。

## 根因

旧版 `EnsureAdminExists` 只在数据库中不存在管理员时读取
`ADMIN_PASSWORD` 并创建账号。一旦持久化数据库中已经存在 admin，后续修改环境
变量不会更新 bcrypt 密码哈希。

部署时还需要区分源码目录和实际运行目录。修改 NAS 中的源码不会自动替换已经运行
的二进制；必须重新构建并重启实际监听端口的进程。本次故障中，8080 端口运行的是
`/code/hubterm/hubterm-center`，而不是直接运行 NAS 源码。

## 当前行为

- 当 `ADMIN_PASSWORD` 已配置时，中心服务每次启动都会将它同步到用户名为
  `admin` 的内置账号，并确保该账号角色为 `admin`。
- 当 `ADMIN_PASSWORD` 未配置且 admin 已存在时，保留数据库中的原密码。
- 首次启动且未配置密码时，仍生成一次性随机密码并输出到启动日志。
- 登录失败日志只记录用户名和客户端 IP，不再记录密码、密码十六进制或哈希信息。

因此，在 Docker、systemd 或手工启动脚本中配置 `ADMIN_PASSWORD` 时，该环境
变量就是内置 admin 密码的来源。修改后必须重新启动中心服务。

## 排查步骤

1. 确认服务和数据库健康：

   ```bash
   curl -fsS http://127.0.0.1:8080/api/health
   ```

2. 确认实际监听 8080 的进程及运行目录：

   ```bash
   ss -ltnp 'sport = :8080'
   readlink -f /proc/<PID>/cwd
   readlink -f /proc/<PID>/exe
   ```

3. 确认启动环境包含 `ADMIN_PASSWORD` 和 `JWT_SECRET`，但不要把变量值输出到
   日志或终端历史。

4. 从最新源码重新构建，并在保留数据库的前提下替换二进制、重启服务。

5. 再次调用登录接口确认返回 200。验证时不要输出 JWT token。

## 注意事项

- 不需要删除 SQLite 数据库或 Docker volume 来重置 admin 密码。
- `docker-compose.yml` 要求显式提供 `ADMIN_PASSWORD` 和 `JWT_SECRET`。
- 若使用手工后台进程，环境变量只存在于该进程生命周期内；机器重启后应由
  systemd、Docker Compose 或其他受管服务重新注入。
