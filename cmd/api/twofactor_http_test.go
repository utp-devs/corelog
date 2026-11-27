package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jhonsferg/corelog/internal/platform/config"
	"github.com/jhonsferg/corelog/internal/platform/crypto"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
	"github.com/jhonsferg/corelog/internal/user/domain"
)

const (
	tfPassword = "contrasena-segura-1"
	tfEmail    = "ana.dos@example.com"
)

// tfClient drives the real router with JSON requests and decodes the {data}/{error} envelope.
type tfClient struct {
	t      *testing.T
	router http.Handler
}

func (c tfClient) do(method, path, bearer string, body any) (int, map[string]any) {
	c.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			c.t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, "/api/v1"+path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	c.router.ServeHTTP(rec, req)

	var out map[string]any
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			c.t.Fatalf("%s %s: decode %q: %v", method, path, rec.Body.String(), err)
		}
	}
	return rec.Code, out
}

func tfData(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("response has no data object: %v", body)
	}
	return data
}

func tfErrorCode(t *testing.T, body map[string]any) string {
	t.Helper()
	errBody, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("response has no error object: %v", body)
	}
	code, _ := errBody["code"].(string)
	return code
}

func TestTwoFactorLoginFlowThroughRouter(t *testing.T) {
	db := testdb.New(t)
	key := make([]byte, crypto.KeySize)
	for i := range key {
		key[i] = byte(i + 1)
	}
	cfg := &config.Config{Auth: config.AuthConfig{
		JWTSecret:              "two-factor-test-secret",
		JWTExpiration:          time.Hour,
		TwoFactorEncryptionKey: base64.StdEncoding.EncodeToString(key),
	}}
	router, err := buildRouter(cfg, db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("buildRouter: %v", err)
	}
	c := tfClient{t: t, router: router}

	if status, _ := c.do(http.MethodPost, "/auth/register", "", map[string]any{
		"name": "Ana Dos", "email": tfEmail, "password": tfPassword,
	}); status != http.StatusCreated {
		t.Fatalf("register: status %d", status)
	}

	// Without the second factor the login is unchanged.
	status, body := c.do(http.MethodPost, "/auth/login", "", map[string]any{"email": tfEmail, "password": tfPassword})
	if status != http.StatusOK {
		t.Fatalf("login: status %d", status)
	}
	token, _ := tfData(t, body)["token"].(string)
	if token == "" {
		t.Fatal("login without two-factor must return a token")
	}

	status, body = c.do(http.MethodGet, "/auth/2fa", token, nil)
	if status != http.StatusOK || tfData(t, body)["enabled"] != false {
		t.Fatalf("status before enrollment: %d %v", status, body)
	}
	status, body = c.do(http.MethodPost, "/auth/2fa/confirm", token, map[string]any{"code": "123456"})
	if status != http.StatusConflict || tfErrorCode(t, body) != "auth.two_factor_not_enrolled" {
		t.Fatalf("confirm without enrollment: %d %v", status, body)
	}

	status, body = c.do(http.MethodPost, "/auth/2fa/enroll", token, nil)
	if status != http.StatusOK {
		t.Fatalf("enroll: status %d %v", status, body)
	}
	enroll := tfData(t, body)
	secretText, _ := enroll["secret"].(string)
	uri, _ := enroll["otpauth_uri"].(string)
	if !strings.HasPrefix(uri, "otpauth://totp/") || len(secretText) != 32 {
		t.Fatalf("enroll response: %v", enroll)
	}
	secret, err := domain.ParseTOTPSecret(secretText)
	if err != nil {
		t.Fatal(err)
	}

	if status, body = c.do(http.MethodPost, "/auth/2fa/confirm", token, map[string]any{"code": "000000"}); status != http.StatusUnprocessableEntity {
		t.Fatalf("confirm with a wrong code: %d %v", status, body)
	}
	code := domain.TOTPAt(secret, domain.StepOf(time.Now()))
	status, body = c.do(http.MethodPost, "/auth/2fa/confirm", token, map[string]any{"code": code})
	if status != http.StatusOK {
		t.Fatalf("confirm: status %d %v", status, body)
	}
	rawCodes, _ := tfData(t, body)["recovery_codes"].([]any)
	if len(rawCodes) != domain.RecoveryCodeCount {
		t.Fatalf("got %d recovery codes", len(rawCodes))
	}
	recovery, _ := rawCodes[0].(string)
	recoveryNext, _ := rawCodes[1].(string)

	// Login now answers with a challenge and no session.
	status, body = c.do(http.MethodPost, "/auth/login", "", map[string]any{"email": tfEmail, "password": tfPassword})
	if status != http.StatusOK {
		t.Fatalf("login with two-factor: status %d", status)
	}
	challenge := tfData(t, body)
	if challenge["two_factor_required"] != true || challenge["token"] != nil {
		t.Fatalf("expected a two-factor challenge without a session token: %v", challenge)
	}
	challengeToken, _ := challenge["challenge_token"].(string)

	// The challenge token is not an access token.
	if status, _ := c.do(http.MethodGet, "/users/me", challengeToken, nil); status != http.StatusUnauthorized {
		t.Fatalf("challenge token used as access token: status %d", status)
	}
	// The access token is not a challenge token.
	if status, body := c.do(http.MethodPost, "/auth/login/2fa", "", map[string]any{
		"challenge_token": token, "recovery_code": recoveryNext,
	}); status != http.StatusUnauthorized || tfErrorCode(t, body) != "auth.challenge_expired" {
		t.Fatalf("access token as challenge: %d %v", status, body)
	}

	// A wrong recovery code is rejected without opening a session.
	if status, body := c.do(http.MethodPost, "/auth/login/2fa", "", map[string]any{
		"challenge_token": challengeToken, "recovery_code": "ZZZZZZZZZZ",
	}); status != http.StatusUnprocessableEntity || tfErrorCode(t, body) != "auth.two_factor_invalid" {
		t.Fatalf("wrong recovery code: %d %v", status, body)
	}

	status, body = c.do(http.MethodPost, "/auth/login/2fa", "", map[string]any{
		"challenge_token": challengeToken, "recovery_code": recovery,
	})
	if status != http.StatusOK {
		t.Fatalf("login with recovery code: status %d %v", status, body)
	}
	sessionToken, _ := tfData(t, body)["token"].(string)
	if status, _ := c.do(http.MethodGet, "/users/me", sessionToken, nil); status != http.StatusOK {
		t.Fatalf("session from the challenge cannot read the profile: %d", status)
	}

	// The same recovery code cannot be used again, even with a new challenge.
	status, body = c.do(http.MethodPost, "/auth/login", "", map[string]any{"email": tfEmail, "password": tfPassword})
	if status != http.StatusOK {
		t.Fatalf("second login: %d", status)
	}
	again, _ := tfData(t, body)["challenge_token"].(string)
	if status, body := c.do(http.MethodPost, "/auth/login/2fa", "", map[string]any{
		"challenge_token": again, "recovery_code": recovery,
	}); status != http.StatusUnprocessableEntity || tfErrorCode(t, body) != "auth.two_factor_invalid" {
		t.Fatalf("reused recovery code: %d %v", status, body)
	}

	status, body = c.do(http.MethodGet, "/auth/2fa", sessionToken, nil)
	if status != http.StatusOK || tfData(t, body)["recovery_codes_remaining"] != float64(domain.RecoveryCodeCount-1) {
		t.Fatalf("status after one recovery code: %d %v", status, body)
	}

	// Disable needs the password; a wrong password is refused.
	if status, _ := c.do(http.MethodPost, "/auth/2fa/disable", sessionToken, map[string]any{
		"password": "otra-contrasena", "recovery_code": recoveryNext,
	}); status != http.StatusUnauthorized {
		t.Fatalf("disable with wrong password: status %d", status)
	}
	if status, _ := c.do(http.MethodPost, "/auth/2fa/disable", sessionToken, map[string]any{
		"password": tfPassword, "recovery_code": recoveryNext,
	}); status != http.StatusNoContent {
		t.Fatalf("disable: status %d", status)
	}

	// Back to the plain login.
	status, body = c.do(http.MethodPost, "/auth/login", "", map[string]any{"email": tfEmail, "password": tfPassword})
	if status != http.StatusOK || tfData(t, body)["token"] == nil {
		t.Fatalf("login after disable must return a token: %d %v", status, body)
	}
}
