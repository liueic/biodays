package settings

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

// CiphertextPrefix marks an encrypted value inside the document. Plaintext
// secrets are never persisted; anything without this prefix is either the
// clear sentinel or a value on its way through EncryptDocument.
const CiphertextPrefix = "v1:"

// ErrBadCiphertext reports a value that carries the version prefix but does
// not decrypt with the configured key (wrong key, or corrupted data).
var ErrBadCiphertext = errors.New("encrypted setting could not be decrypted")

// ParseEncryptionKey decodes a base64-encoded 32-byte AES-256 key exactly as
// produced by `openssl rand -base64 32`.
func ParseEncryptionKey(raw string) ([]byte, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, errors.New("empty encryption key")
	}
	key, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil {
		return nil, fmt.Errorf("decode BIODAYS_SETTINGS_ENC_KEY: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("BIODAYS_SETTINGS_ENC_KEY must decode to 32 bytes, got %d", len(key))
	}
	return key, nil
}

func isCiphertext(value string) bool {
	return strings.HasPrefix(value, CiphertextPrefix)
}

func encrypt(key []byte, plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return CiphertextPrefix + base64.StdEncoding.EncodeToString(sealed), nil
}

func decrypt(key []byte, value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if !isCiphertext(value) {
		return "", fmt.Errorf("%w: missing %s prefix", ErrBadCiphertext, CiphertextPrefix)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, CiphertextPrefix))
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrBadCiphertext, err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("%w: truncated", ErrBadCiphertext)
	}
	nonce, sealed := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", fmt.Errorf("%w: authentication failed", ErrBadCiphertext)
	}
	return string(plaintext), nil
}

// SecretPlaintexts decrypts the three secret fields in one call. A missing
// key with non-empty ciphertext is an error the UI surfaces as "unreadable,
// re-enter". Empty fields decrypt to "".
func (d Document) SecretPlaintexts(key []byte) (smtpPassword, turnstileSecret, aiToken string, err error) {
	if smtpPassword, err = decrypt(key, d.SMTP.Password); err != nil {
		return "", "", "", fmt.Errorf("smtp password: %w", err)
	}
	if turnstileSecret, err = decrypt(key, d.Turnstile.SecretKey); err != nil {
		return "", "", "", fmt.Errorf("turnstile secret: %w", err)
	}
	if aiToken, err = decrypt(key, d.AISpam.APIToken); err != nil {
		return "", "", "", fmt.Errorf("ai api token: %w", err)
	}
	return smtpPassword, turnstileSecret, aiToken, nil
}

// EncryptSecrets encrypts the plaintext secret fields in place. Values that
// are already ciphertext or the clear sentinel pass through untouched, so a
// document that round-tripped through the API stays valid.
func (d *Document) EncryptSecrets(key []byte) error {
	encryptField := func(value string) (string, error) {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || trimmed == ClearSentinel || isCiphertext(trimmed) {
			return trimmed, nil
		}
		return encrypt(key, trimmed)
	}
	password, err := encryptField(d.SMTP.Password)
	if err != nil {
		return fmt.Errorf("encrypt smtp password: %w", err)
	}
	d.SMTP.Password = password
	secret, err := encryptField(d.Turnstile.SecretKey)
	if err != nil {
		return fmt.Errorf("encrypt turnstile secret: %w", err)
	}
	d.Turnstile.SecretKey = secret
	token, err := encryptField(d.AISpam.APIToken)
	if err != nil {
		return fmt.Errorf("encrypt ai token: %w", err)
	}
	d.AISpam.APIToken = token
	return nil
}
