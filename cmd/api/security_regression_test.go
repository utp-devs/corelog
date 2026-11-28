package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
)

// syncBuffer is a log sink that can be written from the request goroutines and read by the test.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// securityHeaders are set on every response of the API, whatever its status.
var securityHeaders = []string{
	"X-Content-Type-Options", "X-Frame-Options", "Referrer-Policy", "Permissions-Policy",
	"Content-Security-Policy", "Cross-Origin-Resource-Policy", "Cache-Control",
}

// TestSecurityHeadersOnEveryRoute checks the defensive headers on the answers of every route (anonymous 401s and
// authorized answers alike) and on the answers of the router itself (404 and 405).
func TestSecurityHeadersOnEveryRoute(t *testing.T) {
	registry, err := parseAuthzRegistry()
	if err != nil {
		t.Fatal(err)
	}
	f := newAuthzFixture(t)
	check := func(label string, rec *httptest.ResponseRecorder) {
		for _, h := range securityHeaders {
			if rec.Header().Get(h) == "" && !(h == "Cache-Control" && isHealthLabel(label)) {
				t.Errorf("%s: %d without %s", label, rec.Code, h)
			}
		}
		if rec.Header().Get("X-Request-ID") == "" {
			t.Errorf("%s: %d without X-Request-ID", label, rec.Code)
		}
	}
	do := func(method, target, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, target, nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		f.router.ServeHTTP(rec, req)
		return rec
	}
	for _, route := range sortedRoutes(registry) {
		target := "/api/v1" + resolve(route.Path, f.ids, "").target
		if route.Path == "/health" {
			target = "/health"
		}
		check(route.key()+" (anonymous)", do(route.Method, target, ""))
		check(route.key()+" (lead)", do(route.Method, target, f.tokens[actLead]))
	}
	check("unknown path", do(http.MethodGet, "/api/v1/no-such-route", f.tokens[actLead]))
	check("wrong method", do("BREW", "/api/v1/issues", f.tokens[actLead]))
}

// TestOversizedBodiesAreRefusedOnEveryMutatingRoute sends a JSON body above the limit to every route that writes and
// expects 413 before any handler runs, for anonymous and authenticated callers.
func TestOversizedBodiesAreRefusedOnEveryMutatingRoute(t *testing.T) {
	registry, err := parseAuthzRegistry()
	if err != nil {
		t.Fatal(err)
	}
	f := newAuthzFixture(t)
	huge := `{"x":"` + strings.Repeat("a", (1<<20)+64) + `"}`
	for _, route := range sortedRoutes(registry) {
		if route.Method == http.MethodGet {
			continue
		}
		target := resolve(route.Path, f.ids, "").target
		for _, token := range []string{"", f.tokens[actLead]} {
			if status, _ := f.call(route.Method, target, token, jsonRaw(huge)); status != http.StatusRequestEntityTooLarge {
				t.Errorf("%s with a %d byte body (token=%v): %d, want 413", route.key(), len(huge), token != "", status)
			}
		}
	}
}

// routesWithoutBody never read the request body (they act on the path and the caller), so there is nothing to be
// strict about. A route that starts reading a body must leave this list.
var routesWithoutBody = map[string]bool{
	"POST /api/v1/auth/2fa/enroll": true, "POST /api/v1/auth/email/verify/request": true,
	"POST /api/v1/automation-rules/{}/enable": true, "POST /api/v1/automation-rules/{}/disable": true,
	"POST /api/v1/issues/{}/watchers": true, "POST /api/v1/me/calendar/feed": true,
	"POST /api/v1/notes/{}/pin": true, "POST /api/v1/notes/{}/unpin": true, "POST /api/v1/notifications/read-all": true,
	"POST /api/v1/organizations/{}/webhooks/{}/rotate-secret": true, "POST /api/v1/organizations/{}/webhooks/{}/test": true,
	"POST /api/v1/projects/{}/permissions/reset": true, "POST /api/v1/users/me/sessions/revoke-others": true,
	"PUT /api/v1/comments/{}/reactions/{}": true, "PUT /api/v1/issues/{}/vote": true, "PUT /api/v1/sprints/{}/issues/{}": true,
}

