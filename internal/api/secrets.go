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

// backupKey derives the AES key for backup archives: MYGIT_BACKUP_KEY if set,
// otherwise a key derived from the JWT secret.
func backupKey(jwtSecret, configured string) []byte {
if configured != "" {
if len(configured) >= 32 {
return []byte(configured)[:32]
}
sum := sha256.Sum256([]byte(configured))
return sum[:]
}
sum := sha256.Sum256([]byte(jwtSecret + ":backup"))
return sum[:]
}

// newAEAD derives the AES-256-GCM AEAD for backup archives.
func newAEAD(jwtSecret, configured string) (cipher.AEAD, error) {
block, err := aes.NewCipher(backupKey(jwtSecret, configured))
if err != nil {
return nil, err
}
return cipher.NewGCM(block)
}

// encryptFile encrypts a file with AES-256-GCM and writes it to path+".enc",
// removing the plaintext source.
func encryptFile(path, jwtSecret, configured string) (string, error) {
plain, err := os.ReadFile(path)
if err != nil {
return "", err
}
sealed, err := sealBytes(plain, jwtSecret, configured)
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

// decryptFile decrypts a ".enc" backup archive, returning the plaintext.
func decryptFile(path, jwtSecret, configured string) ([]byte, error) {
sealed, err := os.ReadFile(path)
if err != nil {
return nil, err
}
return openBytes(sealed, jwtSecret, configured)
}

func sealBytes(plain []byte, jwtSecret, configured string) ([]byte, error) {
aead, err := newAEAD(jwtSecret, configured)
if err != nil {
return nil, err
}
nonce := make([]byte, aead.NonceSize())
if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
return nil, err
}
return aead.Seal(nonce, nonce, plain, nil), nil
}

func openBytes(sealed []byte, jwtSecret, configured string) ([]byte, error) {
aead, err := newAEAD(jwtSecret, configured)
if err != nil {
return nil, err
}
if len(sealed) < aead.NonceSize() {
return nil, fmt.Errorf("ciphertext too short")
}
nonce, body := sealed[:aead.NonceSize()], sealed[aead.NonceSize():]
return aead.Open(nil, nonce, body, nil)
}