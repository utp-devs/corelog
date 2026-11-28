package http_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

// errorEnvelope mirrors the {"error":{...}} body written by httpresponse.Error.
type errorEnvelope struct {
	Error struct {
		Message string `json:"message"`
		Code    string `json:"code"`
		Details []struct {
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"details"`
	} `json:"error"`
}

func decodeErrorEnvelope(t *testing.T, body []byte) errorEnvelope {
	t.Helper()
	var env errorEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("decode error body: %v; body: %s", err, body)
	}
	return env
}

func TestUserHTTP_ErrorCodes(t *testing.T) {
	router, _, _ := setupUserRouter()
	const email, password = "codigos@example.com", "Tr1cky-Pass9x"
	register := `{"name":"Codigos","email":"` + email + `","password":"` + password + `","password_confirmation":"` + password + `"}`

	if rec := doJSON(t, router, http.MethodPost, "/auth/register", register, ""); rec.Code != http.StatusCreated {
		t.Fatalf("register status %d: %s", rec.Code, rec.Body.String())
	}

	t.Run("duplicate email returns auth.email_taken", func(t *testing.T) {
		rec := doJSON(t, router, http.MethodPost, "/auth/register", register, "")
		if rec.Code != http.StatusConflict {
			t.Fatalf("status %d, want 409", rec.Code)
		}
		if env := decodeErrorEnvelope(t, rec.Body.Bytes()); env.Error.Code != "auth.email_taken" {
			t.Fatalf("code %q, want auth.email_taken", env.Error.Code)
		}
	})

	t.Run("bad credentials return auth.invalid_credentials", func(t *testing.T) {
		rec := doJSON(t, router, http.MethodPost, "/auth/login", `{"email":"`+email+`","password":"wrong-password"}`, "")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status %d, want 401", rec.Code)
		}
		if env := decodeErrorEnvelope(t, rec.Body.Bytes()); env.Error.Code != "auth.invalid_credentials" {
			t.Fatalf("code %q, want auth.invalid_credentials", env.Error.Code)
		}
	})

	t.Run("invalid body returns validation.failed with field details", func(t *testing.T) {
		rec := doJSON(t, router, http.MethodPost, "/auth/register", `{"name":"","email":"no-es-correo","password":"x","password_confirmation":"x"}`, "")
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status %d, want 422", rec.Code)
		}
		env := decodeErrorEnvelope(t, rec.Body.Bytes())
		if env.Error.Code != "validation.failed" {
			t.Fatalf("code %q, want validation.failed", env.Error.Code)
		}
		fields := map[string]string{}
		for _, d := range env.Error.Details {
			fields[d.Field] = d.Message
		}
		for _, want := range []string{"email", "password"} {
			if fields[want] == "" {
				t.Errorf("missing detail for field %q; details: %+v", want, env.Error.Details)
			}
		}
	})
}
