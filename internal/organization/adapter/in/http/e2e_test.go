package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	orghttp "github.com/jhonsferg/corelog/internal/organization/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/organization/adapter/out/postgres"
	"github.com/jhonsferg/corelog/internal/organization/application/usecase"
	"github.com/jhonsferg/corelog/internal/platform/authtoken"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
	"github.com/jhonsferg/corelog/internal/platform/events"
)

const e2eSecret = "e2e-secret-with-enough-length-0123456789ab"

// sqlEmails resolves emails from the users table, like the adapter wired in cmd/api.
type sqlEmails struct{ db *sqlx.DB }

func (e sqlEmails) EmailOf(ctx context.Context, userID uuid.UUID) (string, error) {
	var email string
	if err := e.db.GetContext(ctx, &email, `SELECT email FROM users WHERE id = $1`, userID); err != nil {
		return "", err
	}
	return email, nil
}

type actor struct {
	id    uuid.UUID
	email string
	token string
}

type harness struct {
	t      *testing.T
	router http.Handler
}

func newHarness(t *testing.T, db *sqlx.DB) *harness {
	t.Helper()
	repo := postgres.NewRepository(db)
	pub := events.NewPostgresPublisher(func(ctx context.Context) sqlx.ExtContext { return database.Executor(ctx, db) })
	svc := usecase.NewOrganizationService(repo, database.NewTxManager(db), pub, clock.RealClock{})
	h := orghttp.NewHandler(svc, sqlEmails{db: db}, clock.RealClock{}, e2eSecret, "http://app.test")
	r := chi.NewRouter()
	r.Route("/api/v1", h.RegisterRoutes)
	return &harness{t: t, router: r}
}

func newActor(t *testing.T, db *sqlx.DB, label string) actor {
	t.Helper()
	id := uuid.New()
	email := fmt.Sprintf("%s-%s@example.test", label, id.String()[:8])
	_, err := db.ExecContext(context.Background(), `
INSERT INTO users (id, name, display_name, email, password_hash, role)
VALUES ($1, $2, $2, $3, 'not-a-real-hash', 'user')`, id, label, email)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	tok, err := authtoken.Generate(e2eSecret, time.Hour, id, "user")
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	return actor{id: id, email: email, token: tok}
}

func (h *harness) do(method, path string, a *actor, body string) *httptest.ResponseRecorder {
	h.t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	if a != nil {
		req.Header.Set("Authorization", "Bearer "+a.token)
	}
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}

func (h *harness) mustStatus(rec *httptest.ResponseRecorder, want int) {
	h.t.Helper()
	if rec.Code != want {
		h.t.Fatalf("status %d, want %d: %s", rec.Code, want, rec.Body.String())
	}
}

func errorCodeOf(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return body.Error.Code
}

func dataOf(t *testing.T, rec *httptest.ResponseRecorder, into any) {
	t.Helper()
	var body struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	if err := json.Unmarshal(body.Data, into); err != nil {
		t.Fatalf("decode data %s: %v", body.Data, err)
	}
}

