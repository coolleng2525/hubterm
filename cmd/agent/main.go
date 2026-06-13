package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/coolleng2525/hubterm/internal/agent/reporter"
)

type NodeConfig struct {
	NodeID string `json:"node_id"`
	Token  string `json:"token,omitempty"`
}

func loadOrCreateConfig(dataDir string) *NodeConfig {
	configPath := filepath.Join(dataDir, "node.json")
	cfg := &NodeConfig{}

	if data, err := os.ReadFile(configPath); err == nil {
		if json.Unmarshal(data, cfg) == nil && cfg.NodeID != "" {
			return cfg
		}
	}

	cfg.NodeID = uuid.New().String()
	os.MkdirAll(dataDir, 0755)
	data, _ := json.Marshal(cfg)
	os.WriteFile(configPath, data, 0644)
	return cfg
}

func saveConfig(dataDir string, cfg *NodeConfig) {
	configPath := filepath.Join(dataDir, "node.json")
	data, _ := json.Marshal(cfg)
	os.WriteFile(configPath, data, 0644)
}

func main() {
	centerURL := flag.String("center", "http://localhost:8080", "Center service URL")
	nodeName := flag.String("name", "", "Node display name (default: hostname)")
	dataDir := flag.String("data", "./data", "Data directory for node config")
	flag.Parse()

	cfg := loadOrCreateConfig(*dataDir)

	if *nodeName == "" {
		hostname, _ := os.Hostname()
		*nodeName = hostname
	}

	log.Printf("Agent starting: node_id=%s center=%s name=%s", cfg.NodeID, *centerURL, *nodeName)

	rep := reporter.NewReporter(*centerURL, cfg.NodeID, *nodeName)
	if cfg.Token != "" {
		rep.SetNodeToken(cfg.Token)
		log.Printf("Loaded saved node token")
	}

	// first report immediately
	if err := rep.Report(); err != nil {
		log.Printf("Initial report error: %v", err)
	}

	// Save token if received from first report
	if rep.NodeToken != "" && rep.NodeToken != cfg.Token {
		cfg.Token = rep.NodeToken
		saveConfig(*dataDir, cfg)
		log.Printf("Node token saved to disk")
	}

	// then report every 3 seconds
	go rep.Start(3 * time.Second)

	// wait for signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	fmt.Printf("\nReceived signal %v, shutting down...\n", sig)
}
