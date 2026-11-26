package filestorage

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validKey = "0123456789abcdef0123456789abcdef"

func TestDiskStorageRejectsInvalidKeys(t *testing.T) {
	root := t.TempDir()
	s := NewDiskStorage(filepath.Join(root, "objects"))
	ctx := context.Background()

	keys := []string{
		"",
		"../x",
		"..\\x",
		"../../etc/passwd",
		"/etc/passwd",
		"C:\\Windows\\win.ini",
		"a/b",
		"0123456789abcdef0123456789abcde",   // 31 chars
		"0123456789abcdef0123456789abcdef0", // 33 chars
		"0123456789ABCDEF0123456789ABCDEF",  // uppercase
		"0123456789abcdef0123456789abcdeg",  // non hex
		validKey + "\n",
		"..",
	}
	for _, key := range keys {
		if err := s.Save(ctx, key, strings.NewReader("x")); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("Save(%q) err = %v, want ErrInvalidKey", key, err)
		}
		if _, err := s.Open(ctx, key); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("Open(%q) err = %v, want ErrInvalidKey", key, err)
		}
		if err := s.Delete(ctx, key); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("Delete(%q) err = %v, want ErrInvalidKey", key, err)
		}
		if _, err := s.Stat(ctx, key); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("Stat(%q) err = %v, want ErrInvalidKey", key, err)
		}
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Fatalf("rejected keys must not create anything, found %d entries", len(entries))
	}
}

func TestDiskStorageSaveOpenStatDelete(t *testing.T) {
	s := NewDiskStorage(filepath.Join(t.TempDir(), "objects"))
	ctx := context.Background()

	if err := s.Save(ctx, validKey, strings.NewReader("hello")); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := s.Save(ctx, validKey, strings.NewReader("again")); err == nil {
		t.Fatal("Save over an existing key must fail")
	}

	info, err := s.Stat(ctx, validKey)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Size != 5 || info.ModTime.IsZero() {
		t.Fatalf("Stat = %+v, want size 5 and a modtime", info)
	}

	rc, err := s.Open(ctx, validKey)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	got, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil || string(got) != "hello" {
		t.Fatalf("content = %q, err = %v", got, err)
	}

	if err := s.Delete(ctx, validKey); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := s.Delete(ctx, validKey); err != nil {
		t.Fatalf("second Delete must be a no-op, got %v", err)
	}
	if _, err := s.Open(ctx, validKey); !errors.Is(err, ErrNotFound) {
		t.Errorf("Open after delete err = %v, want ErrNotFound", err)
	}
	if _, err := s.Stat(ctx, validKey); !errors.Is(err, ErrNotFound) {
		t.Errorf("Stat after delete err = %v, want ErrNotFound", err)
	}
}

func TestDiskStorageCanceledContext(t *testing.T) {
	s := NewDiskStorage(t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.Save(ctx, validKey, strings.NewReader("x")); !errors.Is(err, context.Canceled) {
		t.Errorf("Save err = %v, want context.Canceled", err)
	}
}

func TestNewKeyIsValidAndUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		k, err := NewKey()
		if err != nil {
			t.Fatal(err)
		}
		if !keyPattern.MatchString(k) {
			t.Fatalf("NewKey() = %q does not match the key pattern", k)
		}
		if seen[k] {
			t.Fatalf("duplicate key %q", k)
		}
		seen[k] = true
	}
}
