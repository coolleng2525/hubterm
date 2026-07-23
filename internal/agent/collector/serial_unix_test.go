//go:build !windows

package collector

import "testing"

func TestIsSerialDevice(t *testing.T) {
	tests := []struct {
		goos       string
		name       string
		devicePath string
		want       bool
	}{
		{"linux", "ttyUSB0", "/sys/devices/pci/usb/ttyUSB0", true},
		{"linux", "ttyACM2", "/sys/devices/pci/usb/ttyACM2", true},
		{"linux", "ttyS4", "/sys/devices/pci0000:00/0000:00:16.3/tty/ttyS4", true},
		{"linux", "ttyS0", "/sys/devices/platform/serial8250/serial8250:0/serial8250:0.0", false},
		{"linux", "ttyS0", "", false},
		{"linux", "tty0", "/sys/devices/virtual/tty/tty0", false},
		{"darwin", "tty.usbserial", "", false},
		{"darwin", "cu.usbserial", "", true},
		{"darwin", "ttyUSB0", "", false},
	}

	for _, tt := range tests {
		if got := isSerialDevice(tt.goos, tt.name, tt.devicePath); got != tt.want {
			t.Errorf("isSerialDevice(%q, %q, %q) = %v, want %v", tt.goos, tt.name, tt.devicePath, got, tt.want)
		}
	}
}
