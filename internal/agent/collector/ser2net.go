package collector

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// Ser2netStatus ser2net 安装和运行状态
type Ser2netStatus struct {
	Installed  bool          `json:"installed"`
	Running    bool          `json:"running"`
	Version    string        `json:"version"`
	ConfigPath string        `json:"config_path"`
	Ports      []Ser2netPort `json:"ports"`
}

// Ser2netPort ser2net 配置中定义的串口映射
type Ser2netPort struct {
	TCPPort  int    `json:"tcp_port"`
	Device   string `json:"device"`
	BaudRate int    `json:"baud_rate"`
	Enabled  bool   `json:"enabled"`
}

// ser2netConfigPaths 按平台返回可能的 ser2net 配置文件路径
func ser2netConfigPaths() []string {
	switch runtime.GOOS {
	case "linux":
		return []string{
			"/etc/ser2net.conf",
			"/etc/ser2net/ser2net.conf",
		}
	case "darwin":
		return []string{
			"/usr/local/etc/ser2net.conf",
			"/opt/homebrew/etc/ser2net.conf",
		}
	default:
		return nil
	}
}

// findSer2netConfig 查找系统中存在的 ser2net 配置文件
func findSer2netConfig() string {
	for _, p := range ser2netConfigPaths() {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// checkSer2netBinary 检查 ser2net 二进制是否在 PATH 中
func checkSer2netBinary() (string, bool) {
	paths := []string{"/usr/sbin/ser2net", "/usr/bin/ser2net", "/usr/local/bin/ser2net"}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
	}
	// fallback: which
	out, err := exec.LookPath("ser2net")
	if err == nil {
		return out, true
	}
	return "", false
}

// getSer2netVersion 获取 ser2net 版本号
func getSer2netVersion(binaryPath string) string {
	out, err := exec.Command(binaryPath, "-v").Output()
	if err != nil {
		// some ser2net versions use --version
		out, err = exec.Command(binaryPath, "--version").Output()
		if err != nil {
			return ""
		}
	}
	return strings.TrimSpace(string(out))
}

// checkSer2netRunning 检查 ser2net 是否正在运行
// Linux: 优先用 systemctl，回退 pgrep
// macOS: 用 pgrep
func checkSer2netRunning() bool {
	switch runtime.GOOS {
	case "linux":
		// try systemctl first
		out, err := exec.Command("systemctl", "is-active", "ser2net").Output()
		if err == nil && strings.TrimSpace(string(out)) == "active" {
			return true
		}
		// fallback: pgrep
		out, err = exec.Command("pgrep", "-x", "ser2net").Output()
		return err == nil && len(out) > 0
	case "darwin":
		out, err := exec.Command("pgrep", "-x", "ser2net").Output()
		return err == nil && len(out) > 0
	default:
		return false
	}
}

// ser2netLinePattern 匹配 ser2net 配置行格式:
//
//	TCPPort:type:timeout:device:baud [,parity]
//	TCPPort:type:timeout:device:baud DATABITS PARITY STOPBITS
var ser2netLinePattern = regexp.MustCompile(`^(\d+):\w+:\d+:(/\dev/\S+):(\d+)`)

// parseSer2netPorts 解析 ser2net 配置文件中的串口映射
func parseSer2netPorts(configPath string) []Ser2netPort {
	var ports []Ser2netPort

	f, err := os.Open(configPath)
	if err != nil {
		return ports
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		matches := ser2netLinePattern.FindStringSubmatch(line)
		if len(matches) < 5 {
			continue
		}

		tcpPort, _ := strconv.Atoi(matches[1])
		baudRate, _ := strconv.Atoi(matches[4])

		ports = append(ports, Ser2netPort{
			TCPPort:  tcpPort,
			Device:   matches[3],
			BaudRate: baudRate,
			Enabled:  true,
		})
	}

	return ports
}

// DetectSer2net 检测 ser2net 安装和运行状态。
//
// Linux: 检查 /etc/ser2net.conf 和 systemctl status ser2net
// macOS: 检查 /usr/local/etc/ser2net.conf 和 pgrep ser2net
// Windows: 不适用，返回空状态
func DetectSer2net() *Ser2netStatus {
	status := &Ser2netStatus{
		Installed: false,
		Running:   false,
		Ports:     make([]Ser2netPort, 0),
	}

	// Windows: not applicable
	if runtime.GOOS == "windows" {
		return status
	}

	// Check if ser2net binary exists
	binaryPath, found := checkSer2netBinary()
	if !found {
		// no binary, but check if config exists (partial install)
		configPath := findSer2netConfig()
		if configPath == "" {
			return status
		}
		status.ConfigPath = configPath
		status.Ports = parseSer2netPorts(configPath)
		return status
	}

	status.Installed = true
	status.Version = getSer2netVersion(binaryPath)
	status.Running = checkSer2netRunning()

	// Find config file
	configPath := findSer2netConfig()
	if configPath == "" {
		// try common locations relative to binary
		dir := filepath.Dir(binaryPath)
		candidates := []string{
			filepath.Join(dir, "..", "etc", "ser2net.conf"),
			filepath.Join(dir, "..", "etc", "ser2net", "ser2net.conf"),
		}
		for _, p := range candidates {
			abs, _ := filepath.Abs(p)
			if _, err := os.Stat(abs); err == nil {
				configPath = abs
				break
			}
		}
	}

	status.ConfigPath = configPath
	if configPath != "" {
		status.Ports = parseSer2netPorts(configPath)
	}

	return status
}
