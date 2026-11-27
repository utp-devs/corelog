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

	authdomain "github.com/jhonsferg/corelog/internal/authorization/domain"
	issuehttp "github.com/jhonsferg/corelog/internal/issue/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/application/usecase"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/authtoken"
)

// linkStore holds links, votes and the org of the in-memory issues; it implements the link, vote and children ports.
type linkStore struct {
	issues *memoryIssueRepo
	org    uuid.UUID
	links  []*domain.IssueLink
	votes  map[uuid.UUID]map[uuid.UUID]bool
}

func (r *linkStore) FindLinkable(_ context.Context, id uuid.UUID) (port.IssueSummary, error) {
	issue, ok := r.issues.issues[id]
	if !ok {
		return port.IssueSummary{}, domain.ErrIssueNotFound
	}
	return port.IssueSummary{ID: id, ProjectID: issue.ProjectID, OrganizationID: r.org, Key: issue.Key(), Summary: issue.Summary, Status: issue.Status, TypeName: "Tarea", PriorityName: "Baja"}, nil
}

func (r *linkStore) ListLinks(ctx context.Context, id uuid.UUID) ([]port.LinkedIssue, error) {
	var out []port.LinkedIssue
	for _, l := range r.links {
		if l.SourceIssueID != id && l.TargetIssueID != id {
			continue
		}
		other := l.TargetIssueID
		if l.TargetIssueID == id {
			other = l.SourceIssueID
		}
		summary, err := r.FindLinkable(ctx, other)
		if err != nil {
			continue
		}
		out = append(out, port.LinkedIssue{LinkID: l.ID, Kind: l.Kind, Outward: l.SourceIssueID == id, Other: summary})
	}
	return out, nil
}

func (r *linkStore) FindLinkForIssue(_ context.Context, issueID, linkID uuid.UUID) (*domain.IssueLink, error) {
	for _, l := range r.links {
		if l.ID == linkID && (l.SourceIssueID == issueID || l.TargetIssueID == issueID) {
			return l, nil
		}
	}
	return nil, domain.ErrIssueLinkNotFound
}

func (r *linkStore) PairExists(_ context.Context, a, b uuid.UUID, kind domain.IssueLinkKind) (bool, error) {
	for _, l := range r.links {
		if l.Kind == kind && ((l.SourceIssueID == a && l.TargetIssueID == b) || (l.SourceIssueID == b && l.TargetIssueID == a)) {
			return true, nil
		}
	}
	return false, nil
}

