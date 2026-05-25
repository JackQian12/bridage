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
	"golang.org/x/crypto/hkdf"
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
// masterKey must be at least 32 bytes; HKDF-SHA256 is used to derive the AES key.
// Returns a "v2:"-prefixed base64-encoded ciphertext for forward compatibility.
func EncryptSecret(masterKey, plaintext string) (string, error) {
	key := deriveKeyHKDF(masterKey)
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
	return "v2:" + base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptSecret reverses EncryptSecret.
// Supports both v2 (HKDF) and legacy (SHA-256) ciphertexts for migration compatibility.
func DecryptSecret(masterKey, encoded string) (string, error) {
	var key []byte
	if strings.HasPrefix(encoded, "v2:") {
		key = deriveKeyHKDF(masterKey)
		encoded = encoded[3:]
	} else {
		// Legacy SHA-256 derivation — kept for migrating old records.
		key = deriveKeySHA256(masterKey)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decrypt secret: decode: %w", err)
	}
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

// deriveKeyHKDF returns a 32-byte AES key derived from masterKey using HKDF-SHA256.
// This is the current (v2) key derivation method.
func deriveKeyHKDF(masterKey string) []byte {
	r := hkdf.New(sha256.New, []byte(strings.TrimSpace(masterKey)), nil, []byte("bridage-aes-gcm-v1"))
	key := make([]byte, 32)
	if _, err := io.ReadFull(r, key); err != nil {
		panic("hkdf derive key: " + err.Error())
	}
	return key
}

// deriveKeySHA256 is the legacy (v1) derivation kept only for decrypting
// secrets that were encrypted before the HKDF upgrade. Do not use for new encryptions.
func deriveKeySHA256(masterKey string) []byte {
	sum := sha256.Sum256([]byte(strings.TrimSpace(masterKey)))
	return sum[:]
}
