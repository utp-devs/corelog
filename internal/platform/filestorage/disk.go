package filestorage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
)

// keyPattern accepts exactly the output of NewKey. Anything else (separators, dots, drive letters,
// empty strings) is rejected before a path is built, which makes traversal impossible by construction.
var keyPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// DiskStorage keeps objects as files inside a single directory.
type DiskStorage struct {
	root string
}

var _ Storage = (*DiskStorage)(nil)

// NewDiskStorage returns a storage rooted at dir. The directory is created on the first Save.
func NewDiskStorage(dir string) *DiskStorage {
	return &DiskStorage{root: dir}
}

func (s *DiskStorage) path(key string) (string, error) {
	if !keyPattern.MatchString(key) {
		return "", ErrInvalidKey
	}
	return filepath.Join(s.root, key), nil
}

func ctxErr(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// Save writes r to a new file. The file is created exclusively, and removed again if the copy fails.
func (s *DiskStorage) Save(ctx context.Context, key string, r io.Reader) error {
	p, err := s.path(key)
	if err != nil {
		return err
	}
	if err := ctxErr(ctx); err != nil {
		return err
	}
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return fmt.Errorf("filestorage: create directory: %w", err)
	}
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("filestorage: create object: %w", err)
	}
	_, copyErr := io.Copy(f, &ctxReader{ctx: ctx, r: r})
	closeErr := f.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(p)
		return errors.Join(copyErr, closeErr)
	}
	return nil
}

// Open returns the object content.
func (s *DiskStorage) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	p, err := s.path(key)
	if err != nil {
		return nil, err
	}
	if err := ctxErr(ctx); err != nil {
		return nil, err
	}
	f, err := os.Open(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNotFound
	}
	return f, err
}

// Delete removes the object. A missing object is not an error.
func (s *DiskStorage) Delete(ctx context.Context, key string) error {
	p, err := s.path(key)
	if err != nil {
		return err
	}
	if err := ctxErr(ctx); err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// Stat returns the size and modification time of the object.
func (s *DiskStorage) Stat(ctx context.Context, key string) (Info, error) {
	p, err := s.path(key)
	if err != nil {
		return Info{}, err
	}
	if err := ctxErr(ctx); err != nil {
		return Info{}, err
	}
	fi, err := os.Stat(p)
	if errors.Is(err, fs.ErrNotExist) {
		return Info{}, ErrNotFound
	}
	if err != nil {
		return Info{}, err
	}
	return Info{Size: fi.Size(), ModTime: fi.ModTime()}, nil
}

type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (c *ctxReader) Read(p []byte) (int, error) {
	if err := ctxErr(c.ctx); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}
