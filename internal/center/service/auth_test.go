package service

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashAndVerify(t *testing.T) {
	t.Run("hash and verify correct password", func(t *testing.T) {
		password := "testpassword123"
		hash, err := HashPassword(password)
		if err != nil {
			t.Fatalf("HashPassword failed: %v", err)
		}
		if hash == "" {
			t.Fatal("expected non-empty hash")
		}
		if hash == password {
			t.Fatal("hash should not equal plaintext password")
		}

		if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
			t.Errorf("verify correct password failed: %v", err)
		}
	})

	t.Run("verify wrong password fails", func(t *testing.T) {
		password := "testpassword123"
		hash, err := HashPassword(password)
		if err != nil {
			t.Fatalf("HashPassword failed: %v", err)
		}

		if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("wrongpassword")); err == nil {
			t.Error("expected error for wrong password, got nil")
		}
	})

	t.Run("different passwords produce different hashes", func(t *testing.T) {
		hash1, _ := HashPassword("password1")
		hash2, _ := HashPassword("password1")
		hash3, _ := HashPassword("password2")

		if hash1 == hash2 {
			t.Error("bcrypt should produce different hashes each time (random salt)")
		}
		if hash1 == hash3 {
			t.Error("different passwords should produce different hashes")
		}
	})
}
