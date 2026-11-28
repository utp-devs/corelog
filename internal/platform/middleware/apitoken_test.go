package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

func TestAPITokenAuthentication(t *testing.T) {
	user, tokenID := uuid.New(), uuid.New()
	valid := "cl_pat_" + strings.Repeat("a", 40)
	readOnly := func(method string) bool { return method == http.MethodGet }
	SetAPITokenAuthenticator(func(_ context.Context, plaintext string) (APITokenIdentity, error) {
		switch plaintext {
		case valid:
			return APITokenIdentity{UserID: user, Role: "user", TokenID: tokenID, AllowsMethod: readOnly}, nil
		case "cl_pat_limited":
			return APITokenIdentity{}, apperror.New(apperror.KindRateLimited, "slow down")
		}
		return APITokenIdentity{}, apperror.New(apperror.KindUnauthorized, "no")
	}, func(path string) bool { return strings.HasPrefix(path, "/auth") })
	t.Cleanup(func() { SetAPITokenAuthenticator(nil, nil) })

	h := Authenticate("secret-secret-secret-secret-secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, _ := UserIDFromContext(r.Context())
		role, _ := RoleFromContext(r.Context())
		tid, viaToken := APITokenIDFromContext(r.Context())
		if uid != user || role != "user" || !viaToken || tid != tokenID {
			t.Errorf("context: %v %q %v %v", uid, role, tid, viaToken)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	do := func(method, path, bearer string) int {
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("Authorization", "Bearer "+bearer)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	cases := []struct {
		name, method, path, bearer string
		want                       int
	}{
		{"valid read", http.MethodGet, "/issues", valid, http.StatusNoContent},
		{"write with read scope", http.MethodPost, "/issues", valid, http.StatusForbidden},
		{"session only path", http.MethodGet, "/auth/2fa", valid, http.StatusForbidden},
		{"unknown token", http.MethodGet, "/issues", "cl_pat_nope", http.StatusUnauthorized},
		{"rate limited", http.MethodGet, "/issues", "cl_pat_limited", http.StatusTooManyRequests},
	}
	for _, c := range cases {
		if got := do(c.method, c.path, c.bearer); got != c.want {
			t.Errorf("%s: status %d, want %d", c.name, got, c.want)
		}
	}
	if _, ok := APITokenIDFromContext(context.Background()); ok {
		t.Error("an empty context has no token")
	}
}

func TestAPITokenWithoutAuthenticatorIsRejected(t *testing.T) {
	SetAPITokenAuthenticator(nil, nil)
	h := Authenticate("secret-secret-secret-secret-secret")(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("handler must not run")
	}))
	req := httptest.NewRequest(http.MethodGet, "/issues", nil)
	req.Header.Set("Authorization", "Bearer cl_pat_"+strings.Repeat("a", 40))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
}
