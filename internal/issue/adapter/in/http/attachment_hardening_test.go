package http_test

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	issuehttp "github.com/jhonsferg/corelog/internal/issue/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/issue/application/usecase"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/authtoken"
)

func TestAttachmentHardening(t *testing.T) {
	secret := "test-secret-key-that-is-long-enough-32-bytes"
	repo := newMemoryIssueRepo()
	author := uuid.New()
	issue, err := domain.NewIssue("Hardening", "description", domain.PriorityMedium, author)
	if err != nil {
		t.Fatal(err)
	}
	issue.ProjectID = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	if err = repo.Save(context.Background(), issue); err != nil {
		t.Fatal(err)
	}
	attachments := &testAttachments{items: map[uuid.UUID]*domain.Attachment{}}
	files := &testFileStorage{files: map[string][]byte{}}
	handler := issuehttp.NewHandler(usecase.NewIssueService(repo, newMemoryDirectory(), projectRoleResolver{}), secret).WithAttachments(attachments, files)
	router := chi.NewRouter()
	handler.RegisterRoutes(router)
	token, err := authtoken.Generate(secret, time.Hour, author, "contributor")
	if err != nil {
		t.Fatal(err)
	}
	upload := func(name, content string) *httptest.ResponseRecorder {
		var body bytes.Buffer
		w := multipart.NewWriter(&body)
		part, _ := w.CreateFormFile("file", name)
		_, _ = io.Copy(part, strings.NewReader(content))
		_ = w.Close()
		req := httptest.NewRequest(http.MethodPost, "/issues/"+issue.ID.String()+"/attachments", &body)
		req.Header.Set("Content-Type", w.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	// The content decides the type, not the extension: HTML named .txt is refused.
	if rec := upload("innocent.txt", "<!DOCTYPE html><html><script>alert(1)</script></html>"); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("html disguised as txt: %d %s", rec.Code, rec.Body)
	}
	// Path traversal in the file name never reaches the storage key or the stored name.
	if rec := upload("../../etc/passwd.txt", "plain text"); rec.Code != http.StatusCreated {
		t.Fatalf("traversal name upload: %d %s", rec.Code, rec.Body)
	}
	var stored *domain.Attachment
	for _, a := range attachments.items {
		stored = a
	}
	if stored == nil || strings.ContainsAny(stored.FileName, `/\`) || strings.Contains(stored.StorageKey, "..") || strings.ContainsAny(stored.StorageKey, `/\`) {
		t.Fatalf("stored attachment = %+v", stored)
	}

	req := httptest.NewRequest(http.MethodGet, "/issues/"+issue.ID.String()+"/attachments/"+stored.ID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("download: %d", rec.Code)
	}
	if !strings.HasPrefix(rec.Header().Get("Content-Disposition"), "attachment") {
		t.Fatalf("Content-Disposition = %q, want attachment", rec.Header().Get("Content-Disposition"))
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("download must set X-Content-Type-Options: nosniff")
	}
}