// TestUnknownFieldsAreRejected adds an unknown member to the valid body of every route that accepts a JSON body
// and succeeds: a handler with strict decoding must refuse the extra member instead of ignoring it.
func TestUnknownFieldsAreRejected(t *testing.T) {
	registry, err := parseAuthzRegistry()
	if err != nil {
		t.Fatal(err)
	}
	f := newAuthzFixture(t)
	var lenient []string
	checked := 0
	for _, route := range sortedRoutes(registry) {
		if route.Method == http.MethodGet || route.Method == http.MethodDelete || route.Scope == scopePublic || route.NoPositive || routesWithoutBody[route.key()] {
			continue
		}
		actor := actLead
		switch route.Scope {
		case scopePlat:
			actor = actPlatform
		case scopeOrgW, scopeOrgO, scopeOrgC:
			actor = actOrgOwner
		}
		raw := routeBodies[route.key()]
		if raw == "" {
			raw = "{}"
		}
		var body map[string]any
		if err := json.Unmarshal([]byte(fill(raw, f.ids, f.users[actLead].String())), &body); err != nil {
			t.Fatalf("%s: body template: %v", route.key(), err)
		}
		target := resolve(route.Path, f.ids, "").target
		status, resp := f.call(route.Method, target, f.tokens[actor], body)
		if status < 200 || status > 299 {
			continue // the valid body is not enough to succeed here (missing fields, state): nothing to compare
		}
		checked++
		body["zz_unknown_field"] = 1
		again := resolve(route.Path, f.ids, "").target
		status2, resp2 := f.call(route.Method, again, f.tokens[actor], body)
		if status2 >= 200 && status2 < 300 {
			lenient = append(lenient, fmt.Sprintf("%s accepted an unknown field (%d %s) after %d %s", route.key(), status2, truncate(resp2, 80), status, truncate(resp, 60)))
		}
	}
	t.Logf("%d routes accept their valid body; %d of them ignore unknown fields", checked, len(lenient))
	sort.Strings(lenient)
	if len(lenient) > 0 {
		t.Errorf("handlers without strict decoding:\n  %s", strings.Join(lenient, "\n  "))
	}
}

