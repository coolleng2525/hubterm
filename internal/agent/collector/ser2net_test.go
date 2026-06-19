package collector

import (
	"testing"
)

// TestDetectSer2net 验证 DetectSer2net 返回结构正确，不依赖外部环境。
func TestDetectSer2net(t *testing.T) {
	status := DetectSer2net()

	if status == nil {
		t.Fatal("DetectSer2net() returned nil")
	}

	// 结构字段应该都有默认零值，不会 panic
	t.Logf("ser2net installed: %v", status.Installed)
	t.Logf("ser2net running: %v", status.Running)
	t.Logf("ser2net version: %q", status.Version)
	t.Logf("ser2net config_path: %q", status.ConfigPath)
	t.Logf("ser2net ports count: %d", len(status.Ports))

	// Ports 不能是 nil（应该是空 slice）
	if status.Ports == nil {
		t.Error("expected non-nil Ports slice, got nil")
	}

	// 每个 port 字段应该有零值
	for i, p := range status.Ports {
		if p.TCPPort == 0 && p.Device == "" && p.BaudRate == 0 {
			t.Logf("port[%d] has all zero values (unlikely but possible)", i)
		}
	}
}
