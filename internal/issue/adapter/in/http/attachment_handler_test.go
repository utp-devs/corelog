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

type testAttachments struct {
	items map[uuid.UUID]*domain.Attachment
}

func (r *testAttachments) SaveAttachment(_ context.Context, a *domain.Attachment) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	r.items[a.ID] = a
	return nil
}
func (r *testAttachments) FindAttachment(_ context.Context, issueID, id uuid.UUID) (*domain.Attachment, error) {
	a := r.items[id]
	if a == nil || a.IssueID != issueID {
		return nil, domain.ErrIssueNotFound
	}
	return a, nil
}
func (r *testAttachments) ListAttachments(_ context.Context, issueID uuid.UUID) ([]*domain.Attachment, error) {
	out := []*domain.Attachment{}
	for _, a := range r.items {
		if a.IssueID == issueID {
			out = append(out, a)
		}
	}
	return out, nil
}
func (r *testAttachments) DeleteAttachment(_ context.Context, issueID, id uuid.UUID) error {
	delete(r.items, id)
	return nil
}

type testFileStorage struct{ files map[string][]byte }

type viewerRoleResolver struct{}

func (viewerRoleResolver) FindRole(context.Context, uuid.UUID, uuid.UUID) (*string, error) {
	role := "viewer"
	return &role, nil
}
func (viewerRoleResolver) ListProjectIDs(context.Context, uuid.UUID) ([]uuid.UUID, error) {
	return []uuid.UUID{uuid.MustParse("00000000-0000-4000-8000-000000000001")}, nil
}

func (s *testFileStorage) Save(_ context.Context, key string, src io.Reader) (int64, error) {
	b, e := io.ReadAll(src)
	if e == nil {
		s.files[key] = b
	}
	return int64(len(b)), e
}
func (s *testFileStorage) Open(_ context.Context, key string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(s.files[key])), nil
}
func (s *testFileStorage) Delete(_ context.Context, key string) error {
	delete(s.files, key)
	return nil
}

func TestAttachmentHTTPUploadListAndDownload(t *testing.T) {
	secret := "test-secret-key-that-is-long-enough-32-bytes"
	repo := newMemoryIssueRepo()
	author := uuid.New()
	issue, err := domain.NewIssue("Upload test", "description", domain.PriorityMedium, author)
	if err != nil {
		t.Fatal(err)
	}
	issue.ProjectID = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	if err = repo.Save(context.Background(), issue); err != nil {
		t.Fatal(err)
	}
	attachments := &testAttachments{items: map[uuid.UUID]*domain.Attachment{}}
	files := &testFileStorage{files: map[string][]byte{}}
	service := usecase.NewIssueService(repo, newMemoryDirectory(), projectRoleResolver{})
	handler := issuehttp.NewHandler(service, secret).WithAttachments(attachments, files)
	router := chi.NewRouter()
	handler.RegisterRoutes(router)
	token, err := authtoken.Generate(secret, time.Hour, author, "contributor")
	if err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "note.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(part, strings.NewReader("attachment body"))
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/issues/"+issue.ID.String()+"/attachments", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("upload status=%d body=%s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), `"data":{"data"`) {
		t.Fatalf("attachment response has nested data envelope: %s", response.Body.String())
	}
	items := attachments.items
	if len(items) != 1 {
		t.Fatalf("stored attachments=%d", len(items))
	}
	var stored *domain.Attachment
	for _, a := range items {
		stored = a
	}
	list := httptest.NewRequest(http.MethodGet, "/issues/"+issue.ID.String()+"/attachments", nil)
	list.Header.Set("Authorization", "Bearer "+token)
	listResponse := httptest.NewRecorder()
	router.ServeHTTP(listResponse, list)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status=%d", listResponse.Code)
	}
	download := httptest.NewRequest(http.MethodGet, "/issues/"+issue.ID.String()+"/attachments/"+stored.ID.String(), nil)
	download.Header.Set("Authorization", "Bearer "+token)
	downloadResponse := httptest.NewRecorder()
	router.ServeHTTP(downloadResponse, download)
	if downloadResponse.Code != http.StatusOK || downloadResponse.Body.String() != "attachment body" {
		t.Fatalf("download status=%d body=%q", downloadResponse.Code, downloadResponse.Body.String())
	}
	wrongIssue := httptest.NewRequest(http.MethodGet, "/issues/"+uuid.NewString()+"/attachments/"+stored.ID.String(), nil)
	wrongIssue.Header.Set("Authorization", "Bearer "+token)
	wrongIssueResponse := httptest.NewRecorder()
	router.ServeHTTP(wrongIssueResponse, wrongIssue)
	if wrongIssueResponse.Code != http.StatusNotFound {
		t.Fatalf("cross-issue download status=%d, want 404", wrongIssueResponse.Code)
	}
}

func TestAttachmentHTTPRejectsUnsupportedType(t *testing.T) {
	secret := "test-secret-key-that-is-long-enough-32-bytes"
	repo := newMemoryIssueRepo()
	author := uuid.New()
	issue, err := domain.NewIssue("Upload test", "description", domain.PriorityMedium, author)
	if err != nil {
		t.Fatal(err)
	}
	issue.ProjectID = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	_ = repo.Save(context.Background(), issue)
	service := usecase.NewIssueService(repo, newMemoryDirectory(), projectRoleResolver{})
	handler := issuehttp.NewHandler(service, secret).WithAttachments(&testAttachments{items: map[uuid.UUID]*domain.Attachment{}}, &testFileStorage{files: map[string][]byte{}})
	router := chi.NewRouter()
	handler.RegisterRoutes(router)
	token, _ := authtoken.Generate(secret, time.Hour, author, "contributor")
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("file", "payload.svg")
	_, _ = io.WriteString(part, "<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>")
	_ = writer.Close()
	request := httptest.NewRequest(http.MethodPost, "/issues/"+issue.ID.String()+"/attachments", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestAttachmentHTTPViewerCannotUpload(t *testing.T) {
	secret := "test-secret-key-that-is-long-enough-32-bytes"
	repo := newMemoryIssueRepo()
	actor := uuid.New()
	issue, err := domain.NewIssue("Viewer upload", "description", domain.PriorityMedium, actor)
	if err != nil {
		t.Fatal(err)
	}
	issue.ProjectID = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	if err := repo.Save(context.Background(), issue); err != nil {
		t.Fatal(err)
	}
	service := usecase.NewIssueService(repo, newMemoryDirectory(), viewerRoleResolver{})
	handler := issuehttp.NewHandler(service, secret).WithAttachments(&testAttachments{items: map[uuid.UUID]*domain.Attachment{}}, &testFileStorage{files: map[string][]byte{}})
	router := chi.NewRouter()
	handler.RegisterRoutes(router)
	token, err := authtoken.Generate(secret, time.Hour, actor, "requester")
	if err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("file", "note.txt")
	_, _ = io.WriteString(part, "notes")
	_ = writer.Close()
	request := httptest.NewRequest(http.MethodPost, "/issues/"+issue.ID.String()+"/attachments", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
