//go:build windows

package collector

import "testing"

func TestIsCOMPortName(t *testing.T) {
	tests := map[string]bool{
		"COM1":   true,
		"COM256": true,
		"COM0":   false,
		"COM":    false,
		"LPT1":   false,
	}
	for name, want := range tests {
		if got := isCOMPortName(name); got != want {
			t.Errorf("isCOMPortName(%q) = %v, want %v", name, got, want)
		}
	}
}
