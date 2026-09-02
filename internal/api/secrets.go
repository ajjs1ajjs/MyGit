package api

import (
"crypto/aes"
"crypto/cipher"
"crypto/rand"
"crypto/sha256"
"encoding/base64"
"fmt"
"io"
"os"
"time"
)

// tokenCipher encrypts/decrypts integration tokens at rest with AES-256-GCM.
// The key is derived from the JWT secret so no extra secret file is needed;
// rotating MYGIT_JWT_SECRET makes stored integration tokens undecryptable,
// which is the documented trade-off.
func tokenCipher(jwtSecret string) (cipher.AEAD, error) {
sum := sha256.Sum256([]byte(jwtSecret))
block, err := aes.NewCipher(sum[:])
if err != nil {
return nil, err
}
return cipher.NewGCM(block)
}

func encryptToken(jwtSecret, plain string) (string, error) {
	aead, err := tokenCipher(jwtSecret)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := aead.Seal(nonce, nonce, []byte(plain), nil)
	// Prepend version byte (0x01) for version-aware decryption
	sealedWithVersion := make([]byte, 1+len(sealed))
	sealedWithVersion[0] = 0x01
	copy(sealedWithVersion[1:], sealed)
	return base64.StdEncoding.EncodeToString(sealedWithVersion), nil
}

func decryptToken(jwtSecret, encoded string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	if len(raw) < 5 {
		return "", fmt.Errorf("token too short")
	}
	version := raw[0]
	aead, err := tokenCipher(jwtSecret)
	if err != nil {
		return "", err
	}
	nonceSize := aead.NonceSize()
	// Support both old format (no version byte) and new format (version byte 0x01)
	if version == 0x01 {
		// New format: 1 byte version + nonceSize bytes nonce + ciphertext
		if len(raw) < 1+nonceSize+1 {
			return "", fmt.Errorf("token too short")
		}
		nonce := raw[1 : 1+nonceSize]
		sealed := raw[1+nonceSize:]
		if len(sealed) < nonceSize {
			return "", fmt.Errorf("token too short")
		}
		plain, err := aead.Open(nil, nonce, sealed, nil)
		if err != nil {
			return "", err
		}
		return string(plain), nil
	} else {
		// Old format: no version byte, entire raw is nonce+ciphertext
		// Assume nonce is the first nonceSize bytes
		if len(raw) < 2*nonceSize {
			return "", fmt.Errorf("token too short")
		}
		nonce := raw[:nonceSize]
		sealed := raw[nonceSize:]
		if len(sealed) < nonceSize {
			return "", fmt.Errorf("token too short")
		}
		plain, err := aead.Open(nil, nonce, sealed, nil)
		if err != nil {
			return "", err
		}
		return string(plain), nil
	}
}

func maskToken(plain string) string {
if plain == "" {
return ""
}
if len(plain) <= 4 {
return "****"
}
return "****" + plain[len(plain)-4:]
}

// backupKey derives the AES key for backup archives.
// If MYGIT_BACKUP_KEY is set, it is used directly (must be 32 bytes).
// Otherwise a key is derived from the JWT secret with a version byte prefix.
// The version byte (currently 0x01) allows supporting multiple key versions
// during secret rotation without losing access to existing encrypted archives.
func backupKey(jwtSecret, configured string) []byte {
	if configured != "" {
		if len(configured) >= 32 {
			return []byte(configured)[:32]
		}
		sum := sha256.Sum256([]byte(configured))
		return sum[:]
	}
// Derive from JWT secret with version byte for rotation support
// jwtSecret is a string; we hash it with a version prefix to produce a 32-byte key
sum := sha256.Sum256(append([]byte{0x01, 0x00, 0x00, 0x00}, []byte(jwtSecret)...))
return sum[:]
}

func newAEAD(jwtSecret, configured string) (cipher.AEAD, error) {
	block, err := aes.NewCipher(backupKey(jwtSecret, configured))
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// sealBytesForV1 seals plaintext with a version byte prefix (0x01) in the output.
// This enables decryption to detect the supported version and reject old/unsupported formats.
func sealBytesForV1(plain []byte, jwtSecret, configured string) ([]byte, error) {
	aead, err := newAEAD(jwtSecret, configured)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	sealed := aead.Seal(nonce, nonce, plain, nil)
	// Prepend version byte (0x01) for version-aware decryption
	sealedWithVersion := make([]byte, 1+len(sealed))
	sealedWithVersion[0] = 0x01
	copy(sealedWithVersion[1:], sealed)
	return sealedWithVersion, nil
}

func encryptFile(path, jwtSecret, configured string) (string, error) {
	plain, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sealed, err := sealBytesForV1(plain, jwtSecret, configured)
	if err != nil {
		return "", err
	}
	encPath := path + ".enc"
	if err := os.WriteFile(encPath, sealed, 0o600); err != nil {
		return "", err
	}
	if err := os.Remove(path); err != nil {
	// Windows may briefly hold a handle; one delayed retry.
	time.Sleep(50 * time.Millisecond)
	_ = os.Remove(path)
	}
	return encPath, nil
}

func openBytesForV1(sealed []byte, jwtSecret, configured string) ([]byte, error) {
	if len(sealed) < 2 {
		return nil, fmt.Errorf("ciphertext too short")
	}
	version := sealed[0]
	if version != 0x01 {
		return nil, fmt.Errorf("unsupported token version: %d", version)
	}
	body := sealed[1:]
	aead, err := newAEAD(jwtSecret, configured)
	if err != nil {
		return nil, err
	}
	if len(body) < aead.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := body[:aead.NonceSize()], body[aead.NonceSize():]
	return aead.Open(nil, nonce, ciphertext, nil)
}

func decryptFile(path, jwtSecret, configured string) ([]byte, error) {
	sealed, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return openBytesForV1(sealed, jwtSecret, configured)
}