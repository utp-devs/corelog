package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"

	"encoding/base64"
	"github.com/google/uuid"
	"github.com/jhonsferg/corelog/internal/legal/defaults"
	"github.com/jhonsferg/corelog/internal/platform/authtoken"
	"github.com/jhonsferg/corelog/internal/platform/config"
	"github.com/jhonsferg/corelog/internal/platform/crypto"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
)

const legalPassword = "contrasena-segura-1"

type legalEnv struct {
	t      *testing.T
	db     *sqlx.DB
	c      tfClient
	router http.Handler
}

func newLegalEnv(t *testing.T, enforcement string) *legalEnv {
	t.Helper()
	db := testdb.New(t)
	for _, d := range defaults.Documents() {
		if _, err := db.Exec(`INSERT INTO legal_documents (slug, version, title, body_markdown, summary, required, effective_from)
			VALUES ($1, 1, $2, $3, $4, $5, now() - interval '1 hour')`, string(d.Slug), d.Title, d.Body, d.Summary, d.Required); err != nil {
			t.Fatalf("seed document: %v", err)
		}
	}
	key := make([]byte, crypto.KeySize)
	cfg := &config.Config{
		Auth:  config.AuthConfig{JWTSecret: "legal-test-secret", JWTExpiration: time.Hour, TwoFactorEncryptionKey: base64.StdEncoding.EncodeToString(key)},
		Legal: config.LegalConfig{Enforcement: enforcement},
	}
	router, err := buildRouter(cfg, db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("buildRouter: %v", err)
	}
	return &legalEnv{t: t, db: db, c: tfClient{t: t, router: router}, router: router}
}

func consents(version int, slugs ...string) []map[string]any {
	out := []map[string]any{}
	for _, s := range slugs {
		out = append(out, map[string]any{"slug": s, "version": version})
	}
	return out
}

var requiredSlugs = []string{"privacy_policy", "data_processing", "terms_of_use"}

func (e *legalEnv) register(email string, cs []map[string]any) (int, map[string]any) {
	body := map[string]any{"name": "Ana Prueba", "email": email, "password": legalPassword, "password_confirmation": legalPassword}
	if cs != nil {
		body["consents"] = cs
	}
	return e.c.do(http.MethodPost, "/auth/register", "", body)
}

func (e *legalEnv) login(email string) (string, map[string]any) {
	e.t.Helper()
	status, body := e.c.do(http.MethodPost, "/auth/login", "", map[string]any{"email": email, "password": legalPassword})
	if status != http.StatusOK {
		e.t.Fatalf("login %s: %d %v", email, status, body)
	}
	data := tfData(e.t, body)
	return data["token"].(string), data
}

func (e *legalEnv) count(query string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.db.Get(&n, query, args...); err != nil {
		e.t.Fatal(err)
	}
	return n
}

func detailFields(body map[string]any) []string {
	errBody, _ := body["error"].(map[string]any)
	details, _ := errBody["details"].([]any)
	var out []string
	for _, d := range details {
		out = append(out, d.(map[string]any)["field"].(string))
	}
	return out
}

func TestLegalRegistrationConsentMatrix(t *testing.T) {
	e := newLegalEnv(t, "warn")

	status, body := e.register("none@example.test", nil)
	if status != http.StatusUnprocessableEntity || tfErrorCode(t, body) != "legal.consent_required" || len(detailFields(body)) != 3 {
		t.Fatalf("no consents: %d %v", status, body)
	}
	status, body = e.register("one@example.test", consents(1, "privacy_policy", "data_processing"))
	if status != http.StatusUnprocessableEntity || tfErrorCode(t, body) != "legal.consent_required" || len(detailFields(body)) != 1 || detailFields(body)[0] != "terms_of_use" {
		t.Fatalf("missing one: %d %v", status, body)
	}
	status, body = e.register("wrong@example.test", consents(7, requiredSlugs...))
	if status != http.StatusUnprocessableEntity || tfErrorCode(t, body) != "legal.consent_required" {
		t.Fatalf("wrong version: %d %v", status, body)
	}
	if n := e.count(`SELECT count(*) FROM users`); n != 0 {
		t.Fatalf("rejected registrations created %d users", n)
	}

	status, body = e.register("ok@example.test", consents(1, requiredSlugs...))
	if status != http.StatusCreated {
		t.Fatalf("all present: %d %v", status, body)
	}
	if n := e.count(`SELECT count(*) FROM user_consents WHERE source = 'register' AND ip = '192.0.2.0'`); n != 3 {
		t.Fatalf("consent rows with anonymized ip = %d, want 3", n)
	}

	status, body = e.register("extra@example.test", consents(1, append([]string{"communications"}, requiredSlugs...)...))
	if status != http.StatusCreated {
		t.Fatalf("optional extra: %d %v", status, body)
	}
	if n := e.count(`SELECT count(*) FROM user_consents WHERE document_slug = 'communications'`); n != 1 {
		t.Fatalf("communications rows = %d", n)
	}
}

