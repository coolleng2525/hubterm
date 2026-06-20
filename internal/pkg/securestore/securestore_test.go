package securestore

import (
	"os"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-with-enough-entropy")
	encoded, err := Encrypt("sensitive value")
	if err != nil {
		t.Fatal(err)
	}
	if encoded == "sensitive value" {
		t.Fatal("credential was not encrypted")
	}
	decoded, err := Decrypt(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != "sensitive value" {
		t.Fatalf("Decrypt() = %q", decoded)
	}
	os.Unsetenv("JWT_SECRET")
}
