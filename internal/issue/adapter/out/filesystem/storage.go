package filesystem

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/jhonsferg/corelog/internal/issue/domain"
)

type Storage struct{ root string }

func New(root string) *Storage { return &Storage{root: root} }

func (s *Storage) Save(ctx context.Context, key string, src io.Reader) (int64, error) {
	if _, err := uuid.Parse(key); err != nil {
		return 0, fmt.Errorf("invalid storage key")
	}
	if err := os.MkdirAll(s.root, 0700); err != nil {
		return 0, err
	}
	path := filepath.Join(s.root, key)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return 0, err
	}
	limited := io.LimitReader(&contextReader{ctx: ctx, r: src}, domain.MaxAttachmentSize+1)
	n, copyErr := io.Copy(f, limited)
	closeErr := f.Close()
	if copyErr != nil || closeErr != nil || n > domain.MaxAttachmentSize {
		_ = os.Remove(path)
		if copyErr != nil {
			return 0, copyErr
		}
		if closeErr != nil {
			return 0, closeErr
		}
		return n, domain.ErrInvalidAttachment
	}
	return n, nil
}

func (s *Storage) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	if _, err := uuid.Parse(key); err != nil {
		return nil, fmt.Errorf("invalid storage key")
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	return os.Open(filepath.Join(s.root, key))
}

func (s *Storage) Delete(ctx context.Context, key string) error {
	if _, err := uuid.Parse(key); err != nil {
		return fmt.Errorf("invalid storage key")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	err := os.Remove(filepath.Join(s.root, key))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	default:
		return r.r.Read(p)
	}
}