func TestLegalRegistrationIsAtomic(t *testing.T) {
	e := newLegalEnv(t, "warn")
	// The required consents are fine but the optional one names a version that does not exist: recording fails
	// after the user insert, so neither the user nor any consent may remain.
	cs := append(consents(1, requiredSlugs...), map[string]any{"slug": "communications", "version": 9})
	status, body := e.register("atomic@example.test", cs)
	if status != http.StatusUnprocessableEntity || tfErrorCode(t, body) != "legal.version_invalid" {
		t.Fatalf("status %d %v", status, body)
	}
	if u, c := e.count(`SELECT count(*) FROM users`), e.count(`SELECT count(*) FROM user_consents`); u != 0 || c != 0 {
		t.Fatalf("users=%d consents=%d after a failed registration", u, c)
	}
}

func TestLegalPublicEndpointsAndCaching(t *testing.T) {
	e := newLegalEnv(t, "warn")
	get := func(path, inm string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/v1"+path, nil)
		if inm != "" {
			req.Header.Set("If-None-Match", inm)
		}
		rec := httptest.NewRecorder()
		e.router.ServeHTTP(rec, req)
		return rec
	}
	rec := get("/legal/documents", "")
	if rec.Code != http.StatusOK || rec.Header().Get("ETag") == "" || rec.Header().Get("Cache-Control") == "" {
		t.Fatalf("list: %d %v", rec.Code, rec.Header())
	}
	if again := get("/legal/documents", rec.Header().Get("ETag")); again.Code != http.StatusNotModified {
		t.Fatalf("conditional list: %d", again.Code)
	}
	rec = get("/legal/documents/privacy_policy", "")
	if rec.Code != http.StatusOK || rec.Header().Get("ETag") != `"privacy_policy-v1"` {
		t.Fatalf("detail: %d %v", rec.Code, rec.Header())
	}
	if again := get("/legal/documents/privacy_policy", `"privacy_policy-v1"`); again.Code != http.StatusNotModified {
		t.Fatalf("conditional detail: %d", again.Code)
	}
	if rec := get("/legal/documents/privacy_policy?version=1", ""); rec.Code != http.StatusOK {
		t.Fatalf("explicit version: %d", rec.Code)
	}
	if rec := get("/legal/documents/privacy_policy?version=9", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("missing version: %d", rec.Code)
	}
	if rec := get("/legal/documents/privacy_policy?version=abc", ""); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad version: %d", rec.Code)
	}
	if rec := get("/legal/documents/nope", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown slug: %d", rec.Code)
	}
	status, body := e.c.do(http.MethodGet, "/legal/documents", "", nil)
	items := body["data"].([]any)
	if status != http.StatusOK || len(items) != 5 {
		t.Fatalf("documents = %d (%d)", len(items), status)
	}
	if _, leaked := items[0].(map[string]any)["body_markdown"]; leaked {
		t.Fatal("the list must not carry the body")
	}
}

