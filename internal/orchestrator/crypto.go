package orchestrator

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"
)

func deriveKey(secret string) []byte {
	if secret == "" {
		digest := sha256.Sum256([]byte("default-secret"))
		return digest[:]
	}
	digest := sha256.Sum256([]byte(secret))
	return digest[:]
}

func EncryptSession(plaintext string, key []byte) ([]byte, error) {
	if len(key) == 0 {
		return nil, errors.New("secret key required")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return ciphertext, nil
}

func DecryptSession(ciphertext, key []byte) (string, error) {
	if len(ciphertext) == 0 {
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
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	nonce := ciphertext[:nonceSize]
	data := ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
