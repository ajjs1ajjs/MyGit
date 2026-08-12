package api

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
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
	return base64.StdEncoding.EncodeToString(sealed), nil
}

func decryptToken(jwtSecret, encoded string) (string, error) {
	aead, err := tokenCipher(jwtSecret)
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	if len(raw) < aead.NonceSize() {
		return "", fmt.Errorf("token too short")
	}
	nonce, sealed := raw[:aead.NonceSize()], raw[aead.NonceSize():]
	plain, err := aead.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
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
