package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// apiKeyPrefix tags UniDisk keys so they're recognizable in logs/configs.
const apiKeyPrefix = "udk_"

// GenerateAPIKey returns a new random API key (plaintext), its sha256 hash for
// storage, and a short display prefix. The plaintext is returned to the caller
// once and never stored.
func GenerateAPIKey() (plaintext, hash, displayPrefix string, err error) {
	b := make([]byte, 24) // 192 bits of entropy
	if _, err = rand.Read(b); err != nil {
		return "", "", "", fmt.Errorf("generate api key: %w", err)
	}
	plaintext = apiKeyPrefix + hex.EncodeToString(b)
	hash = HashAPIKey(plaintext)
	// Show enough to identify the key without revealing it.
	displayPrefix = plaintext[:len(apiKeyPrefix)+6]
	return plaintext, hash, displayPrefix, nil
}

// HashAPIKey returns the sha256 hex hash used to look a key up. A plain hash
// (no bcrypt) is appropriate here: keys are high-entropy random tokens, so the
// hash must be deterministic for O(1) lookup and there's nothing to brute-force.
func HashAPIKey(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}
