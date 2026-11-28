package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/authtoken"
	"github.com/jhonsferg/corelog/internal/platform/events"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
)

const integrationOrgID = "00000000-0000-4000-8000-000000000002"

// drainEvents runs the outbox consumers of the integrations until no event is left to deliver.
func drainEvents(t *testing.T, env *transitionEnv) {
	t.Helper()
	in := currentIntegrations.Load()
	if in == nil {
		t.Fatal("integrations are not wired")
	}
	dispatcher, err := events.NewDispatcher(env.db, []events.Consumer{in.webhooks.Service.Consumer(), in.automation.Engine.Consumer()},
		events.Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		if err := dispatcher.RunOnce(context.Background()); err != nil {
			t.Fatalf("dispatch: %v", err)
		}
	}
}

func (e *transitionEnv) userToken(name, orgRole string) string {
	e.t.Helper()
	id := uuid.New()
	e.exec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, $2, $2, $3, 'x', 'user')`, id, name, name+"@example.test")
	e.exec(`INSERT INTO organization_members (organization_id, user_id, role) VALUES ($1, $2, $3)`, integrationOrgID, id, orgRole)
	signed, err := authtoken.Generate(transitionTestSecret, time.Hour, id, "user")
	if err != nil {
		e.t.Fatal(err)
	}
	return signed
}

func TestAPITokensThroughTheRealRouter(t *testing.T) {
	env := newTransitionEnv(t, "scrum")
	issue := env.createIssue(env.contrib)

	create := func(token string, body map[string]any) (int, map[string]any) {
		return env.client.do(http.MethodPost, "/me/api-tokens", token, body)
	}
	if status, _ := create("", map[string]any{"name": "x", "scopes": []string{"read"}}); status != http.StatusUnauthorized {
		t.Fatalf("no auth: %d", status)
	}
	if status, body := create(env.contrib, map[string]any{"name": "", "scopes": []string{"read"}}); status != http.StatusUnprocessableEntity {
		t.Fatalf("empty name: %d %v", status, body)
	}
	if status, _ := create(env.contrib, map[string]any{"name": "x", "scopes": []string{"root"}}); status != http.StatusUnprocessableEntity {
		t.Fatalf("unknown scope: %d", status)
	}
	if status, body := create(env.contrib, map[string]any{"name": "x", "scopes": []string{"admin"}}); status != http.StatusForbidden || tfErrorCode(t, body) != "api_token.admin_scope_forbidden" {
		t.Fatalf("admin scope for a regular user: %d %v", status, body)
	}

	status, body := create(env.contrib, map[string]any{"name": "ci read", "scopes": []string{"read"}})
	if status != http.StatusCreated {
		t.Fatalf("create read token: %d %v", status, body)
	}
	read := tfData(t, body)
	plaintext := read["token"].(string)
	if !strings.HasPrefix(plaintext, "cl_pat_") || len(plaintext) != len("cl_pat_")+40 {
		t.Fatalf("token format: %q", plaintext)
	}
	status, body = create(env.contrib, map[string]any{"name": "ci write", "scopes": []string{"read", "write"}})
	if status != http.StatusCreated {
		t.Fatalf("create write token: %d %v", status, body)
	}
	writeToken := tfData(t, body)["token"].(string)

	// The plaintext is never shown again and only its hash is stored.
	status, body = env.client.do(http.MethodGet, "/me/api-tokens", env.contrib, nil)
	if status != http.StatusOK {
		t.Fatalf("list: %d", status)
	}
	raw, _ := json.Marshal(body)
	if strings.Contains(string(raw), plaintext) || strings.Contains(string(raw), "\"token\"") {
		t.Fatalf("list leaks the token: %s", raw)
	}
	var stored string
	if err := env.db.Get(&stored, `SELECT token_hash FROM api_tokens WHERE id = $1`, read["id"]); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(plaintext))
	if stored != hex.EncodeToString(sum[:]) || strings.Contains(stored, "cl_pat_") {
		t.Fatalf("stored hash = %q", stored)
	}
	// Another user does not see or revoke the token.
	if status, _ = env.client.do(http.MethodDelete, "/me/api-tokens/"+read["id"].(string), env.lead, nil); status != http.StatusNotFound {
		t.Fatalf("foreign revoke: %d", status)
	}

	// Authentication and scope enforcement.
	if status, body = env.client.do(http.MethodGet, "/issues/"+issue, plaintext, nil); status != http.StatusOK {
		t.Fatalf("read with read token: %d %v", status, body)
	}
	comment := map[string]any{"body": "hola"}
	if status, body = env.client.do(http.MethodPost, "/issues/"+issue+"/comments", plaintext, comment); status != http.StatusForbidden || tfErrorCode(t, body) != "api_token.scope_insufficient" {
		t.Fatalf("write with read token: %d %v", status, body)
	}
	if status, body = env.client.do(http.MethodPost, "/issues/"+issue+"/comments", writeToken, comment); status != http.StatusCreated {
		t.Fatalf("write with write token: %d %v", status, body)
	}
	// Prefix collision: a token that shares the display prefix of a real one does not authenticate.
	forged := plaintext[:11] + strings.Repeat("A", 40+7-11)
	if status, _ = env.client.do(http.MethodGet, "/issues/"+issue, forged, nil); status != http.StatusUnauthorized {
		t.Fatalf("forged token with the same prefix: %d", status)
	}
	if status, _ = env.client.do(http.MethodGet, "/issues/"+issue, "cl_pat_short", nil); status != http.StatusUnauthorized {
		t.Fatalf("malformed token: %d", status)
	}

	// A token never authorizes authentication endpoints or token management.
	for _, path := range []string{"/me/api-tokens", "/auth/2fa"} {
		if status, body = env.client.do(http.MethodGet, path, writeToken, nil); status != http.StatusForbidden || tfErrorCode(t, body) != "api_token.session_required" {
			t.Fatalf("token on %s: %d %v", path, status, body)
		}
	}
	if status, _ = create(writeToken, map[string]any{"name": "child", "scopes": []string{"read"}}); status != http.StatusForbidden {
		t.Fatalf("token creating token: %d", status)
	}

	// last_used_at is recorded; revocation is immediate.
	var used *time.Time
	if err := env.db.Get(&used, `SELECT last_used_at FROM api_tokens WHERE id = $1`, read["id"]); err != nil || used == nil {
		t.Fatalf("last_used_at = %v, %v", used, err)
	}
	if status, _ = env.client.do(http.MethodDelete, "/me/api-tokens/"+read["id"].(string), env.contrib, nil); status != http.StatusNoContent {
		t.Fatalf("revoke: %d", status)
	}
	if status, _ = env.client.do(http.MethodGet, "/issues/"+issue, plaintext, nil); status != http.StatusUnauthorized {
		t.Fatalf("revoked token: %d", status)
	}

	// Expired tokens are rejected.
	env.exec(`UPDATE api_tokens SET expires_at = now() - interval '1 minute' WHERE name = 'ci write'`)
	if status, _ = env.client.do(http.MethodGet, "/issues/"+issue, writeToken, nil); status != http.StatusUnauthorized {
		t.Fatalf("expired token: %d", status)
	}

	// At most 20 active tokens.
	for i := 0; i < 20; i++ {
		if status, body = create(env.viewer, map[string]any{"name": "t" + strconv.Itoa(i), "scopes": []string{"read"}}); status != http.StatusCreated {
			t.Fatalf("token %d: %d %v", i, status, body)
		}
	}
	if status, body = create(env.viewer, map[string]any{"name": "one too many", "scopes": []string{"read"}}); status != http.StatusConflict || tfErrorCode(t, body) != "api_token.limit_reached" {
		t.Fatalf("21st token: %d %v", status, body)
	}
	var audits int
	if err := env.db.Get(&audits, `SELECT count(*) FROM domain_events WHERE type IN ('auth.token_created', 'auth.token_revoked')`); err != nil || audits < 3 {
		t.Fatalf("audit events: %d %v", audits, err)
	}
	appmw.SetAPITokenAuthenticator(nil, nil)
}

func TestWebhooksThroughTheRealRouter(t *testing.T) {
	t.Setenv("WEBHOOK_ALLOW_PRIVATE", "true")
	env := newTransitionEnv(t, "scrum")
	orgAdmin := env.userToken("orgadmin", "admin")
	base := "/organizations/" + integrationOrgID + "/webhooks"

	var mu sync.Mutex
	var received []struct {
		body    []byte
		headers http.Header
	}
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		received = append(received, struct {
			body    []byte
			headers http.Header
		}{raw, r.Header.Clone()})
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	input := map[string]any{"name": "CI", "url": target.URL, "events": []string{"issue.created"}}
	// Only organization admins manage webhooks.
	for name, token := range map[string]string{"lead": env.lead, "contributor": env.contrib, "viewer": env.viewer} {
		if status, _ := env.client.do(http.MethodPost, base, token, input); status != http.StatusForbidden {
			t.Fatalf("%s create: %d", name, status)
		}
	}
	if status, _ := env.client.do(http.MethodGet, base, env.outside, nil); status != http.StatusNotFound {
		t.Fatalf("outsider list: %d", status)
	}
	if status, _ := env.client.do(http.MethodPost, base, "", input); status != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", status)
	}
	if status, body := env.client.do(http.MethodPost, base, orgAdmin, map[string]any{"name": "x", "url": "ftp://x", "events": []string{"issue.created"}}); status != http.StatusUnprocessableEntity {
		t.Fatalf("bad scheme: %d %v", status, body)
	}
	if status, _ := env.client.do(http.MethodPost, base, orgAdmin, map[string]any{"name": "x", "url": target.URL, "events": []string{"nope"}}); status != http.StatusUnprocessableEntity {
		t.Fatalf("bad event: %d", status)
	}

	status, body := env.client.do(http.MethodPost, base, orgAdmin, input)
	if status != http.StatusCreated {
		t.Fatalf("create: %d %v", status, body)
	}
	hook := tfData(t, body)
	secret, _ := hook["secret"].(string)
	if !strings.HasPrefix(secret, "whsec_") {
		t.Fatalf("secret: %q", secret)
	}
	id := hook["id"].(string)
	var sealed []byte
	if err := env.db.Get(&sealed, `SELECT secret_encrypted FROM webhooks WHERE id = $1`, id); err != nil || bytes.Contains(sealed, []byte(secret)) {
		t.Fatalf("secret must be stored encrypted: %v", err)
	}
	if status, body = env.client.do(http.MethodGet, base+"/"+id, orgAdmin, nil); status != http.StatusOK || tfData(t, body)["secret"] != nil {
		t.Fatalf("get must not return the secret: %d %v", status, body)
	}

	// Ping.
	if status, body = env.client.do(http.MethodPost, base+"/"+id+"/test", orgAdmin, nil); status != http.StatusOK || tfData(t, body)["status"] != "success" {
		t.Fatalf("test ping: %d %v", status, body)
	}

	// A real event is captured, delivered and signed.
	issueID := env.createIssue(env.contrib)
	drainEvents(t, env)
	in := currentIntegrations.Load()
	if n, err := in.webhooks.Service.RunDue(context.Background()); err != nil || n != 1 {
		t.Fatalf("RunDue = %d, %v", n, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(received) != 2 {
		t.Fatalf("received %d requests, want ping and issue.created", len(received))
	}
	got := received[1]
	if got.headers.Get("X-CoreLog-Event") != "issue.created" || got.headers.Get("X-CoreLog-Delivery") == "" {
		t.Fatalf("headers: %v", got.headers)
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(got.headers.Get("X-CoreLog-Timestamp") + "."))
	mac.Write(got.body)
	if want := "sha256=" + hex.EncodeToString(mac.Sum(nil)); got.headers.Get("X-CoreLog-Signature") != want {
		t.Fatalf("signature = %q, want %q", got.headers.Get("X-CoreLog-Signature"), want)
	}
	var payload map[string]any
	if err := json.Unmarshal(got.body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["version"] != "v1" || payload["event"] != "issue.created" || payload["organization_id"] != integrationOrgID {
		t.Fatalf("payload: %v", payload)
	}
	if issue, _ := payload["issue"].(map[string]any); issue == nil || issue["id"] != issueID {
		t.Fatalf("payload issue: %v", payload["issue"])
	}

	// Redelivery and secret rotation.
	status, body = env.client.do(http.MethodGet, base+"/"+id+"/deliveries", orgAdmin, nil)
	list, _ := body["data"].([]any)
	if status != http.StatusOK || len(list) != 2 {
		t.Fatalf("deliveries: %d %v", status, body)
	}
	did := list[0].(map[string]any)["id"].(string)
	if status, _ = env.client.do(http.MethodPost, base+"/"+id+"/deliveries/"+did+"/redeliver", orgAdmin, nil); status != http.StatusAccepted {
		t.Fatalf("redeliver: %d", status)
	}
	if status, _ = env.client.do(http.MethodPost, base+"/"+id+"/deliveries/"+uuid.NewString()+"/redeliver", orgAdmin, nil); status != http.StatusNotFound {
		t.Fatalf("redeliver unknown: %d", status)
	}
	status, body = env.client.do(http.MethodPost, base+"/"+id+"/rotate-secret", orgAdmin, nil)
	if status != http.StatusOK || tfData(t, body)["secret"] == secret || tfData(t, body)["secret"] == "" {
		t.Fatalf("rotate: %d %v", status, body)
	}
	if status, _ = env.client.do(http.MethodPatch, base+"/"+id, orgAdmin, map[string]any{"enabled": false}); status != http.StatusOK {
		t.Fatalf("disable: %d", status)
	}
	if status, _ = env.client.do(http.MethodDelete, base+"/"+id, orgAdmin, nil); status != http.StatusNoContent {
		t.Fatalf("delete: %d", status)
	}
	if status, _ = env.client.do(http.MethodGet, base+"/"+id, orgAdmin, nil); status != http.StatusNotFound {
		t.Fatalf("get deleted: %d", status)
	}
}

func TestAutomationThroughTheRealRouter(t *testing.T) {
	env := newTransitionEnv(t, "scrum")
	project := env.project.String()
	path := "/projects/" + project + "/automation-rules"
	rule := map[string]any{
		"name":    "Etiquetar al crear",
		"trigger": map[string]any{"type": "issue_created"},
		"actions": []any{
			map[string]any{"type": "add_label", "label": "auto"},
			map[string]any{"type": "add_comment", "text": "Hola {{actor.name}}, creaste {{issue.key}}: {{issue.summary}}"},
		},
	}

	// Permission matrix: manage_automation is held by the lead only.
	for name, token := range map[string]string{"contributor": env.contrib, "viewer": env.viewer} {
		if status, _ := env.client.do(http.MethodPost, path, token, rule); status != http.StatusForbidden {
			t.Fatalf("%s create: %d", name, status)
		}
		if status, _ := env.client.do(http.MethodGet, path, token, nil); status != http.StatusForbidden {
			t.Fatalf("%s list: %d", name, status)
		}
	}
	if status, _ := env.client.do(http.MethodGet, path, env.outside, nil); status != http.StatusForbidden && status != http.StatusNotFound {
		t.Fatalf("outsider list: %d", status)
	}
	if status, _ := env.client.do(http.MethodGet, path, "", nil); status != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", status)
	}
	bad := map[string]any{"name": "x", "trigger": map[string]any{"type": "issue_created"}, "actions": []any{map[string]any{"type": "add_comment", "text": "{{secret.token}}"}}}
	if status, body := env.client.do(http.MethodPost, path, env.lead, bad); status != http.StatusUnprocessableEntity || tfErrorCode(t, body) != "automation.invalid_rule" {
		t.Fatalf("unknown smart value: %d %v", status, body)
	}

	status, body := env.client.do(http.MethodPost, path, env.lead, rule)
	if status != http.StatusCreated {
		t.Fatalf("create: %d %v", status, body)
	}
	created := tfData(t, body)
	ruleID := created["id"].(string)
	if created["version"].(float64) != 1 || created["enabled"] != true {
		t.Fatalf("rule: %v", created)
	}

	// Dry run.
	status, body = env.client.do(http.MethodPost, "/automation-rules/validate", env.lead, map[string]any{
		"project_id": project, "name": "x", "trigger": rule["trigger"], "actions": rule["actions"],
	})
	if status != http.StatusOK || tfData(t, body)["valid"] != true || len(tfData(t, body)["planned_actions"].([]any)) != 2 {
		t.Fatalf("validate: %d %v", status, body)
	}
	status, body = env.client.do(http.MethodPost, "/automation-rules/validate", env.lead, map[string]any{
		"project_id": project, "trigger": map[string]any{"type": "nope"}, "actions": rule["actions"],
	})
	if status != http.StatusOK || tfData(t, body)["valid"] != false {
		t.Fatalf("validate invalid: %d %v", status, body)
	}

	// The rule runs for a new issue as the automation identity.
	issueID := env.createIssue(env.contrib)
	drainEvents(t, env)
	var labels []string
	if err := env.db.Select(&labels, `SELECT l.name FROM issue_labels il JOIN labels l ON l.id = il.label_id WHERE il.issue_id = $1`, issueID); err != nil || len(labels) != 1 || labels[0] != "auto" {
		var details []string
		_ = env.db.Select(&details, `SELECT status || ": " || detail FROM automation_runs`)
		t.Fatalf("labels = %v, %v; runs = %v", labels, err, details)
	}
	var comment string
	if err := env.db.Get(&comment, `SELECT body FROM comments WHERE issue_id = $1`, issueID); err != nil || !strings.Contains(comment, "Etiquetar al crear") || !strings.Contains(comment, "Incidencia de prueba") {
		t.Fatalf("comment = %q, %v", comment, err)
	}
	status, body = env.client.do(http.MethodGet, "/automation-rules/"+ruleID+"/runs", env.lead, nil)
	runs, _ := body["data"].([]any)
	if status != http.StatusOK || len(runs) == 0 || runs[0].(map[string]any)["status"] != "success" {
		t.Fatalf("runs: %d %v", status, body)
	}

	// Loop prevention: a rule on every update that edits the issue is not re-triggered by its own change.
	loop := map[string]any{
		"name":    "Bucle",
		"trigger": map[string]any{"type": "issue_updated"},
		"actions": []any{map[string]any{"type": "add_label", "label": "tocada"}, map[string]any{"type": "set_field", "field": "summary", "value": "{{issue.summary}} !"}},
	}
	status, body = env.client.do(http.MethodPost, path, env.lead, loop)
	if status != http.StatusCreated {
		t.Fatalf("create loop rule: %d %v", status, body)
	}
	loopID := tfData(t, body)["id"].(string)
	if status, _ = env.client.do(http.MethodPatch, "/issues/"+issueID, env.contrib, map[string]any{"summary": "Cambiada"}); status != http.StatusOK {
		t.Fatalf("patch issue: %d", status)
	}
	drainEvents(t, env)
	var executed, skipped int
	_ = env.db.Get(&executed, `SELECT count(*) FROM automation_runs WHERE rule_id = $1 AND status <> 'skipped'`, loopID)
	_ = env.db.Get(&skipped, `SELECT count(*) FROM automation_runs WHERE rule_id = $1 AND status = 'skipped'`, loopID)
	if executed != 1 || skipped < 1 {
		t.Fatalf("loop rule executed %d times and skipped %d times", executed, skipped)
	}

	// Versioned update, enable and disable, delete.
	if status, body = env.client.do(http.MethodPatch, "/automation-rules/"+ruleID, env.lead, map[string]any{"name": "Renombrada", "version": 1}); status != http.StatusOK || tfData(t, body)["version"].(float64) != 2 {
		t.Fatalf("patch: %d %v", status, body)
	}
	if status, body = env.client.do(http.MethodPatch, "/automation-rules/"+ruleID, env.lead, map[string]any{"name": "Vieja", "version": 1}); status != http.StatusConflict || tfErrorCode(t, body) != "automation.version_conflict" {
		t.Fatalf("stale patch: %d %v", status, body)
	}
	if status, body = env.client.do(http.MethodPost, "/automation-rules/"+ruleID+"/disable", env.lead, nil); status != http.StatusOK || tfData(t, body)["enabled"] != false {
		t.Fatalf("disable: %d %v", status, body)
	}
	if status, _ = env.client.do(http.MethodPost, "/automation-rules/"+ruleID+"/enable", env.contrib, nil); status != http.StatusForbidden {
		t.Fatalf("contributor enable: %d", status)
	}
	if status, _ = env.client.do(http.MethodGet, "/automation-rules/"+uuid.NewString(), env.lead, nil); status != http.StatusNotFound {
		t.Fatalf("unknown rule: %d", status)
	}
	if status, _ = env.client.do(http.MethodDelete, "/automation-rules/"+ruleID, env.lead, nil); status != http.StatusNoContent {
		t.Fatalf("delete: %d", status)
	}
}

// multipartCSV posts a CSV file to the import endpoint.
func multipartCSV(t *testing.T, env *transitionEnv, path, token, name, content string) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", name)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte(content))
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/v1"+path, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	env.client.router.ServeHTTP(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func rawGet(env *transitionEnv, path, token string) (int, string, http.Header) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1"+path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	env.client.router.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String(), rec.Header()
}

func TestCSVImportAndExportThroughTheRealRouter(t *testing.T) {
	env := newTransitionEnv(t, "scrum")
	project := env.project.String()
	upload := "/projects/" + project + "/import/csv"
	csvText := "\xEF\xBB\xBFSummary;Description;Issue Type;Priority;Assignee;Labels;Due date;Story Points\n" +
		"=cmd|' /C calc'!A0;Formula como texto;Task;High;lead@example.test;a,b;2026-12-01;3\n" +
		"Segunda fila;\"Con\nsalto de linea\";Bug;Low;;;;\n" +
		"Sin tipo valido;x;Inexistente;;;;;\n" +
		";sin resumen;Task;;;;;\n" +
		"Fecha mala;x;Task;;;;31-31-2026;\n"

	if status, _ := multipartCSV(t, env, upload, env.contrib, "a.csv", csvText); status != http.StatusForbidden {
		t.Fatalf("contributor upload: %d", status)
	}
	if status, _ := multipartCSV(t, env, upload, env.outside, "a.csv", csvText); status != http.StatusForbidden && status != http.StatusNotFound {
		t.Fatalf("outsider upload: %d", status)
	}
	if status, _ := multipartCSV(t, env, upload, "", "a.csv", csvText); status != http.StatusUnauthorized {
		t.Fatalf("anonymous upload: %d", status)
	}
	if status, body := multipartCSV(t, env, upload, env.lead, "a.csv", "\xff\xfe"); status != http.StatusUnprocessableEntity || tfErrorCode(t, body) != "import.invalid_encoding" {
		t.Fatalf("invalid encoding: %d %v", status, body)
	}
	if status, body := multipartCSV(t, env, upload, env.lead, "a.csv", "Summary\n"); status != http.StatusUnprocessableEntity || tfErrorCode(t, body) != "import.empty_file" {
		t.Fatalf("empty file: %d %v", status, body)
	}

	status, body := multipartCSV(t, env, upload, env.lead, "jira.csv", csvText)
	if status != http.StatusCreated {
		t.Fatalf("upload: %d %v", status, body)
	}
	preview := tfData(t, body)
	jobID := preview["job_id"].(string)
	suggested := preview["suggested_mapping"].(map[string]any)
	if suggested["summary"] != "Summary" || suggested["story_points"] != "Story Points" || suggested["due_date"] != "Due date" || preview["delimiter"] != ";" {
		t.Fatalf("preview: %v", preview)
	}
	if len(preview["sample_rows"].([]any)) != 5 || preview["total_rows"].(float64) != 5 {
		t.Fatalf("sample rows: %v", preview)
	}

	mapping := map[string]any{}
	for k, v := range suggested {
		mapping[k] = v
	}
	start := "/projects/" + project + "/import/csv/" + jobID + "/start"
	if status, _ = env.client.do(http.MethodPost, start, env.contrib, map[string]any{"mapping": mapping}); status != http.StatusForbidden {
		t.Fatalf("contributor start: %d", status)
	}
	if status, body = env.client.do(http.MethodPost, start, env.lead, map[string]any{"mapping": map[string]any{"description": "Description"}}); status != http.StatusUnprocessableEntity || tfErrorCode(t, body) != "import.invalid_mapping" {
		t.Fatalf("mapping without summary: %d %v", status, body)
	}
	// Dry run first: nothing is created.
	if status, body = env.client.do(http.MethodPost, start, env.lead, map[string]any{"mapping": mapping, "dry_run": true}); status != http.StatusAccepted {
		t.Fatalf("dry run start: %d %v", status, body)
	}
	if status, _ = env.client.do(http.MethodPost, start, env.lead, map[string]any{"mapping": mapping}); status != http.StatusConflict {
		t.Fatalf("second start: %d", status)
	}
	in := currentIntegrations.Load()
	if err := in.importExport.Service.RunQueued(context.Background()); err != nil {
		t.Fatal(err)
	}
	var issues int
	_ = env.db.Get(&issues, `SELECT count(*) FROM issues WHERE project_id = $1`, env.project)
	if issues != 0 {
		t.Fatalf("dry run created %d issues", issues)
	}
	status, body = env.client.do(http.MethodGet, "/projects/"+project+"/import/jobs/"+jobID, env.lead, nil)
	job := tfData(t, body)
	if status != http.StatusOK || job["status"] != "done" || job["dry_run"] != true || job["error_rows"].(float64) != 3 {
		t.Fatalf("dry run job: %d %v", status, job)
	}

	// Real run.
	status, body = multipartCSV(t, env, upload, env.lead, "jira.csv", csvText)
	jobID = tfData(t, body)["job_id"].(string)
	start = "/projects/" + project + "/import/csv/" + jobID + "/start"
	if status, body = env.client.do(http.MethodPost, start, env.lead, map[string]any{"mapping": mapping}); status != http.StatusAccepted {
		t.Fatalf("start: %d %v", status, body)
	}
	if err := in.importExport.Service.RunQueued(context.Background()); err != nil {
		t.Fatal(err)
	}
	status, body = env.client.do(http.MethodGet, "/projects/"+project+"/import/jobs/"+jobID, env.lead, nil)
	job = tfData(t, body)
	if status != http.StatusOK || job["status"] != "done" || job["created_rows"].(float64) != 2 || job["error_rows"].(float64) != 3 || job["processed_rows"].(float64) != 5 {
		t.Fatalf("job: %d %v", status, job)
	}
	var summaries []string
	if err := env.db.Select(&summaries, `SELECT summary FROM issues WHERE project_id = $1 ORDER BY key_number`, env.project); err != nil || len(summaries) != 2 || !strings.HasPrefix(summaries[0], "=cmd") {
		t.Fatalf("summaries = %v, %v", summaries, err)
	}
	code, report, headers := rawGet(env, "/projects/"+project+"/import/jobs/"+jobID+"/errors.csv", env.lead)
	if code != http.StatusOK || !strings.HasPrefix(headers.Get("Content-Type"), "text/csv") || !strings.Contains(report, "Inexistente") || !strings.Contains(report, "fila") {
		t.Fatalf("error report: %d %q", code, report)
	}
	if code, _, _ = rawGet(env, "/projects/"+project+"/import/jobs", env.contrib); code != http.StatusForbidden {
		t.Fatalf("contributor job list: %d", code)
	}

	// Export: the formula in the summary is neutralized, the viewer may export, the outsider may not.
	code, exported, _ := rawGet(env, "/projects/"+project+"/export/issues.csv", env.viewer)
	if code != http.StatusOK || !strings.Contains(exported, "'=cmd") || strings.Contains(exported, ",=cmd") || strings.Contains(exported, "\n=cmd") {
		t.Fatalf("export: %d %q", code, exported)
	}
	if code, _, _ = rawGet(env, "/projects/"+project+"/export/issues.csv", env.outside); code != http.StatusForbidden && code != http.StatusNotFound {
		t.Fatalf("outsider export: %d", code)
	}
	code, filtered, _ := rawGet(env, "/projects/"+project+"/export/issues.csv?filter="+"summary%20~%20%22Segunda%22", env.lead)
	if code != http.StatusOK || !strings.Contains(filtered, "Segunda fila") || strings.Contains(filtered, "cmd") {
		t.Fatalf("filtered export: %d %q", code, filtered)
	}
	code, dump, _ := rawGet(env, "/projects/"+project+"/export/project.json", env.lead)
	var parsed map[string]any
	if code != http.StatusOK || json.Unmarshal([]byte(dump), &parsed) != nil || len(parsed["issues"].([]any)) != 2 {
		t.Fatalf("project json: %d %.200s", code, dump)
	}
	if code, _, _ = rawGet(env, "/projects/"+project+"/export/project.json", env.contrib); code != http.StatusForbidden {
		t.Fatalf("contributor backup: %d", code)
	}
}
