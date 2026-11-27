// Package crypto provides authenticated encryption for secrets stored at rest.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// KeySize is the key length in bytes required by AES-256.
const KeySize = 32

// ErrInvalidKey is returned when a key is not 32 bytes after decoding.
var ErrInvalidKey = errors.New("crypto: key must be 32 bytes")

// ErrOpenFailed is returned when a sealed value cannot be authenticated: wrong key, tampered data or
// different additional data.
var ErrOpenFailed = errors.New("crypto: sealed value could not be opened")

// Cipher encrypts and decrypts values with AES-256-GCM. Each Seal call uses a fresh random nonce, which
// is stored as the prefix of the sealed value.
type Cipher struct {
	aead cipher.AEAD
}

// NewCipher builds a cipher from a 32-byte key.
func NewCipher(key []byte) (*Cipher, error) {
	if len(key) != KeySize {
		return nil, ErrInvalidKey
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: aes: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: gcm: %w", err)
	}
	return &Cipher{aead: aead}, nil
}

// Seal encrypts plaintext and binds it to additional. The result is nonce || ciphertext.
func (c *Cipher) Seal(plaintext, additional []byte) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("crypto: nonce: %w", err)
	}
	return c.aead.Seal(nonce, nonce, plaintext, additional), nil
}

// Open decrypts a value produced by Seal with the same additional data.
func (c *Cipher) Open(sealed, additional []byte) ([]byte, error) {
	ns := c.aead.NonceSize()
	if len(sealed) < ns+c.aead.Overhead() {
		return nil, ErrOpenFailed
	}
	plaintext, err := c.aead.Open(nil, sealed[:ns], sealed[ns:], additional)
	if err != nil {
		return nil, ErrOpenFailed
	}
	return plaintext, nil
}

// ParseKey decodes a standard base64 encoded 32-byte key.
func ParseKey(encoded string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, ErrInvalidKey
	}
	if len(key) != KeySize {
		return nil, ErrInvalidKey
	}
	return key, nil
}

// DeriveKey derives a 32-byte key from secret with HKDF-SHA256. The info string separates keys
// derived from the same secret for different purposes.
func DeriveKey(secret []byte, info string) ([]byte, error) {
	key, err := hkdf.Key(sha256.New, secret, nil, info, KeySize)
	if err != nil {
		return nil, fmt.Errorf("crypto: derive key: %w", err)
	}
	return key, nil
}
