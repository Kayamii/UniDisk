// Package crypto provides authenticated symmetric encryption for sensitive
// values stored at rest (notably provider credentials / refresh tokens).
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// prefix marks a value produced by this package, so Decrypt can tell encrypted
// values apart from legacy plaintext and decrypt only what it should.
const prefix = "enc:v1:"

// Box performs AES-256-GCM encryption with a fixed key.
type Box struct {
	gcm cipher.AEAD
}

// New builds a Box from a secret of any length (the key is its SHA-256).
func New(secret string) (*Box, error) {
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{gcm: gcm}, nil
}

// LoadOrCreateKey returns a stable encryption secret. If configuredKey is set,
// it's used as-is. Otherwise a random key is generated once and persisted at
// keyPath (0600), so encryption survives restarts with zero configuration.
func LoadOrCreateKey(configuredKey, keyPath string) (string, error) {
	if configuredKey != "" {
		return configuredKey, nil
	}
	if b, err := os.ReadFile(keyPath); err == nil && len(b) > 0 {
		return string(b), nil
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	key := base64.StdEncoding.EncodeToString(buf)
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(keyPath, []byte(key), 0o600); err != nil {
		return "", err
	}
	return key, nil
}

// Encrypt returns a prefixed, base64 ciphertext for plaintext.
func (b *Box) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, b.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := b.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return prefix + base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt reverses Encrypt. Values without the encryption prefix are returned
// unchanged — this lets legacy plaintext rows (written before encryption was
// enabled) keep working until they're next saved.
func (b *Box) Decrypt(value string) (string, error) {
	if len(value) < len(prefix) || value[:len(prefix)] != prefix {
		return value, nil // legacy plaintext
	}
	raw, err := base64.StdEncoding.DecodeString(value[len(prefix):])
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}
	ns := b.gcm.NonceSize()
	if len(raw) < ns {
		return "", errors.New("ciphertext too short")
	}
	plain, err := b.gcm.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plain), nil
}
