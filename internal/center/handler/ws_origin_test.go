package handler

import (
	"net/http/httptest"
	"testing"
)

func TestWebSocketOriginBehindReverseProxy(t *testing.T) {
	tests := []struct {
		name   string
		host   string
		origin string
		want   bool
	}{
		{name: "same host and port", host: "192.168.1.55:8097", origin: "http://192.168.1.55:8097", want: true},
		{name: "proxy removed public port", host: "192.168.1.55", origin: "http://192.168.1.55:8097", want: true},
		{name: "different host", host: "192.168.1.55", origin: "http://attacker.example", want: false},
		{name: "non-browser request", host: "192.168.1.55", origin: "", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://"+tt.host+"/api/v1/terminal/connect", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if got := upgrader.CheckOrigin(req); got != tt.want {
				t.Fatalf("CheckOrigin() = %v, want %v", got, tt.want)
			}
		})
	}
}