func (r *linkStore) BlocksReach(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

func (r *linkStore) Save(_ context.Context, l *domain.IssueLink) error {
	l.ID = uuid.New()
	r.links = append(r.links, l)
	return nil
}

func (r *linkStore) Delete(_ context.Context, linkID uuid.UUID) error {
	for i, l := range r.links {
		if l.ID == linkID {
			r.links = append(r.links[:i], r.links[i+1:]...)
			return nil
		}
	}
	return domain.ErrIssueLinkNotFound
}

func (r *linkStore) CastVote(_ context.Context, issueID, userID uuid.UUID) (bool, error) {
	if r.votes[issueID] == nil {
		r.votes[issueID] = map[uuid.UUID]bool{}
	}
	if r.votes[issueID][userID] {
		return false, nil
	}
	r.votes[issueID][userID] = true
	r.issues.issues[issueID].VoteCount++
	return true, nil
}

func (r *linkStore) WithdrawVote(_ context.Context, issueID, userID uuid.UUID) (bool, error) {
	if !r.votes[issueID][userID] {
		return false, nil
	}
	delete(r.votes[issueID], userID)
	r.issues.issues[issueID].VoteCount--
	return true, nil
}

func (r *linkStore) HasVoted(_ context.Context, issueID, userID uuid.UUID) (bool, error) {
	return r.votes[issueID][userID], nil
}

func (r *linkStore) ListChildren(_ context.Context, parentID uuid.UUID) ([]*domain.Issue, error) {
	var out []*domain.Issue
	for _, issue := range r.issues.issues {
		if issue.ParentID != nil && *issue.ParentID == parentID {
			out = append(out, issue)
		}
	}
	return out, nil
}

func TestIssueHTTP_LinksVotesAndChildren(t *testing.T) {
	const secret = "test-secret-key-that-is-long-enough-32-bytes"
	contributor, viewer, outsider := uuid.New(), uuid.New(), uuid.New()
	auth := roleAuthorizer{roles: map[uuid.UUID]authdomain.ProjectRole{
		contributor: authdomain.ProjectRoleContributor, viewer: authdomain.ProjectRoleViewer,
	}}
	mem := newMemoryIssueRepo()
	repo := &linkStore{issues: mem, org: uuid.New(), votes: map[uuid.UUID]map[uuid.UUID]bool{}}
	project := uuid.New()
	newIssue := func(summary string) *domain.Issue {
		issue, err := domain.NewIssue(summary, "descripcion", domain.PriorityLow, contributor)
		if err != nil {
			t.Fatal(err)
		}
		issue.ProjectID = project
		if err := mem.Save(context.Background(), issue); err != nil {
			t.Fatal(err)
		}
		return issue
	}
	source, target := newIssue("Origen"), newIssue("Destino")
	service := usecase.NewIssueService(mem, newMemoryDirectory(), repo, auth).WithPermissions(auth)
	router := chi.NewRouter()
	issuehttp.NewHandler(service, secret).RegisterRoutes(router)
	call := func(method, path, body string, as uuid.UUID) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		if as != uuid.Nil {
			tok, _ := authtoken.Generate(secret, time.Hour, as, "user")
			req.Header.Set("Authorization", "Bearer "+tok)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	data := func(rec *httptest.ResponseRecorder) map[string]any {
		var env struct {
			Data map[string]any `json:"data"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &env)
		return env.Data
	}
	expect := func(name string, rec *httptest.ResponseRecorder, want int) {
		t.Helper()
		if rec.Code != want {
			t.Fatalf("%s: status %d, want %d: %s", name, rec.Code, want, rec.Body.String())
		}
	}

	linksPath := "/issues/" + source.ID.String() + "/links"
	body := `{"target":"` + target.ID.String() + `","kind":"blocks"}`
	expect("no token", call(http.MethodPost, linksPath, body, uuid.Nil), http.StatusUnauthorized)
	expect("viewer without edit", call(http.MethodPost, linksPath, body, viewer), http.StatusForbidden)
	expect("outsider", call(http.MethodPost, linksPath, body, outsider), http.StatusNotFound)
	expect("unknown source", call(http.MethodPost, "/issues/"+uuid.New().String()+"/links", body, contributor), http.StatusNotFound)
	expect("unknown target", call(http.MethodPost, linksPath, `{"target":"`+uuid.New().String()+`","kind":"relates"}`, contributor), http.StatusNotFound)
	expect("self link", call(http.MethodPost, linksPath, `{"target":"`+source.ID.String()+`","kind":"relates"}`, contributor), http.StatusUnprocessableEntity)
	expect("unknown kind", call(http.MethodPost, linksPath, `{"target":"`+target.ID.String()+`","kind":"nope"}`, contributor), http.StatusUnprocessableEntity)
	expect("unknown field", call(http.MethodPost, linksPath, `{"target":"`+target.ID.String()+`","kind":"relates","extra":1}`, contributor), http.StatusUnprocessableEntity)

	created := call(http.MethodPost, linksPath, body, contributor)
	expect("create link", created, http.StatusCreated)
	link := data(created)
	if link["direction"] != "outward" || link["label"] != "bloquea" || link["kind"] != "blocks" {
		t.Fatalf("created link payload = %v", link)
	}
	expect("duplicate link", call(http.MethodPost, linksPath, body, contributor), http.StatusConflict)
	expect("inverse blocks cycle is a conflict too", call(http.MethodPost, "/issues/"+target.ID.String()+"/links", `{"target":"`+source.ID.String()+`","kind":"blocks"}`, contributor), http.StatusConflict)

	listed := call(http.MethodGet, linksPath, "", contributor)
	expect("list links", listed, http.StatusOK)
	var env struct {
		Data []map[string]any `json:"data"`
	}
	_ = json.Unmarshal(listed.Body.Bytes(), &env)
	if len(env.Data) != 1 || env.Data[0]["direction"] != "outward" {
		t.Fatalf("listed links = %v", env.Data)
	}
	expect("list links hidden", call(http.MethodGet, linksPath, "", outsider), http.StatusNotFound)

	linkID, _ := link["id"].(string)
	expect("delete link", call(http.MethodDelete, linksPath+"/"+linkID, "", contributor), http.StatusNoContent)
	expect("delete link twice", call(http.MethodDelete, linksPath+"/"+linkID, "", contributor), http.StatusNotFound)

	votePath := "/issues/" + target.ID.String() + "/vote"
	expect("vote without token", call(http.MethodPut, votePath, "", uuid.Nil), http.StatusUnauthorized)
	expect("vote hidden", call(http.MethodPut, votePath, "", outsider), http.StatusNotFound)
	expect("vote", call(http.MethodPut, votePath, "", contributor), http.StatusNoContent)
	expect("vote again", call(http.MethodPut, votePath, "", contributor), http.StatusNoContent)
	view := data(call(http.MethodGet, "/issues/"+target.ID.String(), "", contributor))
	counts, _ := view["counts"].(map[string]any)
	if counts["votes"] != float64(1) || view["viewer_has_voted"] != true {
		t.Fatalf("view after vote: counts=%v viewer_has_voted=%v", counts, view["viewer_has_voted"])
	}
	expect("unvote", call(http.MethodDelete, votePath, "", contributor), http.StatusNoContent)
	expect("unvote again", call(http.MethodDelete, votePath, "", contributor), http.StatusNoContent)
	view = data(call(http.MethodGet, "/issues/"+target.ID.String(), "", contributor))
	if counts, _ = view["counts"].(map[string]any); counts["votes"] != float64(0) || view["viewer_has_voted"] != false {
		t.Fatalf("view after unvote: counts=%v viewer_has_voted=%v", counts, view["viewer_has_voted"])
	}

	closedChild := domain.Issue{}
	_ = closedChild
	doneChild := newIssue("Hecha")
	doneChild.ParentID = &source.ID
	doneChild.Status = domain.StatusClosed
	if err := mem.Save(context.Background(), doneChild); err != nil {
		t.Fatal(err)
	}
	openChild := newIssue("Pendiente")
	openChild.ParentID = &source.ID
	if err := mem.Save(context.Background(), openChild); err != nil {
		t.Fatal(err)
	}
	childrenPath := "/issues/" + source.ID.String() + "/children"
	expect("children hidden", call(http.MethodGet, childrenPath, "", outsider), http.StatusNotFound)
	children := data(call(http.MethodGet, childrenPath, "", contributor))
	progress, _ := children["progress"].(map[string]any)
	if progress["total"] != float64(2) || progress["done"] != float64(1) || progress["percent"] != float64(50) {
		t.Fatalf("children progress = %v", progress)
	}
	items, _ := children["children"].([]any)
	if len(items) != 2 {
		t.Fatalf("children items = %v", items)
	}
}
