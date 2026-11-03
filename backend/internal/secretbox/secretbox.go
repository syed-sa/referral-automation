// Package secretbox encrypts small secrets (OAuth refresh tokens) with
// AES-256-GCM. Output layout: nonce (12 bytes) || ciphertext+tag.
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

type Box struct{ aead cipher.AEAD }

// New builds a Box from a base64-encoded 32-byte key.
func New(base64Key string) (*Box, error) {
	key, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil {
		return nil, fmt.Errorf("decode key: %w", err)
	}
	if len(key) != 32 {
		return nil, errors.New("key must be exactly 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead}, nil
}

// Seal encrypts plaintext. aad (additional authenticated data) is not stored
// but must be supplied again on Open; we bind tokens to the owning user id so
// a ciphertext copied to another user's row cannot be decrypted.
func (b *Box) Seal(plaintext, aad []byte) ([]byte, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return b.aead.Seal(nonce, nonce, plaintext, aad), nil
}

func (b *Box) Open(data, aad []byte) ([]byte, error) {
	n := b.aead.NonceSize()
	if len(data) < n+b.aead.Overhead() {
		return nil, errors.New("ciphertext too short")
	}
	plain, err := b.aead.Open(nil, data[:n], data[n:], aad)
	if err != nil {
		return nil, errors.New("decryption failed")
	}
	return plain, nil
}
