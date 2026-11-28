package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/authtoken"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	userhttp "github.com/jhonsferg/corelog/internal/user/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/user/application/usecase"
	"github.com/jhonsferg/corelog/internal/user/domain"
)

type memoryPhoneRepo struct {
	items []*domain.PhoneVerification
}

func (r *memoryPhoneRepo) CreateWithinLimit(_ context.Context, v *domain.PhoneVerification, since time.Time, max int) error {
	n := 0
	for _, it := range r.items {
		if it.UserID == v.UserID && it.CreatedAt.After(since) {
			n++
		}
	}
	if n >= max {
		return domain.ErrPhoneSendLimit
	}
	v.ID = uuid.New()
	copied := *v
	r.items = append(r.items, &copied)
	return nil
}

func (r *memoryPhoneRepo) FindLatestOpen(_ context.Context, userID uuid.UUID, e164 string) (*domain.PhoneVerification, error) {
	for i := len(r.items) - 1; i >= 0; i-- {
		it := r.items[i]
		if it.UserID == userID && it.E164 == e164 && it.ConsumedAt == nil {
			copied := *it
			return &copied, nil
		}
	}
	return nil, domain.ErrPhoneCodeExpired
}

func (r *memoryPhoneRepo) RegisterAttempt(_ context.Context, id uuid.UUID) (int, error) {
	for _, it := range r.items {
		if it.ID == id {
			it.Attempts++
			return it.Attempts, nil
		}
	}
	return 0, domain.ErrPhoneCodeExpired
}

func (r *memoryPhoneRepo) Consume(_ context.Context, userID, _ uuid.UUID, at time.Time) error {
	for _, it := range r.items {
		if it.UserID == userID && it.ConsumedAt == nil {
			t := at
			it.ConsumedAt = &t
		}
	}
	return nil
}

type captureSMS struct{ to, code string }

func (c *captureSMS) SendVerificationCode(_ context.Context, e164, code string) error {
	c.to, c.code = e164, code
	return nil
}

type identityEnv struct {
	router *chi.Mux
	secret string
	repo   *memoryUserRepo
	clock  *clock.FakeClock
	sms    *captureSMS
}

func newIdentityEnv() *identityEnv {
	secret := "test-secret-key-that-is-long-enough-32-bytes"
	repo := newMemoryUserRepo()
	fc := &clock.FakeClock{T: time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)}
	sms := &captureSMS{}
	svc := usecase.NewUserService(repo, newMemorySessionRepo()).WithClock(fc).WithPhoneVerification(&memoryPhoneRepo{}, sms)
	r := chi.NewRouter()
	userhttp.NewHandler(svc, secret, 24*time.Hour).RegisterRoutes(r)
	return &identityEnv{router: r, secret: secret, repo: repo, clock: fc, sms: sms}
}

