package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const (
	// KeyPrefix is prepended to every generated downstream API key.
	KeyPrefix = "brg_"
	// keyBytes is the number of random bytes for a downstream key.
	keyBytes = 32
)

// GenerateAPIKey creates a new cryptographically random downstream API key.
// Returns the plain-text key (display once) and its bcrypt hash for storage.
func GenerateAPIKey() (plaintext, hash string, err error) {
	raw := make([]byte, keyBytes)
	if _, err = rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("generate api key: %w", err)
	}
	plaintext = KeyPrefix + base64.RawURLEncoding.EncodeToString(raw)
	hashed, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	if err != nil {
		return "", "", fmt.Errorf("hash api key: %w", err)
	}
	return plaintext, string(hashed), nil
}

// VerifyAPIKey checks a plaintext key against its stored bcrypt hash.
func VerifyAPIKey(plaintext, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext))
	return err == nil
}

// HashAPIKeyFast returns a fast (SHA-256) hex lookup index for a key.
// This is stored alongside the bcrypt hash so we can quickly find the record
// before doing the expensive bcrypt check.
func HashAPIKeyFast(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// EncryptSecret encrypts a plaintext secret with AES-256-GCM using masterKey.
// masterKey must be at least 32 bytes; only the first 32 are used.
// Returns a base64-encoded ciphertext prefixed with the nonce.
func EncryptSecret(masterKey, plaintext string) (string, error) {
	key := deriveKey(masterKey)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("encrypt secret: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("encrypt secret: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("encrypt secret nonce: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptSecret reverses EncryptSecret.
func DecryptSecret(masterKey, encoded string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decrypt secret: decode: %w", err)
	}
	key := deriveKey(masterKey)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("decrypt secret: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("decrypt secret: %w", err)
	}
	if len(ciphertext) < gcm.NonceSize() {
		return "", errors.New("decrypt secret: ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt secret: %w", err)
	}
	return string(plain), nil
}

// deriveKey returns a 32-byte AES key derived from an arbitrary-length master key.
func deriveKey(masterKey string) []byte {
	sum := sha256.Sum256([]byte(strings.TrimSpace(masterKey)))
	return sum[:]
}