func TestLegalPendingAfterNewVersionAndAdminRules(t *testing.T) {
	e := newLegalEnv(t, "warn")
	// The first account becomes the platform admin.
	if status, body := e.register("admin@example.test", consents(1, requiredSlugs...)); status != http.StatusCreated {
		t.Fatalf("admin: %d %v", status, body)
	}
	if status, body := e.register("user@example.test", consents(1, requiredSlugs...)); status != http.StatusCreated {
		t.Fatalf("user: %d %v", status, body)
	}
	adminToken, data := e.login("admin@example.test")
	if p := data["pending_consents"].([]any); len(p) != 0 {
		t.Fatalf("fresh admin has pending %v", p)
	}
	userToken, _ := e.login("user@example.test")

	doc := map[string]any{"slug": "privacy_policy", "title": "Politica de privacidad v2", "summary": "cambios", "body_markdown": "# nuevo"}
	if status, _ := e.c.do(http.MethodPost, "/admin/legal/documents", "", doc); status != http.StatusUnauthorized {
		t.Fatalf("anonymous publish: %d", status)
	}
	if status, _ := e.c.do(http.MethodPost, "/admin/legal/documents", userToken, doc); status != http.StatusForbidden {
		t.Fatalf("member publish: %d", status)
	}
	if status, _ := e.c.do(http.MethodGet, "/admin/legal/documents", userToken, nil); status != http.StatusForbidden {
		t.Fatalf("member list: %d", status)
	}
	if status, _ := e.c.do(http.MethodGet, "/admin/legal/consents", userToken, nil); status != http.StatusForbidden {
		t.Fatalf("member audit: %d", status)
	}
	if status, body := e.c.do(http.MethodPost, "/admin/legal/documents", adminToken, map[string]any{"slug": "privacy_policy", "title": "", "body_markdown": ""}); status != http.StatusUnprocessableEntity {
		t.Fatalf("invalid publish: %d %v", status, body)
	}
	status, body := e.c.do(http.MethodPost, "/admin/legal/documents", adminToken, doc)
	if status != http.StatusCreated || tfData(t, body)["version"].(float64) != 2 {
		t.Fatalf("publish: %d %v", status, body)
	}
	if status, body := e.c.do(http.MethodGet, "/admin/legal/documents", adminToken, nil); status != http.StatusOK || len(body["data"].([]any)) != 6 {
		t.Fatalf("admin list: %d %v", status, body)
	}

	// Login reports the pending document without blocking.
	_, data = e.login("user@example.test")
	pending := data["pending_consents"].([]any)
	if len(pending) != 1 || pending[0].(map[string]any)["slug"] != "privacy_policy" || pending[0].(map[string]any)["version"].(float64) != 2 {
		t.Fatalf("pending = %v", pending)
	}
	status, body = e.c.do(http.MethodGet, "/users/me", userToken, nil)
	if status != http.StatusOK || len(tfData(t, body)["pending_consents"].([]any)) != 1 {
		t.Fatalf("/users/me: %d %v", status, body)
	}
	status, body = e.c.do(http.MethodGet, "/me/consents", userToken, nil)
	if status != http.StatusOK || len(tfData(t, body)["pending"].([]any)) != 1 || len(tfData(t, body)["accepted"].([]any)) != 3 {
		t.Fatalf("GET /me/consents: %d %v", status, body)
	}

	// Accept version 2 (idempotent when repeated); the old version is rejected.
	if status, _ := e.c.do(http.MethodPost, "/me/consents", userToken, map[string]any{"accepted": consents(1, "privacy_policy"), "source": "login_reprompt"}); status != http.StatusUnprocessableEntity {
		t.Fatalf("stale version: %d", status)
	}
	for i := 0; i < 2; i++ {
		status, body = e.c.do(http.MethodPost, "/me/consents", userToken, map[string]any{"accepted": consents(2, "privacy_policy"), "source": "login_reprompt"})
		if status != http.StatusOK || len(tfData(t, body)["pending"].([]any)) != 0 {
			t.Fatalf("accept v2: %d %v", status, body)
		}
	}
	if n := e.count(`SELECT count(*) FROM user_consents WHERE document_slug = 'privacy_policy' AND document_version = 2`); n != 1 {
		t.Fatalf("v2 rows = %d, want 1", n)
	}
	if n := e.count(`SELECT count(*) FROM domain_events WHERE type = 'legal.consent_recorded'`); n < 3 {
		t.Fatalf("consent events = %d", n)
	}

	// Audit by user.
	var userID string
	if err := e.db.Get(&userID, `SELECT id FROM users WHERE email = 'user@example.test'`); err != nil {
		t.Fatal(err)
	}
	status, body = e.c.do(http.MethodGet, "/admin/legal/consents?user="+userID, adminToken, nil)
	if status != http.StatusOK || len(body["data"].([]any)) != 4 {
		t.Fatalf("audit: %d %v", status, body)
	}
	if status, _ := e.c.do(http.MethodGet, "/admin/legal/consents?user=zzz", adminToken, nil); status != http.StatusUnprocessableEntity {
		t.Fatalf("audit with bad user: %d", status)
	}
}

