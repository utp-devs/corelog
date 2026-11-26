package domain

import (
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

const MaxAttachmentSize int64 = 10 << 20

var ErrInvalidAttachment = apperror.WithCode(apperror.KindValidation, "issue.invalid_attachment", "unsupported attachment")

type Attachment struct {
	ID          uuid.UUID
	IssueID     uuid.UUID
	UploaderID  uuid.UUID
	FileName    string
	ContentType string
	Size        int64
	StorageKey  string
	CreatedAt   time.Time
}

func NewAttachment(issueID, uploaderID uuid.UUID, fileName, contentType string, size int64) (*Attachment, error) {
	fileName = strings.TrimSpace(filepath.Base(strings.ReplaceAll(fileName, "\\", "/")))
	if issueID == uuid.Nil || uploaderID == uuid.Nil || fileName == "" || fileName == "." || fileName == ".." || !utf8.ValidString(fileName) || len(fileName) > 255 || size < 1 || size > MaxAttachmentSize {
		return nil, ErrInvalidAttachment
	}
	for _, r := range fileName {
		if r < 32 || r == 127 {
			return nil, ErrInvalidAttachment
		}
	}
	allowed := map[string]bool{"application/pdf": true, "image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true, "text/plain": true, "text/csv": true}
	if !allowed[contentType] {
		return nil, ErrInvalidAttachment
	}
	now := time.Now().UTC()
	return &Attachment{IssueID: issueID, UploaderID: uploaderID, FileName: fileName, ContentType: contentType, Size: size, StorageKey: uuid.NewString(), CreatedAt: now}, nil
}