func TestOrganizationsEndToEnd(t *testing.T) {
	db := testdb.New(t)
	h := newHarness(t, db)

	alice := newActor(t, db, "alice")
	bob := newActor(t, db, "bob")
	carol := newActor(t, db, "carol")

	// A creates the organization.
	rec := h.do(http.MethodPost, "/api/v1/organizations", &alice, `{"name":"Acme"}`)
	h.mustStatus(rec, http.StatusCreated)
	var org struct {
		ID   string `json:"id"`
		Slug string `json:"slug"`
		Role string `json:"role"`
	}
	dataOf(t, rec, &org)
	if org.Role != "owner" || org.Slug != "acme" {
		t.Fatalf("org = %+v", org)
	}
	orgPath := "/api/v1/organizations/" + org.ID

	// A invites B.
	body := fmt.Sprintf(`{"invitations":[{"email":%q,"role":"member"}]}`, bob.email)
	rec = h.do(http.MethodPost, orgPath+"/invitations", &alice, body)
	h.mustStatus(rec, http.StatusCreated)
	var created struct {
		Invitations []struct {
			ID        string `json:"id"`
			Token     string `json:"token"`
			AcceptURL string `json:"accept_url"`
		} `json:"invitations"`
	}
	dataOf(t, rec, &created)
	if len(created.Invitations) != 1 || created.Invitations[0].Token == "" {
		t.Fatalf("invitation response = %s", rec.Body.String())
	}
	inv := created.Invitations[0]
	if !strings.HasPrefix(inv.AcceptURL, "http://app.test/invite/") {
		t.Fatalf("accept_url = %q", inv.AcceptURL)
	}

	// B previews without a token, then accepts.
	rec = h.do(http.MethodGet, "/api/v1/invitations/"+inv.Token, nil, "")
	h.mustStatus(rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"status":"pending"`) {
		t.Fatalf("preview = %s", rec.Body.String())
	}
	rec = h.do(http.MethodPost, "/api/v1/invitations/"+inv.Token+"/accept", &bob, "")
	h.mustStatus(rec, http.StatusCreated)

	// Token reuse after accept.
	rec = h.do(http.MethodPost, "/api/v1/invitations/"+inv.Token+"/accept", &bob, "")
	h.mustStatus(rec, http.StatusConflict)
	if code := errorCodeOf(t, rec); code != "org.invitation_used" {
		t.Fatalf("reuse code = %q", code)
	}

	// B lists members and sees A and B.
	rec = h.do(http.MethodGet, orgPath+"/members", &bob, "")
	h.mustStatus(rec, http.StatusOK)
	var members []struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"`
	}
	dataOf(t, rec, &members)
	if len(members) != 2 {
		t.Fatalf("members = %+v", members)
	}

	// B, a plain member, cannot change roles.
	rec = h.do(http.MethodPatch, orgPath+"/members/"+alice.id.String(), &bob, `{"role":"guest"}`)
	h.mustStatus(rec, http.StatusForbidden)
	if code := errorCodeOf(t, rec); code != "org.forbidden" {
		t.Fatalf("role change code = %q", code)
	}

	// The last owner cannot leave.
	rec = h.do(http.MethodPost, orgPath+"/leave", &alice, "")
	h.mustStatus(rec, http.StatusConflict)
	if code := errorCodeOf(t, rec); code != "org.last_owner" {
		t.Fatalf("leave code = %q", code)
	}

	// Isolation: an outsider gets 404 on every org-scoped route, with a valid body so that only
	// membership decides the answer.
	resendTarget := uuid.New().String()
	cases := []struct {
		name, method, path, body string
	}{
		{"get org", http.MethodGet, orgPath, ""},
		{"patch org", http.MethodPatch, orgPath, `{"name":"Hack"}`},
		{"delete org", http.MethodDelete, orgPath, `{"confirm_slug":"acme"}`},
		{"leave", http.MethodPost, orgPath + "/leave", ""},
		{"transfer", http.MethodPost, orgPath + "/transfer-ownership", fmt.Sprintf(`{"user_id":%q}`, bob.id)},
		{"list members", http.MethodGet, orgPath + "/members", ""},
		{"change role", http.MethodPatch, orgPath + "/members/" + bob.id.String(), `{"role":"admin"}`},
		{"remove member", http.MethodDelete, orgPath + "/members/" + bob.id.String(), ""},
		{"list invitations", http.MethodGet, orgPath + "/invitations", ""},
		{"create invitations", http.MethodPost, orgPath + "/invitations", `{"invitations":[{"email":"x@example.test","role":"member"}]}`},
		{"resend invitation", http.MethodPost, orgPath + "/invitations/" + resendTarget + "/resend", ""},
		{"revoke invitation", http.MethodDelete, orgPath + "/invitations/" + resendTarget, ""},
	}
	for _, c := range cases {
		rec := h.do(c.method, c.path, &carol, c.body)
		if rec.Code != http.StatusNotFound {
			t.Errorf("isolation %s: status %d, want 404 (%s)", c.name, rec.Code, rec.Body.String())
		}
	}

	// Enumeration safety: an unknown token and an invisible organization have the same error shape.
	unknown := h.do(http.MethodGet, "/api/v1/invitations/"+strings.Repeat("z", 43), nil, "")
	h.mustStatus(unknown, http.StatusNotFound)
	invisible := h.do(http.MethodGet, "/api/v1/organizations/"+uuid.NewString(), &carol, "")
	h.mustStatus(invisible, http.StatusNotFound)
	if shape(t, unknown) != shape(t, invisible) {
		t.Errorf("error shapes differ: %s vs %s", unknown.Body.String(), invisible.Body.String())
	}
	if code := errorCodeOf(t, unknown); code != "org.invitation_not_found" {
		t.Errorf("unknown token code = %q", code)
	}

	// Limit: alice already owns one organization; four more are allowed, the sixth is refused.
	for i := 0; i < 4; i++ {
		rec := h.do(http.MethodPost, "/api/v1/organizations", &alice, fmt.Sprintf(`{"name":"Extra %d"}`, i))
		h.mustStatus(rec, http.StatusCreated)
	}
	rec = h.do(http.MethodPost, "/api/v1/organizations", &alice, `{"name":"Sexta"}`)
	h.mustStatus(rec, http.StatusConflict)
	if code := errorCodeOf(t, rec); code != "org.limit_reached" {
		t.Fatalf("limit code = %q", code)
	}
}

// shape returns the JSON structure of an error body with its values removed.
func shape(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var generic map[string]map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &generic); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	keys := make([]string, 0, len(generic["error"]))
	for k := range generic["error"] {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return fmt.Sprint(len(generic), keys)
}

func TestInvitationRequiresMatchingEmailAndAuthentication(t *testing.T) {
	db := testdb.New(t)
	h := newHarness(t, db)
	alice := newActor(t, db, "alice2")
	mallory := newActor(t, db, "mallory")
	invitee := newActor(t, db, "invitee")

	rec := h.do(http.MethodPost, "/api/v1/organizations", &alice, `{"name":"Beta"}`)
	h.mustStatus(rec, http.StatusCreated)
	var org struct {
		ID string `json:"id"`
	}
	dataOf(t, rec, &org)
	orgPath := "/api/v1/organizations/" + org.ID

	rec = h.do(http.MethodPost, orgPath+"/invitations", &alice,
		fmt.Sprintf(`{"invitations":[{"email":%q,"role":"member"}]}`, invitee.email))
	h.mustStatus(rec, http.StatusCreated)
	var created struct {
		Invitations []struct {
			Token string `json:"token"`
		} `json:"invitations"`
	}
	dataOf(t, rec, &created)
	token := created.Invitations[0].Token

	rec = h.do(http.MethodPost, "/api/v1/invitations/"+token+"/accept", nil, "")
	h.mustStatus(rec, http.StatusUnauthorized)

	rec = h.do(http.MethodPost, "/api/v1/invitations/"+token+"/accept", &mallory, "")
	h.mustStatus(rec, http.StatusForbidden)
	if code := errorCodeOf(t, rec); code != "org.email_mismatch" {
		t.Fatalf("mismatch code = %q", code)
	}

	rec = h.do(http.MethodPost, "/api/v1/invitations/"+token+"/accept", &invitee, "")
	h.mustStatus(rec, http.StatusCreated)
}
