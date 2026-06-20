//go:build windows

package collector

import (
	"sort"
	"strconv"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func scanSerialPorts() []SerialPortInfo {
	key, err := registry.OpenKey(
		registry.LOCAL_MACHINE,
		`HARDWARE\DEVICEMAP\SERIALCOMM`,
		registry.QUERY_VALUE,
	)
	if err != nil {
		return []SerialPortInfo{}
	}
	defer key.Close()

	valueNames, err := key.ReadValueNames(0)
	if err != nil {
		return []SerialPortInfo{}
	}

	ports := make([]SerialPortInfo, 0, len(valueNames))
	seen := make(map[string]struct{}, len(valueNames))
	for _, valueName := range valueNames {
		portName, _, err := key.GetStringValue(valueName)
		portName = strings.ToUpper(strings.TrimSpace(portName))
		if err != nil || !isCOMPortName(portName) {
			continue
		}
		if _, ok := seen[portName]; ok {
			continue
		}
		seen[portName] = struct{}{}
		ports = append(ports, SerialPortInfo{
			PortName:    portName,
			Description: valueName,
			Status:      "online",
			BaudRate:    115200,
		})
	}

	sort.Slice(ports, func(i, j int) bool {
		return comPortNumber(ports[i].PortName) < comPortNumber(ports[j].PortName)
	})
	return ports
}

func isCOMPortName(name string) bool {
	if !strings.HasPrefix(name, "COM") {
		return false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(name, "COM"))
	return err == nil && n > 0
}

func comPortNumber(name string) int {
	n, _ := strconv.Atoi(strings.TrimPrefix(name, "COM"))
	return n
}
