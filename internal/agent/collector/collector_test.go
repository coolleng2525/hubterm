package collector

import (
	"testing"
)

func TestGetHostInfo(t *testing.T) {
	info, err := CollectSystemInfo()
	if err != nil {
		t.Fatalf("CollectSystemInfo failed: %v", err)
	}

	if info == nil {
		t.Fatal("expected non-nil SystemInfo")
	}

	// Hostname should be non-empty on any real system
	if info.Hostname == "" {
		t.Log("warning: hostname is empty (expected in some containerized environments)")
	}

	// OS should be set (linux/darwin/windows)
	if info.OS == "" {
		t.Error("expected non-empty OS")
	}

	// Arch should be set
	if info.Arch == "" {
		t.Error("expected non-empty Arch")
	}

	// Memory values should be reasonable
	if info.MemoryTotal == 0 {
		t.Log("warning: MemoryTotal is 0 (expected in constrained environments)")
	}
}

func TestGetResourceUsage(t *testing.T) {
	info, err := CollectSystemInfo()
	if err != nil {
		t.Fatalf("CollectSystemInfo failed: %v", err)
	}

	// CPU percent should be between 0 and 100
	if info.CPUPercent < 0 || info.CPUPercent > 100 {
		t.Errorf("CPUPercent out of range [0,100]: %f", info.CPUPercent)
	}

	// Memory percent should be between 0 and 100
	if info.MemoryPercent < 0 || info.MemoryPercent > 100 {
		t.Errorf("MemoryPercent out of range [0,100]: %f", info.MemoryPercent)
	}

	// Memory used should not exceed total
	if info.MemoryUsed > info.MemoryTotal && info.MemoryTotal > 0 {
		t.Errorf("MemoryUsed (%d) > MemoryTotal (%d)", info.MemoryUsed, info.MemoryTotal)
	}

	// Disk values should be reasonable
	if info.DiskTotal > 0 {
		if info.DiskUsed > info.DiskTotal {
			t.Errorf("DiskUsed (%d) > DiskTotal (%d)", info.DiskUsed, info.DiskTotal)
		}
	}
}

func TestGetLocalIP(t *testing.T) {
	ip := GetLocalIP()
	if ip == "" {
		t.Error("expected non-empty IP")
	}
	// Should be "unknown" or a valid IP
	if ip != "unknown" && len(ip) < 7 {
		t.Errorf("unexpected IP format: %s", ip)
	}
}

func TestScanSerialPorts(t *testing.T) {
	ports := ScanSerialPorts()
	// This should not panic and should return a slice (possibly empty)
	if ports == nil {
		t.Error("expected non-nil slice, got nil")
	}
}