func TestLegalRevocationRules(t *testing.T) {
	e := newLegalEnv(t, "warn")
	if status, body := e.register("rev@example.test", consents(1, requiredSlugs...)); status != http.StatusCreated {
		t.Fatalf("register: %d %v", status, body)
	}
	token, _ := e.login("rev@example.test")
	if status, _ := e.c.do(http.MethodGet, "/me/consents", "", nil); status != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", status)
	}
	status, body := e.c.do(http.MethodDelete, "/me/consents/privacy_policy", token, nil)
	if status != http.StatusConflict || tfErrorCode(t, body) != "legal.consent_not_revocable" {
		t.Fatalf("revoke required: %d %v", status, body)
	}
	status, body = e.c.do(http.MethodDelete, "/me/consents/communications", token, nil)
	if status != http.StatusNotFound || tfErrorCode(t, body) != "legal.consent_not_found" {
		t.Fatalf("revoke without consent: %d %v", status, body)
	}
	if status, _ := e.c.do(http.MethodPost, "/me/consents", token, map[string]any{"accepted": consents(1, "communications"), "source": "profile"}); status != http.StatusOK {
		t.Fatalf("accept communications: %d", status)
	}
	if status, _ := e.c.do(http.MethodDelete, "/me/consents/communications", token, nil); status != http.StatusNoContent {
		t.Fatalf("revoke communications: %d", status)
	}
	if n := e.count(`SELECT count(*) FROM user_consents WHERE document_slug = 'communications' AND revoked_at IS NOT NULL`); n != 1 {
		t.Fatalf("revoked rows = %d", n)
	}
	if status, _ := e.c.do(http.MethodPost, "/me/consents", token, map[string]any{"accepted": consents(1, "cookies"), "source": "weird"}); status != http.StatusUnprocessableEntity {
		t.Fatalf("bad source: %d", status)
	}
	if status, _ := e.c.do(http.MethodPost, "/me/consents", token, map[string]any{"accepted": consents(1, "cookies"), "source": "profile", "extra": true}); status != http.StatusUnprocessableEntity {
		t.Fatalf("unknown field: %d", status)
	}
}

