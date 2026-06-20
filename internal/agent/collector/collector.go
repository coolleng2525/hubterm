package collector

import (
	"net"
	"runtime"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
)

type SystemInfo struct {
	Hostname      string  `json:"hostname"`
	OS            string  `json:"os"`
	OSVersion     string  `json:"os_version"`
	Arch          string  `json:"arch"`
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryTotal   uint64  `json:"memory_total"`
	MemoryUsed    uint64  `json:"memory_used"`
	MemoryPercent float64 `json:"memory_percent"`
	DiskTotal     uint64  `json:"disk_total"`
	DiskUsed      uint64  `json:"disk_used"`
}

func CollectSystemInfo() (*SystemInfo, error) {
	info := &SystemInfo{}

	hostInfo, err := host.Info()
	if err == nil {
		info.Hostname = hostInfo.Hostname
		info.OS = hostInfo.OS
		info.OSVersion = hostInfo.PlatformVersion
	}

	info.Arch = runtime.GOARCH

	cpuPercent, err := cpu.Percent(0, false)
	if err == nil && len(cpuPercent) > 0 {
		info.CPUPercent = cpuPercent[0]
	}

	memInfo, err := mem.VirtualMemory()
	if err == nil {
		info.MemoryTotal = memInfo.Total
		info.MemoryUsed = memInfo.Used
		info.MemoryPercent = memInfo.UsedPercent
	}

	diskInfo, err := disk.Usage("/")
	if err == nil {
		info.DiskTotal = diskInfo.Total
		info.DiskUsed = diskInfo.Used
	}

	return info, nil
}

type NetworkInterface struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
}

func GetAllInterfaces() []NetworkInterface {
	var result []NetworkInterface

	ifaces, err := net.Interfaces()
	if err != nil {
		return result
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok || ipnet.IP.IsLoopback() {
				continue
			}
			if ip4 := ipnet.IP.To4(); ip4 != nil {
				result = append(result, NetworkInterface{
					Name: iface.Name,
					IP:   ip4.String(),
				})
			}
		}
	}

	return result
}

func GetLocalIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "unknown"
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok || ipnet.IP.IsLoopback() {
				continue
			}
			if ip4 := ipnet.IP.To4(); ip4 != nil {
				return ip4.String()
			}
		}
	}
	return "unknown"
}

type SerialPortInfo struct {
	PortName    string `json:"port_name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	BaudRate    int    `json:"baud_rate"`
}

func ScanSerialPorts() []SerialPortInfo {
	return scanSerialPorts()
}
