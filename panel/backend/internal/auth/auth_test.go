package auth_test

import (
	"testing"
	"time"

	"github.com/zema-tech/MINE/panel/backend/internal/auth"
)

func TestPasswordRoundtrip(t *testing.T) {
	h, err := auth.HashPassword("correct-horse-123!")
	if err != nil {
		t.Fatal(err)
	}
	if !auth.VerifyPassword(h, "correct-horse-123!") {
		t.Fatal("valid password rejected")
	}
	if auth.VerifyPassword(h, "wrong") {
		t.Fatal("invalid password accepted")
	}
}

func TestJWTRoundtrip(t *testing.T) {
	tok, err := auth.IssueJWT("test-secret-32chars-min________", 42, true, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	c, err := auth.ParseJWT("test-secret-32chars-min________", tok)
	if err != nil {
		t.Fatal(err)
	}
	if c.UserID != 42 || !c.IsAdmin {
		t.Fatalf("bad claims: %+v", c)
	}
	if _, err := auth.ParseJWT("other-secret", tok); err == nil {
		t.Fatal("expected error with wrong secret")
	}
}

func TestSecretsAtRest(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef"
	enc, err := auth.EncryptSecret(key, "daemon-token-xyz")
	if err != nil {
		t.Fatal(err)
	}
	dec, err := auth.DecryptSecret(key, enc)
	if err != nil {
		t.Fatal(err)
	}
	if dec != "daemon-token-xyz" {
		t.Fatalf("roundtrip mismatch: %q", dec)
	}
}