func TestLegalBlockMode(t *testing.T) {
	e := newLegalEnv(t, "block")
	if status, body := e.register("blk@example.test", consents(1, requiredSlugs...)); status != http.StatusCreated {
		t.Fatalf("register: %d %v", status, body)
	}
	// A pre-existing account (created without consents) is pending.
	oldID := uuid.New()
	if _, err := e.db.Exec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, 'Old', 'Old', 'old@example.test', $2, 'user')`,
		oldID, "x"); err != nil {
		t.Fatal(err)
	}
	oldToken, err := authtoken.Generate("legal-test-secret", time.Hour, oldID, "user")
	if err != nil {
		t.Fatal(err)
	}

	status, body := e.c.do(http.MethodGet, "/users/me/sessions", oldToken, nil)
	if status != http.StatusForbidden || tfErrorCode(t, body) != "legal.consent_pending" || len(detailFields(body)) != 3 {
		t.Fatalf("blocked call: %d %v", status, body)
	}
	for _, path := range []string{"/users/me", "/me/consents", "/legal/documents"} {
		if status, _ := e.c.do(http.MethodGet, path, oldToken, nil); status != http.StatusOK {
			t.Errorf("%s must stay reachable while pending: %d", path, status)
		}
	}
	if status, _ := e.c.do(http.MethodPost, "/me/consents", oldToken, map[string]any{"accepted": consents(1, requiredSlugs...), "source": "login_reprompt"}); status != http.StatusOK {
		t.Fatalf("accept: %d", status)
	}
	if status, body := e.c.do(http.MethodGet, "/users/me/sessions", oldToken, nil); status == http.StatusForbidden {
		t.Fatalf("still blocked after accepting: %v", body)
	}
	// Anonymous and invalid tokens are not turned into 403.
	if status, _ := e.c.do(http.MethodGet, "/users/me/sessions", "", nil); status != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", status)
	}
}

func TestLegalWarnModeDoesNotBlock(t *testing.T) {
	e := newLegalEnv(t, "warn")
	oldID := uuid.New()
	if _, err := e.db.Exec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, 'Old', 'Old', 'old@example.test', 'x', 'user')`, oldID); err != nil {
		t.Fatal(err)
	}
	token, _ := authtoken.Generate("legal-test-secret", time.Hour, oldID, "user")
	if status, body := e.c.do(http.MethodGet, "/users/me/sessions", token, nil); status == http.StatusForbidden {
		t.Fatalf("warn mode blocked the call: %v", body)
	}
}

func TestLegalInvitationAcceptRequiresConsents(t *testing.T) {
	e := newLegalEnv(t, "warn")
	if status, body := e.register("owner@example.test", consents(1, requiredSlugs...)); status != http.StatusCreated {
		t.Fatalf("owner: %d %v", status, body)
	}
	if status, body := e.register("guest@example.test", consents(1, requiredSlugs...)); status != http.StatusCreated {
		t.Fatalf("guest: %d %v", status, body)
	}
	ownerToken, _ := e.login("owner@example.test")
	guestToken, _ := e.login("guest@example.test")

	status, body := e.c.do(http.MethodPost, "/organizations", ownerToken, map[string]any{"name": "Acme Legal"})
	if status != http.StatusCreated {
		t.Fatalf("create org: %d %v", status, body)
	}
	orgID := tfData(t, body)["id"].(string)
	status, body = e.c.do(http.MethodPost, "/organizations/"+orgID+"/invitations", ownerToken, map[string]any{
		"invitations": []map[string]any{{"email": "guest@example.test", "role": "member"}},
	})
	if status != http.StatusCreated && status != http.StatusOK {
		t.Fatalf("invite: %d %v", status, body)
	}
	token := tfData(t, body)["invitations"].([]any)[0].(map[string]any)["token"].(string)

	// A new privacy policy version is published before the invitation is accepted.
	if _, err := e.db.Exec(`INSERT INTO legal_documents (slug, version, title, body_markdown, required, effective_from) VALUES ('privacy_policy', 2, 'v2', 'x', true, now())`); err != nil {
		t.Fatal(err)
	}
	status, body = e.c.do(http.MethodPost, "/invitations/"+token+"/accept", guestToken, nil)
	if status != http.StatusUnprocessableEntity || tfErrorCode(t, body) != "legal.consent_required" || len(detailFields(body)) != 1 || detailFields(body)[0] != "privacy_policy" {
		t.Fatalf("accept without consent: %d %v", status, body)
	}
	if n := e.count(`SELECT count(*) FROM organization_members WHERE role = 'member'`); n != 0 {
		t.Fatalf("the invitation was accepted despite the missing consent")
	}
	status, body = e.c.do(http.MethodPost, "/invitations/"+token+"/accept", guestToken, map[string]any{"consents": consents(2, "privacy_policy")})
	if status != http.StatusCreated {
		t.Fatalf("accept with consent: %d %v", status, body)
	}
	if n := e.count(`SELECT count(*) FROM user_consents WHERE source = 'invite' AND document_version = 2`); n != 1 {
		t.Fatalf("invite consent rows = %d", n)
	}
}