func (e *identityEnv) do(t *testing.T, method, path, body, token string) (int, map[string]any) {
	t.Helper()
	rec := doJSON(t, e.router, method, path, body, token)
	var env struct {
		Data  any `json:"data"`
		Error any `json:"error"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	out := map[string]any{}
	if m, ok := env.Data.(map[string]any); ok {
		out = m
	}
	if m, ok := env.Error.(map[string]any); ok {
		out = m
	}
	return rec.Code, out
}

func (e *identityEnv) register(t *testing.T, body string) (int, map[string]any) {
	t.Helper()
	return e.do(t, http.MethodPost, "/auth/register", body, "")
}

// token signs a bearer token for a user stored in the memory repo.
func (e *identityEnv) token(t *testing.T, id string) string {
	t.Helper()
	uid, err := uuid.Parse(id)
	if err != nil {
		t.Fatal(err)
	}
	tok, err := authtoken.Generate(e.secret, time.Hour, uid, "user")
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func TestIdentityHTTP_RegisterNames(t *testing.T) {
	env := newIdentityEnv()

	status, data := env.register(t, `{"first_name":"María José","last_name":"O'Brien-Pérez","email":"mj@example.com","password":"Tr1cky-Pass9x","password_confirmation":"Tr1cky-Pass9x"}`)
	if status != http.StatusCreated {
		t.Fatalf("status %d: %v", status, data)
	}
	if data["first_name"] != "María José" || data["last_name"] != "O'Brien-Pérez" || data["name"] != "María José O'Brien-Pérez" || data["locale"] != "es" {
		t.Errorf("unexpected user: %v", data)
	}

	t.Run("legacy name is split and returned", func(t *testing.T) {
		status, data := env.register(t, `{"name":"Ana Perez Lopez","email":"ana@example.com","password":"Tr1cky-Pass9x","password_confirmation":"Tr1cky-Pass9x"}`)
		if status != http.StatusCreated || data["first_name"] != "Ana" || data["last_name"] != "Perez Lopez" || data["name"] != "Ana Perez Lopez" {
			t.Fatalf("status %d: %v", status, data)
		}
	})

	t.Run("extra spaces are collapsed", func(t *testing.T) {
		_, data := env.register(t, `{"first_name":"  Luis   Miguel ","last_name":" Gomez ","email":"lm@example.com","password":"Tr1cky-Pass9x","password_confirmation":"Tr1cky-Pass9x"}`)
		if data["first_name"] != "Luis Miguel" || data["last_name"] != "Gomez" {
			t.Errorf("not normalized: %v", data)
		}
	})

	for name, body := range map[string]string{
		"digits in first name":  `{"first_name":"Ana2","last_name":"Perez","email":"x1@example.com","password":"Tr1cky-Pass9x","password_confirmation":"Tr1cky-Pass9x"}`,
		"symbols in last name":  `{"first_name":"Ana","last_name":"Perez!","email":"x2@example.com","password":"Tr1cky-Pass9x","password_confirmation":"Tr1cky-Pass9x"}`,
		"missing last name":     `{"first_name":"Ana","email":"x3@example.com","password":"Tr1cky-Pass9x","password_confirmation":"Tr1cky-Pass9x"}`,
		"digits in legacy name": `{"name":"User 1","email":"x4@example.com","password":"Tr1cky-Pass9x","password_confirmation":"Tr1cky-Pass9x"}`,
	} {
		t.Run("rejects "+name, func(t *testing.T) {
			status, data := env.register(t, body)
			if status != http.StatusUnprocessableEntity || data["code"] != "user.invalid_name" {
				t.Errorf("status %d: %v", status, data)
			}
		})
	}

	t.Run("no name at all", func(t *testing.T) {
		status, data := env.register(t, `{"email":"x5@example.com","password":"Tr1cky-Pass9x","password_confirmation":"Tr1cky-Pass9x"}`)
		if status != http.StatusUnprocessableEntity || data["code"] != "user.name_required" {
			t.Errorf("status %d: %v", status, data)
		}
	})
}

func TestIdentityHTTP_RegisterPasswordConfirmation(t *testing.T) {
	cases := map[string]string{
		"mismatch":                `"password":"Tr1cky-Pass9x","password_confirmation":"password124"`,
		"missing":                 `"password":"Tr1cky-Pass9x"`,
		"whitespace counts":       `"password":"Tr1cky-Pass9x","password_confirmation":"Tr1cky-Pass9x "`,
		"leading whitespace":      `"password":"Tr1cky-Pass9x","password_confirmation":" Tr1cky-Pass9x"`,
		"case counts":             `"password":"Tr1cky-Pass9x","password_confirmation":"Password123"`,
		"mismatch before policy":  `"password":"123","password_confirmation":"12"`,
		"empty confirmation only": `"password":"Tr1cky-Pass9x","password_confirmation":""`,
	}
	for name, pw := range cases {
		t.Run(name, func(t *testing.T) {
			env := newIdentityEnv()
			status, data := env.register(t, `{"first_name":"Ana","last_name":"Perez","email":"ana@example.com",`+pw+`}`)
			if status != http.StatusUnprocessableEntity || data["code"] != "user.password_mismatch" {
				t.Fatalf("status %d: %v", status, data)
			}
			if data["message"] != "Las contraseñas no coinciden" {
				t.Errorf("message %v", data["message"])
			}
			details, _ := data["details"].([]any)
			if len(details) != 1 || details[0].(map[string]any)["field"] != "password_confirmation" {
				t.Errorf("details %v", data["details"])
			}
			if len(env.repo.users) != 0 {
				t.Errorf("a user was created on mismatch")
			}
		})
	}

	t.Run("matching confirmation registers", func(t *testing.T) {
		env := newIdentityEnv()
		status, _ := env.register(t, `{"first_name":"Ana","last_name":"Perez","email":"ana@example.com","password":"Tr1cky-Pass9x","password_confirmation":"Tr1cky-Pass9x"}`)
		if status != http.StatusCreated {
			t.Fatalf("status %d", status)
		}
	})

	t.Run("password policy still applies when they match", func(t *testing.T) {
		env := newIdentityEnv()
		status, data := env.register(t, `{"first_name":"Ana","last_name":"Perez","email":"ana@example.com","password":"123","password_confirmation":"123"}`)
		if status != http.StatusUnprocessableEntity || data["code"] != "validation.failed" {
			t.Fatalf("status %d: %v", status, data)
		}
	})
}

func TestIdentityHTTP_ChangePasswordConfirmation(t *testing.T) {
	env := newIdentityEnv()
	if status, _ := env.register(t, `{"first_name":"Ana","last_name":"Perez","email":"ana@example.com","password":"Tr1cky-Pass9x","password_confirmation":"Tr1cky-Pass9x"}`); status != http.StatusCreated {
		t.Fatal("register failed")
	}
	var id string
	for k := range env.repo.users {
		id = k.String()
	}
	token := env.token(t, id)

	for name, body := range map[string]string{
		"mismatch": `{"current_password":"Tr1cky-Pass9x","new_password":"New-Pass-4567x","new_password_confirmation":"newpassword2"}`,
		"missing":  `{"current_password":"Tr1cky-Pass9x","new_password":"New-Pass-4567x"}`,
		"spaces":   `{"current_password":"Tr1cky-Pass9x","new_password":"New-Pass-4567x","new_password_confirmation":"New-Pass-4567x "}`,
	} {
		t.Run(name, func(t *testing.T) {
			status, data := env.do(t, http.MethodPost, "/users/me/password", body, token)
			if status != http.StatusUnprocessableEntity || data["code"] != "user.password_mismatch" {
				t.Fatalf("status %d: %v", status, data)
			}
			details, _ := data["details"].([]any)
			if len(details) != 1 || details[0].(map[string]any)["field"] != "new_password_confirmation" {
				t.Errorf("details %v", data["details"])
			}
			if status, _ := env.do(t, http.MethodPost, "/auth/login", `{"email":"ana@example.com","password":"Tr1cky-Pass9x"}`, ""); status != http.StatusOK {
				t.Errorf("old password no longer valid after a mismatch (status %d)", status)
			}
		})
	}

	t.Run("matching change works and the old password stops", func(t *testing.T) {
		status, _ := env.do(t, http.MethodPost, "/users/me/password", `{"current_password":"Tr1cky-Pass9x","new_password":"New-Pass-4567x","new_password_confirmation":"New-Pass-4567x"}`, token)
		if status != http.StatusNoContent {
			t.Fatalf("status %d", status)
		}
		if status, _ := env.do(t, http.MethodPost, "/auth/login", `{"email":"ana@example.com","password":"Tr1cky-Pass9x"}`, ""); status != http.StatusUnauthorized {
			t.Errorf("old password still valid: %d", status)
		}
	})
}

func TestIdentityHTTP_Phone(t *testing.T) {
	env := newIdentityEnv()

	status, data := env.register(t, `{"first_name":"Rosa","last_name":"Diaz","email":"rosa@example.com","password":"Tr1cky-Pass9x","password_confirmation":"Tr1cky-Pass9x","phone":{"country":"PE","number":"0912 345-678"}}`)
	if status != http.StatusCreated {
		t.Fatalf("status %d: %v", status, data)
	}
	phone, _ := data["phone"].(map[string]any)
	if phone["country"] != "PE" || phone["number"] != "912345678" || phone["e164"] != "+51912345678" || phone["verified"] != false {
		t.Fatalf("phone %v", phone)
	}

	t.Run("invalid phones", func(t *testing.T) {
		for name, body := range map[string]string{
			"mismatched prefix": `{"first_name":"A","last_name":"B","email":"p1@example.com","password":"Tr1cky-Pass9x","password_confirmation":"Tr1cky-Pass9x","phone":{"country":"PE","number":"+52 5512345678"}}`,
			"unknown country":   `{"first_name":"A","last_name":"B","email":"p2@example.com","password":"Tr1cky-Pass9x","password_confirmation":"Tr1cky-Pass9x","phone":{"country":"ZZ","number":"123456789"}}`,
			"letters":           `{"first_name":"A","last_name":"B","email":"p3@example.com","password":"Tr1cky-Pass9x","password_confirmation":"Tr1cky-Pass9x","phone":{"country":"PE","number":"91234abcd"}}`,
		} {
			status, data := env.register(t, body)
			if status != http.StatusUnprocessableEntity || data["code"] != "user.invalid_phone" {
				t.Errorf("%s: status %d: %v", name, status, data)
			}
		}
	})

	var id string
	for k := range env.repo.users {
		id = k.String()
	}
	token := env.token(t, id)

	t.Run("verification flow", func(t *testing.T) {
		// Wrong code first.
		if status, _ := env.do(t, http.MethodPost, "/users/me/phone/verification", ``, token); status != http.StatusAccepted {
			t.Fatalf("request status %d", status)
		}
		if env.sms.to != "+51912345678" || len(env.sms.code) != 6 {
			t.Fatalf("sms %+v", env.sms)
		}
		wrong := "000000"
		if env.sms.code == wrong {
			wrong = "111111"
		}
		status, data := env.do(t, http.MethodPost, "/users/me/phone/verify", `{"code":"`+wrong+`"}`, token)
		if status != http.StatusUnprocessableEntity || data["code"] != "user.phone_code_invalid" {
			t.Fatalf("wrong code: %d %v", status, data)
		}
		status, data = env.do(t, http.MethodPost, "/users/me/phone/verify", `{"code":"`+env.sms.code+`"}`, token)
		if status != http.StatusOK {
			t.Fatalf("verify: %d %v", status, data)
		}
		if p, _ := data["phone"].(map[string]any); p["verified"] != true || p["verified_at"] == nil {
			t.Errorf("not verified: %v", data["phone"])
		}
		if status, data := env.do(t, http.MethodPost, "/users/me/phone/verification", ``, token); status != http.StatusConflict || data["code"] != "user.phone_already_verified" {
			t.Errorf("already verified: %d %v", status, data)
		}
	})

	t.Run("changing the number clears the verification", func(t *testing.T) {
		status, data := env.do(t, http.MethodPatch, "/users/me", `{"phone":{"country":"PE","number":"+51 987 654 321"}}`, token)
		if status != http.StatusOK {
			t.Fatalf("status %d: %v", status, data)
		}
		if p, _ := data["phone"].(map[string]any); p["e164"] != "+51987654321" || p["verified"] != false {
			t.Errorf("phone %v", data["phone"])
		}
		// Resending the same number keeps whatever state it has.
		if status, _ := env.do(t, http.MethodPost, "/users/me/phone/verification", ``, token); status != http.StatusAccepted {
			t.Fatal("request failed")
		}
		if status, _ := env.do(t, http.MethodPost, "/users/me/phone/verify", `{"code":"`+env.sms.code+`"}`, token); status != http.StatusOK {
			t.Fatal("verify failed")
		}
		status, data = env.do(t, http.MethodPatch, "/users/me", `{"phone":{"country":"PE","number":"987654321"}}`, token)
		if p, _ := data["phone"].(map[string]any); status != http.StatusOK || p["verified"] != true {
			t.Errorf("same number lost its verification: %v", data)
		}
	})

	t.Run("null clears the phone and omitted keeps it", func(t *testing.T) {
		status, data := env.do(t, http.MethodPatch, "/users/me", `{"job_title":"QA"}`, token)
		if status != http.StatusOK || data["phone"] == nil {
			t.Fatalf("omitted must keep the phone: %d %v", status, data)
		}
		status, data = env.do(t, http.MethodPatch, "/users/me", `{"phone":null}`, token)
		if status != http.StatusOK {
			t.Fatalf("status %d", status)
		}
		if _, ok := data["phone"]; ok {
			t.Errorf("phone must be absent after clearing: %v", data["phone"])
		}
		if status, data := env.do(t, http.MethodPost, "/users/me/phone/verification", ``, token); status != http.StatusConflict || data["code"] != "user.phone_not_set" {
			t.Errorf("not set: %d %v", status, data)
		}
	})
}

func TestIdentityHTTP_PhoneVerificationLimits(t *testing.T) {
	newUser := func() (*identityEnv, string) {
		env := newIdentityEnv()
		_, data := env.register(t, `{"first_name":"Rosa","last_name":"Diaz","email":"rosa@example.com","password":"Tr1cky-Pass9x","password_confirmation":"Tr1cky-Pass9x","phone":{"country":"MX","number":"5512345678"}}`)
		return env, env.token(t, data["id"].(string))
	}

	t.Run("three sends per hour", func(t *testing.T) {
		env, token := newUser()
		for i := 0; i < 3; i++ {
			if status, _ := env.do(t, http.MethodPost, "/users/me/phone/verification", ``, token); status != http.StatusAccepted {
				t.Fatalf("send %d: %d", i, status)
			}
		}
		status, data := env.do(t, http.MethodPost, "/users/me/phone/verification", ``, token)
		if status != http.StatusTooManyRequests || data["code"] != "user.phone_send_limit" {
			t.Fatalf("fourth send: %d %v", status, data)
		}
		env.clock.Advance(61 * time.Minute)
		if status, _ := env.do(t, http.MethodPost, "/users/me/phone/verification", ``, token); status != http.StatusAccepted {
			t.Errorf("window did not slide: %d", status)
		}
	})

	t.Run("five attempts kill the code", func(t *testing.T) {
		env, token := newUser()
		env.do(t, http.MethodPost, "/users/me/phone/verification", ``, token)
		good := env.sms.code
		wrong := "000000"
		if good == wrong {
			wrong = "111111"
		}
		for i := 1; i <= 4; i++ {
			if _, data := env.do(t, http.MethodPost, "/users/me/phone/verify", `{"code":"`+wrong+`"}`, token); data["code"] != "user.phone_code_invalid" {
				t.Fatalf("attempt %d: %v", i, data)
			}
		}
		if status, data := env.do(t, http.MethodPost, "/users/me/phone/verify", `{"code":"`+wrong+`"}`, token); status != http.StatusTooManyRequests || data["code"] != "user.phone_code_attempts" {
			t.Fatalf("fifth attempt: %d %v", status, data)
		}
		// Even the right code is refused now.
		if status, data := env.do(t, http.MethodPost, "/users/me/phone/verify", `{"code":"`+good+`"}`, token); status != http.StatusTooManyRequests {
			t.Errorf("right code after lockout: %d %v", status, data)
		}
	})

	t.Run("expired code", func(t *testing.T) {
		env, token := newUser()
		env.do(t, http.MethodPost, "/users/me/phone/verification", ``, token)
		env.clock.Advance(11 * time.Minute)
		status, data := env.do(t, http.MethodPost, "/users/me/phone/verify", `{"code":"`+env.sms.code+`"}`, token)
		if status != http.StatusUnprocessableEntity || data["code"] != "user.phone_code_expired" {
			t.Fatalf("expired: %d %v", status, data)
		}
	})

	t.Run("no code requested", func(t *testing.T) {
		env, token := newUser()
		status, data := env.do(t, http.MethodPost, "/users/me/phone/verify", `{"code":"123456"}`, token)
		if status != http.StatusUnprocessableEntity || data["code"] != "user.phone_code_expired" {
			t.Fatalf("%d %v", status, data)
		}
	})

	t.Run("malformed code", func(t *testing.T) {
		env, token := newUser()
		if status, _ := env.do(t, http.MethodPost, "/users/me/phone/verify", `{"code":"12ab"}`, token); status != http.StatusUnprocessableEntity {
			t.Errorf("status %d", status)
		}
	})
}

func TestIdentityHTTP_PhonePrivacyMatrix(t *testing.T) {
	env := newIdentityEnv()
	org := uuid.New()
	env.repo.memberships[org] = map[uuid.UUID]string{}

	add := func(name, email, role string, inOrg bool) (*domain.User, string) {
		u, _ := domain.NewUser(name, email, "hash", domain.RoleUser)
		p, err := domain.NewPhone("PE", "912345678")
		if err != nil {
			t.Fatal(err)
		}
		u.SetPhone(p)
		_ = env.repo.Save(context.Background(), u)
		if inOrg {
			env.repo.memberships[org][u.ID] = role
		}
		return u, env.token(t, u.ID.String())
	}
	_, ownerTok := add("Olga Owner", "owner@example.com", "owner", true)
	_, adminTok := add("Adan Admin", "admin@example.com", "admin", true)
	member, memberTok := add("Mario Member", "member@example.com", "member", true)
	_, outsiderTok := add("Omar Outside", "out@example.com", "", false)

	phonesSeen := func(tok string) (seen int, total int) {
		status, rec := listDirectory(t, env, org, tok)
		if status != http.StatusOK {
			return -1, -1
		}
		for _, u := range rec {
			total++
			if u["phone"] != nil {
				seen++
			}
		}
		return seen, total
	}

	if seen, total := phonesSeen(ownerTok); seen != 3 || total != 3 {
		t.Errorf("owner sees %d/%d phones, want all 3", seen, total)
	}
	if seen, total := phonesSeen(adminTok); seen != total || total != 3 {
		t.Errorf("admin sees %d/%d phones, want all", seen, total)
	}
	if seen, total := phonesSeen(memberTok); seen != 1 || total != 3 {
		t.Errorf("member sees %d/%d phones, want only their own", seen, total)
	}
	if status, _ := listDirectory(t, env, org, outsiderTok); status != http.StatusNotFound {
		t.Errorf("outsider status %d, want 404", status)
	}

	// The member always sees their own phone with every field, others see first and last names.
	_, rec := listDirectory(t, env, org, memberTok)
	for _, u := range rec {
		if u["first_name"] == nil || u["last_name"] == nil {
			t.Errorf("directory entry without names: %v", u)
		}
		if u["id"] == member.ID.String() && u["phone"] == nil {
			t.Errorf("member does not see their own phone")
		}
	}
	// The raw JSON of a member's view of someone else never contains the number.
	rawRec := doJSON(t, env.router, http.MethodGet, "/users/search?organization_id="+org.String(), "", memberTok)
	if strings.Count(rawRec.Body.String(), `"phone":`) != 1 {
		t.Errorf("a member can read other phones: %s", rawRec.Body.String())
	}
}

func listDirectory(t *testing.T, env *identityEnv, org uuid.UUID, token string) (int, []map[string]any) {
	t.Helper()
	rec := doJSON(t, env.router, http.MethodGet, "/users/search?organization_id="+org.String(), "", token)
	var body struct {
		Data []map[string]any `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec.Code, body.Data
}

func TestIdentityHTTP_Countries(t *testing.T) {
	env := newIdentityEnv()
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/meta/countries", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Cache-Control"), "max-age") {
		t.Errorf("not cacheable: %q", rec.Header().Get("Cache-Control"))
	}
	var body struct {
		Data []struct {
			Code     string `json:"code"`
			DialCode string `json:"dial_code"`
			Name     string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data) < 200 {
		t.Errorf("only %d countries", len(body.Data))
	}
	found := false
	for _, c := range body.Data {
		if c.Code == "PE" && c.DialCode == "+51" && c.Name == "Perú" {
			found = true
		}
	}
	if !found {
		t.Error("PE missing")
	}
}

