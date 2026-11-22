package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewAttachmentValidatesMetadataAndSanitizesName(t *testing.T) {
	issueID, userID := uuid.New(), uuid.New()
	got, err := NewAttachment(issueID, userID, `..\private\report.pdf`, "application/pdf", 16)
	if err != nil {
		t.Fatalf("NewAttachment() error = %v", err)
	}
	if got.FileName != "report.pdf" {
		t.Fatalf("FileName = %q, want report.pdf", got.FileName)
	}
	if got.ID == uuid.Nil || got.StorageKey == "" {
		t.Fatal("attachment identifiers must be set")
	}
}

func TestNewAttachmentRejectsInvalidSizeAndActiveContent(t *testing.T) {
	for _, tc := range []struct {
		name, contentType string
		size              int64
	}{
		{"empty", "text/plain", 0}, {"too large", "text/plain", MaxAttachmentSize + 1}, {"html", "text/html", 10}, {"svg", "image/svg+xml", 10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewAttachment(uuid.New(), uuid.New(), "file", tc.contentType, tc.size); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}
