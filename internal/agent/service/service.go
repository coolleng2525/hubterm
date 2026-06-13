// Package service provides system service installation for the agent.
package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Install 安装为系统服务
// name 服务名称，desc 服务描述，execPath 可执行文件路径。
// 支持 Linux (systemd)、macOS (launchd)、Windows (Windows Service)。
func Install(name, desc, execPath string) error {
	absPath, err := filepath.Abs(execPath)
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}

	switch runtime.GOOS {
	case "linux":
		return installSystemd(name, desc, absPath)
	case "darwin":
		return installLaunchd(name, desc, absPath)
	case "windows":
		return installWindowsService(name, desc, absPath)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// Uninstall 卸载系统服务
func Uninstall(name string) error {
	switch runtime.GOOS {
	case "linux":
		return uninstallSystemd(name)
	case "darwin":
		return uninstallLaunchd(name)
	case "windows":
		return uninstallWindowsService(name)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// Status 查询服务状态
// 返回 (是否运行, 错误)
func Status(name string) (bool, error) {
	switch runtime.GOOS {
	case "linux":
		return statusSystemd(name)
	case "darwin":
		return statusLaunchd(name)
	case "windows":
		return statusWindowsService(name)
	default:
		return false, fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// installSystemd 安装 systemd 服务
func installSystemd(name, desc, execPath string) error {
	unit := fmt.Sprintf(`[Unit]
Description=%s
After=network.target

[Service]
Type=simple
ExecStart=%s
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
`, desc, execPath)

	unitPath := fmt.Sprintf("/etc/systemd/system/%s.service", name)
	if err := os.WriteFile(unitPath, []byte(unit), 0644); err != nil {
		return fmt.Errorf("write systemd unit: %w", err)
	}

	cmds := [][]string{
		{"systemctl", "daemon-reload"},
		{"systemctl", "enable", name},
		{"systemctl", "start", name},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(out)), err)
		}
	}
	return nil
}

// uninstallSystemd 卸载 systemd 服务
func uninstallSystemd(name string) error {
	cmds := [][]string{
		{"systemctl", "stop", name},
		{"systemctl", "disable", name},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		_ = cmd.Run() // 忽略 stop/disable 错误
	}

	unitPath := fmt.Sprintf("/etc/systemd/system/%s.service", name)
	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove unit file: %w", err)
	}

	cmd := exec.Command("systemctl", "daemon-reload")
	return cmd.Run()
}

// statusSystemd 查询 systemd 服务状态
func statusSystemd(name string) (bool, error) {
	cmd := exec.Command("systemctl", "is-active", name)
	out, err := cmd.Output()
	if err != nil {
		return false, nil // 服务不存在或未运行
	}
	return strings.TrimSpace(string(out)) == "active", nil
}

// installLaunchd 安装 launchd 服务 (macOS)
func installLaunchd(name, desc, execPath string) error {
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>KeepAlive</key>
	<true/>
	<key>RunAtLoad</key>
	<true/>
	<key>StandardOutPath</key>
	<string>/tmp/%s.stdout.log</string>
	<key>StandardErrorPath</key>
	<string>/tmp/%s.stderr.log</string>
</dict>
</plist>
`, name, execPath, name, name)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}

	plistPath := filepath.Join(homeDir, "Library", "LaunchAgents", fmt.Sprintf("%s.plist", name))
	if err := os.MkdirAll(filepath.Dir(plistPath), 0755); err != nil {
		return fmt.Errorf("create LaunchAgents dir: %w", err)
	}
	if err := os.WriteFile(plistPath, []byte(plist), 0644); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}

	cmd := exec.Command("launchctl", "load", plistPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl load: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// uninstallLaunchd 卸载 launchd 服务
func uninstallLaunchd(name string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}

	plistPath := filepath.Join(homeDir, "Library", "LaunchAgents", fmt.Sprintf("%s.plist", name))

	cmd := exec.Command("launchctl", "unload", plistPath)
	_ = cmd.Run()

	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove plist: %w", err)
	}
	return nil
}

// statusLaunchd 查询 launchd 服务状态
func statusLaunchd(name string) (bool, error) {
	cmd := exec.Command("launchctl", "list", name)
	out, err := cmd.Output()
	if err != nil {
		return false, nil
	}
	return !strings.Contains(string(out), "\"PID\" = 0"), nil
}

// installWindowsService 安装 Windows 服务
func installWindowsService(name, desc, execPath string) error {
	// 使用 sc.exe 创建服务
	cmd := exec.Command("sc", "create", name,
		"binPath=", execPath,
		"start=", "auto",
		"DisplayName=", desc,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("sc create: %s: %w", strings.TrimSpace(string(out)), err)
	}

	cmd = exec.Command("sc", "start", name)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("sc start: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// uninstallWindowsService 卸载 Windows 服务
func uninstallWindowsService(name string) error {
	cmd := exec.Command("sc", "stop", name)
	_ = cmd.Run()

	cmd = exec.Command("sc", "delete", name)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("sc delete: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// statusWindowsService 查询 Windows 服务状态
func statusWindowsService(name string) (bool, error) {
	cmd := exec.Command("sc", "query", name)
	out, err := cmd.Output()
	if err != nil {
		return false, nil
	}
	return strings.Contains(string(out), "RUNNING"), nil
}
