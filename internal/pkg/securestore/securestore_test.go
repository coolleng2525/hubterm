package securestore

import (
	"os"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY", "test-encryption-key-with-enough-entropy")
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
	os.Unsetenv("ENCRYPTION_KEY")
}

func TestFallsBackToJWTSecretForExistingDeployments(t *testing.T) {
	t.Setenv("JWT_SECRET", "legacy-secret-with-enough-entropy")
	encoded, err := Encrypt("legacy value")
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decrypt(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != "legacy value" {
		t.Fatalf("Decrypt() = %q", decoded)
	}
	os.Unsetenv("JWT_SECRET")
}
