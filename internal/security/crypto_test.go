package security_test

import (
	"strings"
	"testing"

	"github.com/nuts/bridage/internal/security"
)

func TestGenerateAPIKey_Format(t *testing.T) {
	plain, hash, err := security.GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}
	if !strings.HasPrefix(plain, "brg_") {
		t.Errorf("plaintext key missing brg_ prefix: %q", plain)
	}
	if len(plain) < 20 {
		t.Errorf("plaintext key too short: %q", plain)
	}
	if hash == "" {
		t.Error("hash is empty")
	}
	if hash == plain {
		t.Error("hash must differ from plaintext")
	}
}

func TestGenerateAPIKey_Unique(t *testing.T) {
	plain1, _, _ := security.GenerateAPIKey()
	plain2, _, _ := security.GenerateAPIKey()
	if plain1 == plain2 {
		t.Error("two generated keys must not be identical")
	}
}

func TestVerifyAPIKey_RoundTrip(t *testing.T) {
	plain, hash, err := security.GenerateAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	if !security.VerifyAPIKey(plain, hash) {
		t.Error("VerifyAPIKey returned false for matching pair")
	}
}

func TestVerifyAPIKey_WrongKey(t *testing.T) {
	plain, hash, _ := security.GenerateAPIKey()
	other, _, _ := security.GenerateAPIKey()
	_ = plain
	if security.VerifyAPIKey(other, hash) {
		t.Error("VerifyAPIKey returned true for mismatched pair")
	}
}

func TestHashAPIKeyFast_Deterministic(t *testing.T) {
	key := "brg_testkey123"
	h1 := security.HashAPIKeyFast(key)
	h2 := security.HashAPIKeyFast(key)
	if h1 != h2 {
		t.Error("HashAPIKeyFast is not deterministic")
	}
}

func TestHashAPIKeyFast_Different(t *testing.T) {
	h1 := security.HashAPIKeyFast("brg_key_a")
	h2 := security.HashAPIKeyFast("brg_key_b")
	if h1 == h2 {
		t.Error("different keys must produce different fast hashes")
	}
}

func TestEncryptDecryptSecret_RoundTrip(t *testing.T) {
	masterKey := "my-super-secret-master-key-for-tests"
	secret := "sk-abcdefgh1234567890"

	encrypted, err := security.EncryptSecret(masterKey, secret)
	if err != nil {
		t.Fatalf("EncryptSecret: %v", err)
	}
	if encrypted == secret {
		t.Error("encrypted output must differ from plaintext")
	}

	decrypted, err := security.DecryptSecret(masterKey, encrypted)
	if err != nil {
		t.Fatalf("DecryptSecret: %v", err)
	}
	if decrypted != secret {
		t.Errorf("DecryptSecret = %q, want %q", decrypted, secret)
	}
}

func TestEncryptSecret_Nondeterministic(t *testing.T) {
	masterKey := "my-super-secret-master-key-for-tests"
	secret := "sk-same-value"

	enc1, _ := security.EncryptSecret(masterKey, secret)
	enc2, _ := security.EncryptSecret(masterKey, secret)
	if enc1 == enc2 {
		t.Error("encrypt should produce different ciphertext each call (random nonce)")
	}
}

func TestDecryptSecret_WrongKey(t *testing.T) {
	encrypted, _ := security.EncryptSecret("correct-master-key-padding", "secret")
	_, err := security.DecryptSecret("wrong-master-key-padding-xxx", encrypted)
	if err == nil {
		t.Error("DecryptSecret with wrong key should return error")
	}
}

func TestDecryptSecret_Tampered(t *testing.T) {
	encrypted, _ := security.EncryptSecret("test-key-value", "mysecret")
	// Flip last byte of base64 string
	bs := []byte(encrypted)
	bs[len(bs)-2] ^= 0x01
	_, err := security.DecryptSecret("test-key-value", string(bs))
	if err == nil {
		t.Error("DecryptSecret on tampered ciphertext should return error")
	}
}
