package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	issuehttp "github.com/jhonsferg/corelog/internal/issue/adapter/in/http"
	issueport "github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/authtoken"
)

// stubComments answers the comment port with canned results, so the HTTP tests cover routing, decoding and status
// mapping without the use cases.
type stubComments struct {
	page       *issueport.ThreadPage
	result     *issueport.CommentResult
	reactions  *issueport.CommentReactions
	revisions  []domain.CommentRevision
	err        error
	lastCreate issueport.CommentCreateInput
	lastSort   string
}

func newStubComments() *stubComments {
	return &stubComments{page: &issueport.ThreadPage{Threads: []*issueport.CommentView{}}}
}

func (s *stubComments) ListThreads(_ context.Context, _ issueport.CommentActor, _ uuid.UUID, q issueport.ThreadPageInput) (*issueport.ThreadPage, error) {
	s.lastSort = q.Sort
	return s.page, s.err
}

func (s *stubComments) CreateComment(_ context.Context, _ issueport.CommentActor, in issueport.CommentCreateInput) (*issueport.CommentResult, error) {
	s.lastCreate = in
	return s.result, s.err
}

func (s *stubComments) UpdateComment(_ context.Context, _ issueport.CommentActor, _ uuid.UUID, _ string) (*issueport.CommentResult, error) {
	return s.result, s.err
}

func (s *stubComments) DeleteComment(context.Context, issueport.CommentActor, uuid.UUID) error {
	return s.err
}

func (s *stubComments) ListRevisions(context.Context, issueport.CommentActor, uuid.UUID) ([]domain.CommentRevision, error) {
	return s.revisions, s.err
}

func (s *stubComments) SetReaction(_ context.Context, _ issueport.CommentActor, _ uuid.UUID, _ string, _ bool) (*issueport.CommentReactions, error) {
	return s.reactions, s.err
}

func commentRouter(stub *stubComments) (*chi.Mux, string) {
	secret := "test-secret-key-that-is-long-enough-32-bytes"
	r := chi.NewRouter()
	issuehttp.NewHandler(nil, secret).WithComments(stub).RegisterRoutes(r)
	return r, secret
}

func commentRequest(t *testing.T, r http.Handler, secret, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if secret != "" {
		token, err := authtoken.Generate(secret, time.Hour, uuid.New(), "user")
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func sampleComment(body string) *domain.Comment {
	c, _ := domain.NewComment(uuid.New(), uuid.New(), body, time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC))
	c.ID = uuid.New()
	return c
}

func commentErrorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body %s: %v", rec.Body.String(), err)
	}
	return body.Error.Code
}

func TestCommentRoutesRequireAuthentication(t *testing.T) {
	r, _ := commentRouter(newStubComments())
	issue := uuid.New().String()
	cases := []struct{ method, path, body string }{
		{http.MethodGet, "/issues/" + issue + "/comments", ""},
		{http.MethodPost, "/issues/" + issue + "/comments", `{"body":"x"}`},
		{http.MethodPatch, "/comments/" + uuid.New().String(), `{"body":"x"}`},
		{http.MethodDelete, "/comments/" + uuid.New().String(), ""},
		{http.MethodGet, "/comments/" + uuid.New().String() + "/revisions", ""},
		{http.MethodPut, "/comments/" + uuid.New().String() + "/reactions/heart", ""},
	}
	for _, tc := range cases {
		if rec := commentRequest(t, r, "", tc.method, tc.path, tc.body); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without token = %d, want 401", tc.method, tc.path, rec.Code)
		}
	}
}

