package reporter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/coolleng2525/hubterm/internal/agent/collector"
	hubtermproto "github.com/coolleng2525/hubterm/internal/proto"
)

type Reporter struct {
	CenterURL string
	NodeID    string
	NodeName  string
	NodeToken string
	Client    *http.Client
}

func NewReporter(centerURL, nodeID, nodeName string) *Reporter {
	return &Reporter{
		CenterURL: centerURL,
		NodeID:    nodeID,
		NodeName:  nodeName,
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SetNodeToken sets the node authentication token.
func (r *Reporter) SetNodeToken(token string) {
	r.NodeToken = token
}

func (r *Reporter) Report() error {
	sysInfo, err := collector.CollectSystemInfo()
	if err != nil {
		return fmt.Errorf("collect system info: %w", err)
	}

	ports := collector.ScanSerialPorts()

	report := hubtermproto.NodeReport{
		NodeID:        r.NodeID,
		Name:          r.NodeName,
		IP:            collector.GetLocalIP(),
		Hostname:      sysInfo.Hostname,
		OS:            sysInfo.OS,
		OSVersion:     sysInfo.OSVersion,
		Arch:          sysInfo.Arch,
		CPUPercent:    sysInfo.CPUPercent,
		MemoryTotal:   sysInfo.MemoryTotal,
		MemoryUsed:    sysInfo.MemoryUsed,
		MemoryPercent: sysInfo.MemoryPercent,
		DiskTotal:     sysInfo.DiskTotal,
		DiskUsed:      sysInfo.DiskUsed,
		SerialPorts:   make([]hubtermproto.SerialPortInfo, len(ports)),
		Sessions:      []hubtermproto.SessionInfo{},
	}

	for i, p := range ports {
		report.SerialPorts[i] = hubtermproto.SerialPortInfo{
			PortName:    p.PortName,
			Description: p.Description,
			Status:      p.Status,
			BaudRate:    p.BaudRate,
		}
	}

	data, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}

	req, err := http.NewRequest("POST", r.CenterURL+"/api/nodes/report", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if r.NodeToken != "" {
		req.Header.Set("Authorization", "Bearer "+r.NodeToken)
	}

	resp, err := r.Client.Do(req)
	if err != nil {
		return fmt.Errorf("post report: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("report failed (status %d): %s", resp.StatusCode, string(body))
	}

	// Extract token from response if present (first report registers the node)
	var result struct {
		Success bool   `json:"success"`
		Token   string `json:"token,omitempty"`
	}
	if err := json.Unmarshal(body, &result); err == nil && result.Token != "" {
		r.SetNodeToken(result.Token)
		log.Printf("Node token received and saved: %s", result.Token[:16]+"...")
	}

	return nil
}

func (r *Reporter) Start(interval time.Duration) {
	log.Printf("Reporter started: node=%s center=%s interval=%v", r.NodeID, r.CenterURL, interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		if err := r.Report(); err != nil {
			log.Printf("Report error: %v", err)
		}
	}
}
