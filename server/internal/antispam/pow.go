package antispam

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

var noncePattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,19})$`)

func ValidNonce(value string) bool {
	return noncePattern.MatchString(value)
}

// VerifyPoW reports whether sha256(salt + ":" + nonce) has the required
// number of leading hexadecimal zeroes.
func VerifyPoW(salt, nonce string, difficulty int) bool {
	if difficulty < 1 || difficulty > 16 || !ValidNonce(nonce) {
		return false
	}
	sum := sha256.Sum256([]byte(salt + ":" + nonce))
	encoded := hex.EncodeToString(sum[:])
	return strings.HasPrefix(encoded, strings.Repeat("0", difficulty))
}