func TestListCommentsReturnsThreadsWithCursorMeta(t *testing.T) {
	stub := newStubComments()
	root := sampleComment("Raiz")
	reply := sampleComment("Respuesta")
	reply.ParentID = &root.ID
	stub.page = &issueport.ThreadPage{
		Threads: []*issueport.CommentView{{
			Comment: root, AuthorName: "Ana", ReplyCount: 1,
			Replies:   []*issueport.CommentView{{Comment: reply, AuthorName: "Bob"}},
			Reactions: []issueport.ReactionCount{{Emoji: "heart", Count: 2, ViewerReacted: true}},
		}},
		NextCursor: "siguiente",
	}
	r, secret := commentRouter(stub)
	rec := commentRequest(t, r, secret, http.MethodGet, "/issues/"+uuid.New().String()+"/comments?sort=desc&limit=1", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	if stub.lastSort != "desc" {
		t.Errorf("sort not forwarded: %q", stub.lastSort)
	}
	var body struct {
		Data []struct {
			ID         string `json:"id"`
			ReplyCount int    `json:"reply_count"`
			Author     struct {
				Name string `json:"name"`
			} `json:"author"`
			Replies []struct {
				ParentID *string `json:"parent_id"`
				Body     string  `json:"body"`
			} `json:"replies"`
			Reactions []struct {
				Emoji         string `json:"emoji"`
				Count         int    `json:"count"`
				ViewerReacted bool   `json:"viewer_reacted"`
			} `json:"reactions"`
			Visibility string `json:"visibility"`
		} `json:"data"`
		Meta struct {
			Limit      int     `json:"limit"`
			NextCursor *string `json:"next_cursor"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v in %s", err, rec.Body.String())
	}
	if len(body.Data) != 1 || body.Data[0].ReplyCount != 1 || len(body.Data[0].Replies) != 1 {
		t.Fatalf("threads = %+v", body.Data)
	}
	if body.Data[0].Author.Name != "Ana" || body.Data[0].Visibility != "public" {
		t.Errorf("author or visibility = %+v", body.Data[0])
	}
	if body.Data[0].Replies[0].ParentID == nil || body.Data[0].Replies[0].Body != "Respuesta" {
		t.Errorf("reply = %+v", body.Data[0].Replies[0])
	}
	if len(body.Data[0].Reactions) != 1 || body.Data[0].Reactions[0].Emoji != "heart" || !body.Data[0].Reactions[0].ViewerReacted {
		t.Errorf("reactions = %+v", body.Data[0].Reactions)
	}
	if body.Meta.Limit != 1 || body.Meta.NextCursor == nil || *body.Meta.NextCursor != "siguiente" {
		t.Errorf("meta = %+v", body.Meta)
	}
}

func TestListCommentsStatusMapping(t *testing.T) {
	stub := newStubComments()
	r, secret := commentRouter(stub)
	issue := "/issues/" + uuid.New().String() + "/comments"

	if rec := commentRequest(t, r, secret, http.MethodGet, issue+"?limit=abc", ""); rec.Code != http.StatusUnprocessableEntity || commentErrorCode(t, rec) != "issue.comment_limit_invalid" {
		t.Errorf("limit=abc: %d %s", rec.Code, rec.Body.String())
	}
	stub.err = apperror.WithCode(apperror.KindNotFound, "issue.not_found", "issue not found")
	if rec := commentRequest(t, r, secret, http.MethodGet, issue, ""); rec.Code != http.StatusNotFound {
		t.Errorf("hidden issue: %d", rec.Code)
	}
}

func TestCreateCommentStatusesAndStrictBody(t *testing.T) {
	stub := newStubComments()
	stub.result = &issueport.CommentResult{Comment: &issueport.CommentView{Comment: sampleComment("Hola"), AuthorName: "Ana"}}
	r, secret := commentRouter(stub)
	issue := "/issues/" + uuid.New().String() + "/comments"

	rec := commentRequest(t, r, secret, http.MethodPost, issue, `{"body":"Hola","visibility":"internal","internal_role":"lead"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body %s", rec.Code, rec.Body.String())
	}
	if stub.lastCreate.Visibility != "internal" || stub.lastCreate.InternalRole != "lead" {
		t.Errorf("visibility not forwarded: %+v", stub.lastCreate)
	}
	var created struct {
		Data struct {
			Warnings []any  `json:"warnings"`
			Body     string `json:"body"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil || created.Data.Body != "Hola" || created.Data.Warnings == nil {
		t.Errorf("create body = %s (%v)", rec.Body.String(), err)
	}

	if rec := commentRequest(t, r, secret, http.MethodPost, issue, `{"body":"x","surprise":true}`); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("unknown field accepted: %d", rec.Code)
	}
	if rec := commentRequest(t, r, secret, http.MethodPost, issue, `{"body":"x","parent_id":"no-es-uuid"}`); rec.Code != http.StatusUnprocessableEntity || commentErrorCode(t, rec) != "issue.comment_parent_invalid" {
		t.Errorf("bad parent: %d %s", rec.Code, rec.Body.String())
	}
	if rec := commentRequest(t, r, secret, http.MethodPost, issue, `not json`); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("bad json: %d", rec.Code)
	}

	stub.err = apperror.WithCode(apperror.KindForbidden, "issue.comment_forbidden", "forbidden")
	if rec := commentRequest(t, r, secret, http.MethodPost, issue, `{"body":"x"}`); rec.Code != http.StatusForbidden {
		t.Errorf("viewer comments: %d", rec.Code)
	}
	stub.err = apperror.WithCode(apperror.KindConflict, "project.archived", "archived")
	if rec := commentRequest(t, r, secret, http.MethodPost, issue, `{"body":"x"}`); rec.Code != http.StatusConflict {
		t.Errorf("archived: %d", rec.Code)
	}
}

func TestEditAndDeleteCommentStatuses(t *testing.T) {
	stub := newStubComments()
	stub.result = &issueport.CommentResult{Comment: &issueport.CommentView{Comment: sampleComment("Editado"), AuthorName: "Ana"}}
	r, secret := commentRouter(stub)
	path := "/comments/" + uuid.New().String()

	if rec := commentRequest(t, r, secret, http.MethodPatch, path, `{"body":"Editado"}`); rec.Code != http.StatusOK {
		t.Errorf("patch: %d %s", rec.Code, rec.Body.String())
	}
	if rec := commentRequest(t, r, secret, http.MethodPatch, "/comments/no-uuid", `{"body":"x"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("patch bad id: %d", rec.Code)
	}
	if rec := commentRequest(t, r, secret, http.MethodDelete, path, ""); rec.Code != http.StatusNoContent {
		t.Errorf("delete: %d", rec.Code)
	}
	stub.err = domain.ErrCommentNotFound
	if rec := commentRequest(t, r, secret, http.MethodDelete, path, ""); rec.Code != http.StatusNotFound || commentErrorCode(t, rec) != "issue.comment_not_found" {
		t.Errorf("delete missing: %d %s", rec.Code, rec.Body.String())
	}
	stub.err = domain.ErrCommentForbidden
	if rec := commentRequest(t, r, secret, http.MethodPatch, path, `{"body":"x"}`); rec.Code != http.StatusForbidden {
		t.Errorf("edit others: %d", rec.Code)
	}
}

func TestCommentRevisionsAndReactionsHTTP(t *testing.T) {
	stub := newStubComments()
	r, secret := commentRouter(stub)
	id := uuid.New()
	stub.revisions = []domain.CommentRevision{{ID: uuid.New(), CommentID: id, Body: "antes", EditedBy: uuid.New(), EditedAt: time.Now()}}
	rec := commentRequest(t, r, secret, http.MethodGet, "/comments/"+id.String()+"/revisions", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("revisions: %d %s", rec.Code, rec.Body.String())
	}
	var revisions struct {
		Data []struct {
			Body string `json:"body"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &revisions); err != nil || len(revisions.Data) != 1 || revisions.Data[0].Body != "antes" {
		t.Errorf("revisions body = %s", rec.Body.String())
	}

	stub.reactions = &issueport.CommentReactions{CommentID: id, Reactions: []issueport.ReactionCount{{Emoji: "heart", Count: 1, ViewerReacted: true}}}
	rec = commentRequest(t, r, secret, http.MethodPut, "/comments/"+id.String()+"/reactions/heart", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("put reaction: %d %s", rec.Code, rec.Body.String())
	}
	var reactions struct {
		Data struct {
			CommentID string `json:"comment_id"`
			Reactions []struct {
				Emoji         string `json:"emoji"`
				Count         int    `json:"count"`
				ViewerReacted bool   `json:"viewer_reacted"`
			} `json:"reactions"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &reactions); err != nil || reactions.Data.CommentID != id.String() || !reactions.Data.Reactions[0].ViewerReacted {
		t.Errorf("reactions body = %s", rec.Body.String())
	}
	stub.reactions = &issueport.CommentReactions{CommentID: id, Reactions: []issueport.ReactionCount{}}
	if rec := commentRequest(t, r, secret, http.MethodDelete, "/comments/"+id.String()+"/reactions/heart", ""); rec.Code != http.StatusOK {
		t.Errorf("delete reaction: %d", rec.Code)
	}
	stub.err = domain.ErrCommentReactionInvalid
	if rec := commentRequest(t, r, secret, http.MethodPut, "/comments/"+id.String()+"/reactions/laugh", ""); rec.Code != http.StatusUnprocessableEntity || commentErrorCode(t, rec) != "issue.comment_reaction_invalid" {
		t.Errorf("unknown emoji: %d %s", rec.Code, rec.Body.String())
	}
}

var _ = context.Background