func TestIdentityHTTP_ProfileNames(t *testing.T) {
	env := newIdentityEnv()
	_, data := env.register(t, `{"first_name":"Ana","last_name":"Perez","email":"ana@example.com","password":"Tr1cky-Pass9x","password_confirmation":"Tr1cky-Pass9x"}`)
	token := env.token(t, data["id"].(string))

	status, data := env.do(t, http.MethodPatch, "/users/me", `{"first_name":"Ana María","last_name":"Pérez"}`, token)
	if status != http.StatusOK || data["name"] != "Ana María Pérez" || data["display_name"] != "Ana María Pérez" {
		t.Fatalf("%d %v", status, data)
	}
	status, data = env.do(t, http.MethodPatch, "/users/me", `{"name":"Luisa Gomez Ruiz"}`, token)
	if status != http.StatusOK || data["first_name"] != "Luisa" || data["last_name"] != "Gomez Ruiz" {
		t.Fatalf("legacy name: %d %v", status, data)
	}
	if status, data := env.do(t, http.MethodPatch, "/users/me", `{"first_name":"Ana9"}`, token); status != http.StatusUnprocessableEntity || data["code"] != "user.invalid_name" {
		t.Errorf("digits: %d %v", status, data)
	}
	if status, data := env.do(t, http.MethodPatch, "/users/me", `{"locale":"EN"}`, token); status != http.StatusUnprocessableEntity || data["code"] != "user.locale_invalid" {
		t.Errorf("locale: %d %v", status, data)
	}
	if status, data := env.do(t, http.MethodPatch, "/users/me", `{"locale":"es-PE","timezone":"America/Lima"}`, token); status != http.StatusOK || data["locale"] != "es-PE" {
		t.Errorf("locale ok: %d %v", status, data)
	}
}
