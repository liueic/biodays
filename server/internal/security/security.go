package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// RandomToken returns 32 bytes of entropy encoded as URL-safe base64.
func RandomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashToken is the only representation of session/subscription tokens kept in
// the database.
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// HMACKey derives a stable, non-reversible key for rate limits and email
// fingerprints. Callers pass domain-separated parts.
func HMACKey(secret []byte, parts ...string) string {
	mac := hmac.New(sha256.New, secret)
	for i, part := range parts {
		if i > 0 {
			mac.Write([]byte{0})
		}
		mac.Write([]byte(part))
	}
	return hex.EncodeToString(mac.Sum(nil))
}

// CSRFFor derives the CSRF token from a session token hash so /admin/me can
// return it without storing the raw value.
func CSRFFor(secret []byte, sessionTokenHash string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte("csrf\x00"))
	mac.Write([]byte(sessionTokenHash))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func ConstantTimeEqual(a, b string) bool {
	return hmac.Equal([]byte(a), []byte(b))
}

func CredentialVersion(username, passwordHash string) string {
	sum := sha256.Sum256([]byte(username + "\x00" + passwordHash))
	return fmt.Sprintf("%x", sum[:])
}