// TestLogsNeverContainSecrets drives the authentication, recovery, two factor, token and webhook flows through the
// real router with every logger captured and checks that no password, token, code, secret, full phone or full email
// reaches the logs.
func TestLogsNeverContainSecrets(t *testing.T) {
	sink := &syncBuffer{}
	logger := slog.New(slog.NewJSONHandler(sink, &slog.HandlerOptions{Level: slog.LevelDebug}))
	previous := slog.Default()
	slog.SetDefault(logger)
	log.SetOutput(sink)
	t.Cleanup(func() {
		slog.SetDefault(previous)
		log.SetOutput(io.Discard)
	})

	f := newAuthzFixture(t, func(o *fixtureOptions) { o.logger = logger; o.env = "staging" })
	const (
		email    = "scrub.person@authz.test"
		password = "Sup3r-Secret-Pass!"
		newPass  = "N3w-Secret-Pass!!"
		phone    = "3001234567"
	)
	secrets := []string{password, newPass, phone, email, "reset-token-AAAA-secret", "verify-token-BBBB-secret", "invite-token-CCCC-secret",
		"feed-token-DDDD-secret", "123456", "654321", "cl_bogus_token_EEEE_secret", "recovery-code-FFFF"}

	call := func(method, target, token string, body any) (int, []byte) {
		t.Helper()
		return f.call(method, target, token, body)
	}
	field := func(raw []byte, keys ...string) string {
		var doc map[string]any
		_ = json.Unmarshal(raw, &doc)
		var cur any = doc
		for _, k := range keys {
			m, _ := cur.(map[string]any)
			cur = m[k]
		}
		s, _ := cur.(string)
		return s
	}

	status, raw := call(http.MethodPost, "/auth/register", "", map[string]any{
		"first_name": "Scrub", "last_name": "Person", "email": email, "password": password, "password_confirmation": password,
		"phone": map[string]any{"country": "CO", "number": phone},
	})
	if status != http.StatusCreated && status != http.StatusOK {
		t.Fatalf("register: %d %s", status, truncate(raw, 200))
	}
	status, raw = call(http.MethodPost, "/auth/login", "", map[string]any{"email": email, "password": password})
	if status != http.StatusOK {
		t.Fatalf("login: %d %s", status, truncate(raw, 200))
	}
	access := field(raw, "data", "token")
	if access == "" {
		access = field(raw, "data", "access_token")
	}
	if access == "" {
		t.Fatalf("login response has no token: %s", truncate(raw, 200))
	}
	secrets = append(secrets, access)

	call(http.MethodPost, "/auth/login", "", map[string]any{"email": email, "password": "wrong-" + password})
	call(http.MethodPost, "/auth/login/2fa", "", map[string]any{"challenge_token": "challenge-GGGG-secret", "code": "123456"})
	secrets = append(secrets, "challenge-GGGG-secret", "wrong-"+password)
	call(http.MethodPost, "/auth/password/forgot", "", map[string]any{"email": email})
	call(http.MethodPost, "/auth/password/forgot", "", map[string]any{"email": "nobody." + email})
	call(http.MethodPost, "/auth/password/reset", "", map[string]any{"token": "reset-token-AAAA-secret", "password": newPass, "password_confirmation": newPass})
	call(http.MethodPost, "/auth/email/verify", "", map[string]any{"token": "verify-token-BBBB-secret"})
	call(http.MethodPost, "/auth/email/verify/request", access, map[string]any{})
	call(http.MethodPost, "/users/me/password", access, map[string]any{"current_password": "wrong-" + password, "new_password": newPass, "new_password_confirmation": newPass})

	status, raw = call(http.MethodPost, "/auth/2fa/enroll", access, map[string]any{})
	if status == http.StatusOK || status == http.StatusCreated {
		secrets = append(secrets, field(raw, "data", "secret"), field(raw, "data", "otpauth_url"))
	}
	call(http.MethodPost, "/auth/2fa/confirm", access, map[string]any{"code": "123456"})
	call(http.MethodPost, "/auth/2fa/disable", access, map[string]any{"password": "wrong-" + password, "code": "123456"})
	call(http.MethodPost, "/auth/2fa/recovery-codes", access, map[string]any{"password": password})

	call(http.MethodPost, "/users/me/phone/verification", access, map[string]any{})
	call(http.MethodPost, "/users/me/phone/verify", access, map[string]any{"code": "654321"})

	call(http.MethodGet, "/invitations/invite-token-CCCC-secret", "", nil)
	call(http.MethodPost, "/invitations/invite-token-CCCC-secret/accept", access, map[string]any{})
	call(http.MethodGet, "/calendar/export.ics?token=feed-token-DDDD-secret", "", nil)
	call(http.MethodGet, "/users/me", "cl_bogus_token_EEEE_secret", nil)

	// Tokens and secrets that the API hands out once.
	status, raw = call(http.MethodPost, "/me/api-tokens", f.tokens[actLead], map[string]any{"name": "scrub token", "scopes": []string{"read"}})
	if status == http.StatusCreated {
		plaintext := field(raw, "data", "token")
		secrets = append(secrets, plaintext)
		call(http.MethodGet, "/users/me", plaintext, nil)
	}
	status, raw = call(http.MethodPost, "/organizations/"+f.orgA+"/webhooks/"+f.ids["webhook"]+"/rotate-secret", f.tokens[actOrgAdmin], map[string]any{})
	if status == http.StatusOK || status == http.StatusCreated {
		secrets = append(secrets, field(raw, "data", "secret"))
	}
	call(http.MethodPost, "/organizations/"+f.orgA+"/webhooks", f.tokens[actOrgAdmin], map[string]any{
		"name": "scrub hook", "url": "https://example.com/scrub", "events": []string{"issue.created"},
	})

	logged := sink.String()
	if len(logged) == 0 {
		t.Fatal("nothing was logged: the capture is not wired")
	}
	for _, secret := range secrets {
		if len(secret) < 6 {
			continue
		}
		if strings.Contains(logged, secret) {
			idx := strings.Index(logged, secret)
			start := max(0, idx-120)
			t.Errorf("the logs contain a secret %q:\n...%s...", secret, logged[start:min(len(logged), idx+len(secret)+60)])
		}
	}
	t.Logf("%d bytes of logs inspected against %d secrets", len(logged), len(secrets))
}

// isHealthLabel reports whether a check label belongs to the health probe, which is cacheable by design.
func isHealthLabel(label string) bool { return strings.HasPrefix(label, "GET /health") }
