package filesystem

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestStorageSaveOpenDelete(t *testing.T) {
	storage := New(filepath.Join(t.TempDir(), "files"))
	key := uuid.NewString()
	size, err := storage.Save(context.Background(), key, strings.NewReader("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if size != 5 {
		t.Fatalf("size=%d", size)
	}
	f, err := storage.Open(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(f)
	_ = f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Fatalf("data=%q", data)
	}
	if err := storage.Delete(context.Background(), key); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(storage.root, key)); !os.IsNotExist(err) {
		t.Fatalf("expected deleted file, got %v", err)
	}
}

func TestStorageRejectsTraversalKeyAndOversizedFile(t *testing.T) {
	storage := New(t.TempDir())
	if _, err := storage.Save(context.Background(), "../escape", strings.NewReader("x")); err == nil {
		t.Fatal("expected invalid key error")
	}
	tooLarge := io.LimitReader(strings.NewReader(strings.Repeat("x", (10<<20)+1)), (10<<20)+1)
	if _, err := storage.Save(context.Background(), uuid.NewString(), tooLarge); err == nil {
		t.Fatal("expected size limit error")
	}
	entries, err := os.ReadDir(storage.root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("partial file was not removed: %v", entries)
	}
}
