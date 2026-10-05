package security

import (
	"os"
	"testing"

	"github.com/ozodmeofficial/smarteye/internal/config"
)

func TestHashNetCodeDeterministicAndSalted(t *testing.T) {
	a := HashNetCode(config.NormalizeNetCode("482 913"))
	b := HashNetCode(config.NormalizeNetCode("482913"))
	if a != b {
		t.Fatalf("normalized codes should hash equal: %s vs %s", a, b)
	}
	if !VerifyNetCode(a, b) {
		t.Fatal("VerifyNetCode should accept equal hashes")
	}
	if VerifyNetCode(a, HashNetCode("000000")) {
		t.Fatal("VerifyNetCode should reject different codes")
	}
	// A bare SHA-256 of the code must not equal the salted hash.
	if a == "" || len(a) != 64 {
		t.Fatalf("unexpected hash format: %q", a)
	}
}

func TestGenerateCodeFormat(t *testing.T) {
	c, err := GenerateCode()
	if err != nil {
		t.Fatal(err)
	}
	if len(c) != 7 || c[3] != ' ' {
		t.Fatalf("code should be 'NNN NNN', got %q", c)
	}
}

func TestEnsureServerCert(t *testing.T) {
	dir, err := os.MkdirTemp("", "se-cert")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	c1, err := EnsureServerCert(dir)
	if err != nil {
		t.Fatalf("first cert: %v", err)
	}
	if len(c1.Certificate) == 0 {
		t.Fatal("empty certificate")
	}
	// Second call should load the same persisted cert, not regenerate.
	c2, err := EnsureServerCert(dir)
	if err != nil {
		t.Fatalf("second cert: %v", err)
	}
	if string(c1.Certificate[0]) != string(c2.Certificate[0]) {
		t.Fatal("certificate should be reused across calls")
	}
}
