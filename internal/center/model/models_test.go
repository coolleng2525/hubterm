package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSerialPortConfigMigrationDefaultsAndUniqueness(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}

	cfg := SerialPortConfig{NodeID: "node-1", PortName: "/dev/ttyUSB0"}
	if err := db.Create(&cfg).Error; err != nil {
		t.Fatal(err)
	}
	if cfg.Alias != "" || cfg.BaudRate != 115200 || cfg.DataBits != 8 || cfg.Parity != "none" || cfg.StopBits != 1 || cfg.FlowControl != "none" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}

	duplicate := SerialPortConfig{NodeID: cfg.NodeID, PortName: cfg.PortName}
	if err := db.Create(&duplicate).Error; err == nil {
		t.Fatal("expected node/port uniqueness violation")
	}
}

func TestSerialPortConfigAliasMigrationPreservesExistingRows(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE serial_port_configs (
		id integer PRIMARY KEY AUTOINCREMENT,
		node_id text NOT NULL,
		port_name text NOT NULL,
		baud_rate integer NOT NULL DEFAULT 115200,
		data_bits integer NOT NULL DEFAULT 8,
		parity text NOT NULL DEFAULT 'none',
		stop_bits integer NOT NULL DEFAULT 1,
		flow_control text NOT NULL DEFAULT 'none',
		created_at datetime,
		updated_at datetime
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO serial_port_configs (node_id, port_name) VALUES (?, ?)`, "node-legacy", "/dev/ttyUSB0").Error; err != nil {
		t.Fatal(err)
	}

	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	var cfg SerialPortConfig
	if err := db.Where("node_id = ? AND port_name = ?", "node-legacy", "/dev/ttyUSB0").First(&cfg).Error; err != nil {
		t.Fatal(err)
	}
	if cfg.Alias != "" || cfg.BaudRate != 115200 {
		t.Fatalf("legacy config was not preserved: %+v", cfg)
	}
}
