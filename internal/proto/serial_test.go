package hubtermproto

import "testing"

func TestDefaultSerialConfig(t *testing.T) {
	cfg := DefaultSerialConfig("/dev/cu.usbserial-test")
	if cfg.BaudRate != 115200 || cfg.DataBits != 8 || cfg.Parity != SerialParityNone || cfg.StopBits != 1 || cfg.FlowControl != SerialFlowNone {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config must be valid: %v", err)
	}
}

func TestSerialConfigValidation(t *testing.T) {
	valid := DefaultSerialConfig("COM3")
	tests := []struct {
		name   string
		mutate func(*SerialConfig)
	}{
		{name: "missing port", mutate: func(c *SerialConfig) { c.PortName = "" }},
		{name: "unsupported baud", mutate: func(c *SerialConfig) { c.BaudRate = 12345 }},
		{name: "invalid data bits", mutate: func(c *SerialConfig) { c.DataBits = 9 }},
		{name: "invalid parity", mutate: func(c *SerialConfig) { c.Parity = "mark" }},
		{name: "invalid stop bits", mutate: func(c *SerialConfig) { c.StopBits = 3 }},
		{name: "invalid flow control", mutate: func(c *SerialConfig) { c.FlowControl = "xonxoff" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := valid
			tc.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatalf("expected invalid config: %+v", cfg)
			}
		})
	}
}
