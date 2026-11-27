package crypto

import (
	"bytes"
	"encoding/base64"
	"errors"
	"testing"
)

func testKey(t *testing.T, fill byte) []byte {
	t.Helper()
	key := bytes.Repeat([]byte{fill}, KeySize)
	return key
}

func TestSealOpenRoundTrip(t *testing.T) {
	c, err := NewCipher(testKey(t, 1))
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	plain := []byte("JBSWY3DPEHPK3PXP")
	aad := []byte("user-1")

	sealed, err := c.Seal(plain, aad)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if bytes.Contains(sealed, plain) {
		t.Fatal("sealed value contains plaintext")
	}
	got, err := c.Open(sealed, aad)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("round trip mismatch: %q", got)
	}
}

func TestSealUsesFreshNonce(t *testing.T) {
	c, _ := NewCipher(testKey(t, 1))
	a, _ := c.Seal([]byte("same"), nil)
	b, _ := c.Seal([]byte("same"), nil)
	if bytes.Equal(a, b) {
		t.Fatal("two seals of the same plaintext must differ")
	}
}

func TestOpenRejectsWrongKeyAndWrongAdditionalData(t *testing.T) {
	c1, _ := NewCipher(testKey(t, 1))
	c2, _ := NewCipher(testKey(t, 2))
	sealed, _ := c1.Seal([]byte("secret"), []byte("user-1"))

	if _, err := c2.Open(sealed, []byte("user-1")); !errors.Is(err, ErrOpenFailed) {
		t.Fatalf("wrong key: want ErrOpenFailed, got %v", err)
	}
	if _, err := c1.Open(sealed, []byte("user-2")); !errors.Is(err, ErrOpenFailed) {
		t.Fatalf("wrong additional data: want ErrOpenFailed, got %v", err)
	}
	tampered := append([]byte(nil), sealed...)
	tampered[len(tampered)-1] ^= 0x01
	if _, err := c1.Open(tampered, []byte("user-1")); !errors.Is(err, ErrOpenFailed) {
		t.Fatalf("tampered value: want ErrOpenFailed, got %v", err)
	}
	if _, err := c1.Open([]byte("short"), nil); !errors.Is(err, ErrOpenFailed) {
		t.Fatalf("short value: want ErrOpenFailed, got %v", err)
	}
}

func TestNewCipherRejectsBadKeyLength(t *testing.T) {
	if _, err := NewCipher(make([]byte, 16)); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("want ErrInvalidKey, got %v", err)
	}
}

func TestParseKey(t *testing.T) {
	good := base64.StdEncoding.EncodeToString(testKey(t, 7))
	if key, err := ParseKey(good); err != nil || len(key) != KeySize {
		t.Fatalf("ParseKey good: len=%d err=%v", len(key), err)
	}
	if _, err := ParseKey("not base64!!"); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("invalid base64: want ErrInvalidKey, got %v", err)
	}
	short := base64.StdEncoding.EncodeToString(make([]byte, 31))
	if _, err := ParseKey(short); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("short key: want ErrInvalidKey, got %v", err)
	}
}

func TestDeriveKeyIsStableAndPurposeSeparated(t *testing.T) {
	secret := []byte("jwt-secret")
	a, err := DeriveKey(secret, "purpose-a")
	if err != nil || len(a) != KeySize {
		t.Fatalf("DeriveKey: len=%d err=%v", len(a), err)
	}
	again, _ := DeriveKey(secret, "purpose-a")
	if !bytes.Equal(a, again) {
		t.Fatal("derived key must be stable")
	}
	b, _ := DeriveKey(secret, "purpose-b")
	if bytes.Equal(a, b) {
		t.Fatal("different purposes must derive different keys")
	}
}
