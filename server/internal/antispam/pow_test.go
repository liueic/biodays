package antispam

import (
	"strconv"
	"testing"
)

func findNonce(salt string, difficulty int) string {
	for i := 0; ; i++ {
		nonce := strconv.Itoa(i)
		if VerifyPoW(salt, nonce, difficulty) {
			return nonce
		}
	}
}

func TestVerifyPoW(t *testing.T) {
	nonce := findNonce("test-salt", 2)
	if !VerifyPoW("test-salt", nonce, 2) {
		t.Fatalf("expected nonce %s to satisfy difficulty 2", nonce)
	}
	if VerifyPoW("test-salt", nonce, 3) {
		t.Fatalf("did not expect nonce %s to satisfy difficulty 3", nonce)
	}
	if VerifyPoW("test-salt", "01", 2) {
		t.Fatal("leading-zero nonce must be rejected")
	}
	if VerifyPoW("test-salt", "abc", 2) {
		t.Fatal("non-numeric nonce must be rejected")
	}
}
