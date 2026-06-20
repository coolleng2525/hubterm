//go:build !windows

package collector

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/sys/unix"
)

func scanSerialPorts() []SerialPortInfo {
	ports := make([]SerialPortInfo, 0)
	entries, _ := os.ReadDir("/dev")
	for _, entry := range entries {
		name := entry.Name()
		if !isSerialDevice(runtime.GOOS, name, serialDevicePath(name)) {
			continue
		}
		ports = append(ports, SerialPortInfo{
			PortName: fmt.Sprintf("/dev/%s", name),
			Status:   serialPortStatus(filepath.Join("/dev", name)),
			BaudRate: 115200,
		})
	}
	return ports
}

func serialPortStatus(path string) string {
	if err := unix.Access(path, unix.R_OK|unix.W_OK); err != nil {
		return "offline"
	}
	return "online"
}

func serialDevicePath(name string) string {
	path, err := filepath.EvalSymlinks(filepath.Join("/sys/class/tty", name, "device"))
	if err != nil {
		return ""
	}
	return filepath.ToSlash(path)
}

func isSerialDevice(goos, name, devicePath string) bool {
	switch goos {
	case "linux":
		if strings.HasPrefix(name, "ttyUSB") || strings.HasPrefix(name, "ttyACM") {
			return true
		}
		if !strings.HasPrefix(name, "ttyS") || devicePath == "" {
			return false
		}
		// serial8250 pre-creates ttyS0..ttyS31 even when no UART exists.
		// Keep ports backed by a real bus device (PCI, PNP, etc.) and drop
		// those generic placeholders.
		return !strings.Contains(devicePath, "/devices/platform/serial8250/serial8250:0/")
	case "darwin":
		return strings.HasPrefix(name, "tty.") || strings.HasPrefix(name, "cu.")
	default:
		return false
	}
}
