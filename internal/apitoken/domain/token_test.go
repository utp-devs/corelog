package domain

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

func TestGenerateFormatAndHash(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		tok, err := Generate()
		if err != nil {
			t.Fatal(err)
		}
		if !WellFormed(tok) || !strings.HasPrefix(tok, "cl_pat_") || len(tok) != 47 {
			t.Fatalf("bad token %q", tok)
		}
		if seen[tok] {
			t.Fatal("duplicate token")
		}
		seen[tok] = true
		if h := Hash(tok); len(h) != 64 || strings.Contains(h, "cl_pat_") {
			t.Fatalf("hash %q", h)
		}
		if got := DisplayPrefix(tok); got != tok[:11] {
			t.Fatalf("prefix %q", got)
		}
	}
	if Hash("a") == Hash("b") || Hash("a") != Hash("a") {
		t.Fatal("hash must be deterministic and collision free for distinct inputs")
	}
	if DisplayPrefix("cl") != "cl" {
		t.Fatal("short prefix")
	}
}

func TestWellFormed(t *testing.T) {
	good := "cl_pat_" + strings.Repeat("a", 40)
	cases := map[string]bool{
		good:                                      true,
		"cl_pat_" + strings.Repeat("a", 39):       false,
		"cl_pat_" + strings.Repeat("a", 41):       false,
		"xx_pat_" + strings.Repeat("a", 40):       false,
		"cl_pat_" + strings.Repeat("-", 40):       false,
		"cl_pat_" + strings.Repeat("a", 39) + " ": false,
		"": false,
	}
	for in, want := range cases {
		if got := WellFormed(in); got != want {
			t.Errorf("WellFormed(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestParseScopes(t *testing.T) {
	got, err := ParseScopes([]string{"write", "read", "read"})
	if err != nil || len(got) != 2 || got[0] != ScopeRead || got[1] != ScopeWrite {
		t.Fatalf("scopes = %v, %v", got, err)
	}
	for _, bad := range [][]string{nil, {}, {"root"}, {"read", ""}} {
		if _, err := ParseScopes(bad); apperror.KindOf(err) != apperror.KindValidation {
			t.Errorf("ParseScopes(%v) = %v", bad, err)
		}
	}
}

func TestValidateNameAndExpiry(t *testing.T) {
	if n, err := ValidateName("  CI  "); err != nil || n != "CI" {
		t.Fatalf("name = %q, %v", n, err)
	}
	for _, bad := range []string{"", "   ", strings.Repeat("x", 81)} {
		if _, err := ValidateName(bad); err == nil {
			t.Errorf("name %q accepted", bad)
		}
	}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	future, past, far := now.Add(time.Hour), now.Add(-time.Hour), now.Add(MaxLifetime+time.Hour)
	if ValidateExpiry(nil, now) != nil || ValidateExpiry(&future, now) != nil {
		t.Fatal("valid expiry rejected")
	}
	if ValidateExpiry(&past, now) == nil || ValidateExpiry(&far, now) == nil {
		t.Fatal("invalid expiry accepted")
	}
}

func TestTokenActive(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	before, after := now.Add(-time.Minute), now.Add(time.Minute)
	cases := []struct {
		name string
		tok  Token
		want bool
	}{
		{"no expiry", Token{}, true},
		{"not yet expired", Token{ExpiresAt: &after}, true},
		{"expired", Token{ExpiresAt: &before}, false},
		{"expires exactly now", Token{ExpiresAt: &now}, false},
		{"revoked", Token{RevokedAt: &before}, false},
	}
	for _, c := range cases {
		if got := c.tok.Active(now); got != c.want {
			t.Errorf("%s: Active = %v", c.name, got)
		}
	}
}

func TestAllowsMethod(t *testing.T) {
	read, write, admin := []Scope{ScopeRead}, []Scope{ScopeRead, ScopeWrite}, []Scope{ScopeAdmin}
	cases := []struct {
		scopes []Scope
		method string
		want   bool
	}{
		{read, http.MethodGet, true}, {read, http.MethodHead, true}, {read, http.MethodOptions, true},
		{read, http.MethodPost, false}, {read, http.MethodPatch, false}, {read, http.MethodDelete, false}, {read, http.MethodPut, false},
		{write, http.MethodGet, true}, {write, http.MethodPost, true}, {write, http.MethodDelete, true},
		{admin, http.MethodPost, true}, {nil, http.MethodGet, false}, {nil, http.MethodPost, false},
	}
	for _, c := range cases {
		if got := AllowsMethod(c.scopes, c.method); got != c.want {
			t.Errorf("AllowsMethod(%v, %s) = %v", c.scopes, c.method, got)
		}
	}
}

func TestSessionOnlyPath(t *testing.T) {
	for path, want := range map[string]bool{
		"/api/v1/auth/login": true, "/api/v1/auth/2fa/enroll": true, "/api/v1/me/api-tokens": true,
		"/api/v1/me/api-tokens/abc": true, "/api/v1/me/password": true, "/api/v1/me/sessions/x": true,
		"/api/v1/me/deactivate": true, "/api/v1/me/phone/verify": true, "/api/v1/me/api-tokens/": true,
		"/api/v1/me": false, "/api/v1/me/timesheet": false, "/api/v1/issues": false, "/api/v1/authors": false,
		"/me/api-tokens": true,
	} {
		if got := SessionOnlyPath(path); got != want {
			t.Errorf("SessionOnlyPath(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestScopeHas(t *testing.T) {
	tok := Token{Scopes: []Scope{ScopeRead}}
	if !tok.Has(ScopeRead) || tok.Has(ScopeWrite) {
		t.Fatal("Has")
	}
}
