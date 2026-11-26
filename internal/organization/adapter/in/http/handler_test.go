package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/organization/application/port"
	"github.com/jhonsferg/corelog/internal/organization/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/authtoken"
)

const testSecret = "test-secret-with-enough-length-0123456789"

var httpNow = time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

// fakeService implements port.Service. Each method returns err when set; otherwise it returns the
// canned value of its field and records the last input it received.
type fakeService struct {
	err         error
	org         *domain.Organization
	memberships []port.Membership
	invs        []*domain.Invitation
	results     []port.InvitationResult
	preview     *port.InvitationPreview
	member      *domain.Member
	views       []port.MemberView
	profile     *port.MemberProfile

	lastUpdate port.UpdateInput
	lastInvite []port.InviteInput
	lastEmail  string
	lastStatus bool
}

var _ port.Service = (*fakeService)(nil)

func (f *fakeService) CreateOrganization(_ context.Context, _ uuid.UUID, _ port.CreateInput) (*domain.Organization, error) {
	return f.org, f.err
}

func (f *fakeService) GetOrganization(_ context.Context, _, _ uuid.UUID) (*domain.Organization, error) {
	return f.org, f.err
}

func (f *fakeService) UpdateOrganization(_ context.Context, _, _ uuid.UUID, in port.UpdateInput) (*domain.Organization, error) {
	f.lastUpdate = in
	return f.org, f.err
}

func (f *fakeService) ListMine(_ context.Context, _ uuid.UUID) ([]port.Membership, error) {
	return f.memberships, f.err
}

func (f *fakeService) DeleteOrganization(_ context.Context, _, _ uuid.UUID, _ string) error {
	return f.err
}

func (f *fakeService) LeaveOrganization(_ context.Context, _, _ uuid.UUID) error { return f.err }

func (f *fakeService) TransferOwnership(_ context.Context, _, _, _ uuid.UUID) error { return f.err }

func (f *fakeService) RequireRole(_ context.Context, _, _ uuid.UUID, _ domain.Role) (domain.Role, error) {
	if f.err != nil {
		return "", f.err
	}
	return domain.RoleOwner, nil
}

func (f *fakeService) ListMembers(_ context.Context, _, _ uuid.UUID) ([]port.MemberView, error) {
	return f.views, f.err
}

func (f *fakeService) GetMemberProfile(_ context.Context, _, _, _ uuid.UUID) (*port.MemberProfile, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.profile, nil
}

func (f *fakeService) ChangeMemberRole(_ context.Context, _, _, _ uuid.UUID, _ domain.Role) error {
	return f.err
}

func (f *fakeService) RemoveMember(_ context.Context, _, _, _ uuid.UUID) error { return f.err }

func (f *fakeService) CreateInvitations(_ context.Context, _, _ uuid.UUID, in []port.InviteInput) ([]port.InvitationResult, error) {
	f.lastInvite = in
	return f.results, f.err
}

func (f *fakeService) ListInvitations(_ context.Context, _, _ uuid.UUID, pendingOnly bool) ([]*domain.Invitation, error) {
	f.lastStatus = pendingOnly
	return f.invs, f.err
}

func (f *fakeService) ResendInvitation(_ context.Context, _, _, _ uuid.UUID) (*port.InvitationResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	if len(f.results) == 0 {
		return nil, domain.ErrInvitationNotFound
	}
	return &f.results[0], nil
}

func (f *fakeService) RevokeInvitation(_ context.Context, _, _, _ uuid.UUID) error { return f.err }

func (f *fakeService) GetInvitationPreview(_ context.Context, _ string) (*port.InvitationPreview, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.preview, nil
}

func (f *fakeService) AcceptInvitation(_ context.Context, _ uuid.UUID, email, _ string) (*domain.Member, error) {
	f.lastEmail = email
	return f.member, f.err
}

type fakeEmails struct {
	email string
	err   error
}

func (e fakeEmails) EmailOf(_ context.Context, _ uuid.UUID) (string, error) {
	return e.email, e.err
}

// newTestRouter mounts the routes under /api/v1 exactly as cmd/api does.
func newTestRouter(svc *fakeService, emails UserEmailLookup, publicURL string) chi.Router {
	h := NewHandler(svc, emails, fixedClock{t: httpNow}, testSecret, publicURL)
	r := chi.NewRouter()
	r.Route("/api/v1", h.RegisterRoutes)
	return r
}

func tokenFor(t *testing.T, userID uuid.UUID) string {
	t.Helper()
	tok, err := authtoken.Generate(testSecret, time.Hour, userID, "user")
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	return tok
}

func doRequest(t *testing.T, router http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// errorCode extracts error.code from an error envelope and checks that a message is present.
func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body %q: %v", rec.Body.String(), err)
	}
	if body.Error.Message == "" {
		t.Fatalf("error body without message: %s", rec.Body.String())
	}
	return body.Error.Code
}

// dataKeys returns the top-level keys of the data object, for shape checks.
func dataKeys(t *testing.T, rec *httptest.ResponseRecorder) map[string]json.RawMessage {
	t.Helper()
	var body struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode data body %q: %v", rec.Body.String(), err)
	}
	return body.Data
}

func sampleOrg() *domain.Organization {
	return &domain.Organization{
		ID: uuid.New(), Slug: "acme", Name: "Acme", Description: "d", Settings: domain.DefaultSettings(),
		CreatedAt: httpNow, UpdatedAt: httpNow,
	}
}

func TestOrgRoutesRequireAuthentication(t *testing.T) {
	router := newTestRouter(&fakeService{org: sampleOrg()}, fakeEmails{}, "")
	orgPath := "/api/v1/organizations/" + uuid.NewString()
	cases := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/organizations"},
		{http.MethodPost, "/api/v1/organizations"},
		{http.MethodGet, orgPath},
		{http.MethodPatch, orgPath},
		{http.MethodDelete, orgPath},
		{http.MethodPost, orgPath + "/leave"},
		{http.MethodPost, orgPath + "/transfer-ownership"},
		{http.MethodGet, orgPath + "/members"},
		{http.MethodPatch, orgPath + "/members/" + uuid.NewString()},
		{http.MethodDelete, orgPath + "/members/" + uuid.NewString()},
		{http.MethodGet, orgPath + "/invitations"},
		{http.MethodPost, orgPath + "/invitations"},
		{http.MethodPost, orgPath + "/invitations/" + uuid.NewString() + "/resend"},
		{http.MethodDelete, orgPath + "/invitations/" + uuid.NewString()},
		{http.MethodPost, "/api/v1/invitations/tok/accept"},
	}
	for _, c := range cases {
		rec := doRequest(t, router, c.method, c.path, "", "{}")
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without token: status %d, want 401", c.method, c.path, rec.Code)
		}
	}
}

func TestPreviewIsPublic(t *testing.T) {
	svc := &fakeService{preview: &port.InvitationPreview{
		OrganizationName: "Acme", Email: "b@example.test", Role: domain.RoleMember,
		Status: domain.InvitationPending, ExpiresAt: httpNow.Add(time.Hour),
	}}
	router := newTestRouter(svc, fakeEmails{}, "")
	rec := doRequest(t, router, http.MethodGet, "/api/v1/invitations/tok123", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	keys := dataKeys(t, rec)
	for _, k := range []string{"organization_name", "role", "email", "status", "expires_at"} {
		if _, ok := keys[k]; !ok {
			t.Errorf("preview misses %q: %s", k, rec.Body.String())
		}
	}
}

func TestPreviewUnknownTokenHasNotFoundShape(t *testing.T) {
	router := newTestRouter(&fakeService{err: domain.ErrInvitationNotFound}, fakeEmails{}, "")
	rec := doRequest(t, router, http.MethodGet, "/api/v1/invitations/unknown", "", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d", rec.Code)
	}
	if code := errorCode(t, rec); code != "org.invitation_not_found" {
		t.Fatalf("code %q", code)
	}
	var generic map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &generic); err != nil {
		t.Fatal(err)
	}
	if len(generic) != 1 || generic["error"] == nil {
		t.Fatalf("unexpected top-level shape: %s", rec.Body.String())
	}
}

func TestCreateOrganizationReturns201WithSnakeCaseShape(t *testing.T) {
	router := newTestRouter(&fakeService{org: sampleOrg()}, fakeEmails{}, "")
	rec := doRequest(t, router, http.MethodPost, "/api/v1/organizations", tokenFor(t, uuid.New()), `{"name":"Acme"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	keys := dataKeys(t, rec)
	for _, k := range []string{"id", "slug", "name", "description", "settings", "role", "created_at", "updated_at"} {
		if _, ok := keys[k]; !ok {
			t.Errorf("organization misses %q", k)
		}
	}
	var settings map[string]any
	if err := json.Unmarshal(keys["settings"], &settings); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"who_can_create_projects", "who_can_create_teams", "default_project_visibility", "require_2fa", "max_upload_bytes", "hours_per_day", "days_per_week", "timezone"} {
		if _, ok := settings[k]; !ok {
			t.Errorf("settings misses %q", k)
		}
	}
}

func TestBodyDecodingIsStrictAndValidated(t *testing.T) {
	router := newTestRouter(&fakeService{org: sampleOrg()}, fakeEmails{}, "")
	token := tokenFor(t, uuid.New())

	rec := doRequest(t, router, http.MethodPost, "/api/v1/organizations", token, `{"name":"A","unknown":1}`)
	if rec.Code != http.StatusUnprocessableEntity || errorCode(t, rec) != CodeBodyInvalid {
		t.Fatalf("unknown field: status %d body %s", rec.Code, rec.Body.String())
	}

	rec = doRequest(t, router, http.MethodPost, "/api/v1/organizations", token, `{"slug":"x"}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing name: status %d", rec.Code)
	}
	if code := errorCode(t, rec); code != "validation.failed" {
		t.Fatalf("missing name code %q", code)
	}

	rec = doRequest(t, router, http.MethodPost, "/api/v1/organizations", token, `{not json`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("malformed json: status %d", rec.Code)
	}
}

func TestInvalidPathIDIs422(t *testing.T) {
	router := newTestRouter(&fakeService{}, fakeEmails{}, "")
	rec := doRequest(t, router, http.MethodGet, "/api/v1/organizations/not-a-uuid/members", tokenFor(t, uuid.New()), "")
	if rec.Code != http.StatusUnprocessableEntity || errorCode(t, rec) != CodeIDInvalid {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestErrorMappingStatusAndCode(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{domain.ErrNotFound, 404, "org.not_found"},
		{domain.ErrMemberNotFound, 404, "org.member_not_found"},
		{domain.ErrInvitationNotFound, 404, "org.invitation_not_found"},
		{domain.ErrForbidden, 403, "org.forbidden"},
		{domain.ErrCannotRemoveOwner, 403, "org.cannot_remove_owner"},
		{domain.ErrEmailMismatch, 403, "org.email_mismatch"},
		{domain.ErrSlugTaken, 409, "org.slug_taken"},
		{domain.ErrLimitReached, 409, "org.limit_reached"},
		{domain.ErrLastOwner, 409, "org.last_owner"},
		{domain.ErrMemberExists, 409, "org.member_exists"},
		{domain.ErrAlreadyMember, 409, "org.already_member"},
		{domain.ErrInvitationExists, 409, "org.invitation_exists"},
		{domain.ErrInvitationUsed, 409, "org.invitation_used"},
		{domain.ErrInvitationRevoked, 409, "org.invitation_revoked"},
		{domain.ErrInvitationExpired, 409, "org.invitation_expired"},
		{domain.ErrSelfRoleChange, 422, "org.self_role_change"},
		{domain.ErrTransferSelf, 422, "org.transfer_self"},
		{domain.ErrConfirmationMismatch, 422, "org.confirmation_mismatch"},
		{domain.ErrRoleInvalid, 422, "org.role_invalid"},
		{domain.ErrInvitationBatchInvalid, 422, "org.invitation_batch_invalid"},
		{domain.ErrInvitationEmailInvalid, 422, "org.invitation_email_invalid"},
	}
	token := tokenFor(t, uuid.New())
	for _, c := range cases {
		router := newTestRouter(&fakeService{err: c.err}, fakeEmails{email: "b@example.test"}, "")
		rec := doRequest(t, router, http.MethodGet, "/api/v1/organizations/"+uuid.NewString()+"/members", token, "")
		if rec.Code != c.status {
			t.Errorf("%s: status %d, want %d", c.code, rec.Code, c.status)
		}
		if got := errorCode(t, rec); got != c.code {
			t.Errorf("code %q, want %q", got, c.code)
		}
	}
}

func TestSettingsPatchIsMergedOverCurrentValues(t *testing.T) {
	org := sampleOrg()
	org.Settings.Timezone = "Europe/Madrid"
	svc := &fakeService{org: org}
	router := newTestRouter(svc, fakeEmails{}, "")
	token := tokenFor(t, uuid.New())
	path := "/api/v1/organizations/" + uuid.NewString()

	rec := doRequest(t, router, http.MethodPatch, path, token, `{"name":"Nuevo","settings":{"require_2fa":true}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if svc.lastUpdate.Name == nil || *svc.lastUpdate.Name != "Nuevo" {
		t.Fatalf("name not forwarded: %+v", svc.lastUpdate)
	}
	if svc.lastUpdate.Settings == nil || svc.lastUpdate.Settings.Require2FA == nil || !*svc.lastUpdate.Settings.Require2FA {
		t.Fatalf("settings patch not forwarded: %+v", svc.lastUpdate.Settings)
	}
	if svc.lastUpdate.Settings.Timezone != nil {
		t.Fatal("omitted timezone must stay nil so the use case keeps the stored value")
	}
	if svc.lastUpdate.Description != nil {
		t.Fatal("omitted description must stay nil")
	}
}

func TestSettingsInvalidReturns422WithDetails(t *testing.T) {
	svc := &fakeService{err: domain.Settings{}.Validate()}
	router := newTestRouter(svc, fakeEmails{}, "")
	rec := doRequest(t, router, http.MethodPatch, "/api/v1/organizations/"+uuid.NewString(), tokenFor(t, uuid.New()), `{"settings":{"hours_per_day":99}}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"details"`) {
		t.Fatalf("details missing: %s", rec.Body.String())
	}
}

func TestCreateInvitationsReturnsTokenAndAcceptURL(t *testing.T) {
	inv, err := domain.NewInvitation(uuid.New(), "b@example.test", domain.RoleMember, uuid.New(), "hash", httpNow)
	if err != nil {
		t.Fatal(err)
	}
	inv.ID = uuid.New()
	svc := &fakeService{results: []port.InvitationResult{{Invitation: inv, PlainToken: "plain-token"}}}
	body := `{"invitations":[{"email":"b@example.test","role":"member"}]}`

	t.Run("request origin when PUBLIC_URL is empty", func(t *testing.T) {
		router := newTestRouter(svc, fakeEmails{}, "")
		rec := doRequest(t, router, http.MethodPost, "/api/v1/organizations/"+uuid.NewString()+"/invitations", tokenFor(t, uuid.New()), body)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
		}
		var out struct {
			Data struct {
				Invitations []map[string]any `json:"invitations"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		if len(out.Data.Invitations) != 1 {
			t.Fatalf("invitations = %d", len(out.Data.Invitations))
		}
		got := out.Data.Invitations[0]
		if got["token"] != "plain-token" || got["accept_url"] != "http://example.com/invite/plain-token" {
			t.Fatalf("token/accept_url = %v / %v", got["token"], got["accept_url"])
		}
		if got["status"] != "pending" || got["email"] != "b@example.test" {
			t.Fatalf("status/email = %v / %v", got["status"], got["email"])
		}
	})

	t.Run("PUBLIC_URL wins over the request origin", func(t *testing.T) {
		router := newTestRouter(svc, fakeEmails{}, "https://app.example.test/")
		rec := doRequest(t, router, http.MethodPost, "/api/v1/organizations/"+uuid.NewString()+"/invitations", tokenFor(t, uuid.New()), body)
		if !strings.Contains(rec.Body.String(), `"accept_url":"https://app.example.test/invite/plain-token"`) {
			t.Fatalf("accept_url not built from PUBLIC_URL: %s", rec.Body.String())
		}
	})
}

func TestCreateInvitationsBatchValidation(t *testing.T) {
	router := newTestRouter(&fakeService{}, fakeEmails{}, "")
	token := tokenFor(t, uuid.New())
	path := "/api/v1/organizations/" + uuid.NewString() + "/invitations"
	for _, body := range []string{
		`{"invitations":[]}`,
		`{"invitations":[{"email":"b@example.test","role":"superuser"}]}`,
		`{"invitations":[{"email":"b@example.test","role":"member","extra":1}]}`,
		`{}`,
	} {
		rec := doRequest(t, router, http.MethodPost, path, token, body)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("body %s: status %d, want 422", body, rec.Code)
		}
	}
}

func TestListInvitationsStatusFilter(t *testing.T) {
	svc := &fakeService{}
	router := newTestRouter(svc, fakeEmails{}, "")
	token := tokenFor(t, uuid.New())
	path := "/api/v1/organizations/" + uuid.NewString() + "/invitations"

	rec := doRequest(t, router, http.MethodGet, path, token, "")
	if rec.Code != http.StatusOK || !svc.lastStatus {
		t.Fatalf("default must list pending: status %d pendingOnly %v", rec.Code, svc.lastStatus)
	}
	rec = doRequest(t, router, http.MethodGet, path+"?status=all", token, "")
	if rec.Code != http.StatusOK || svc.lastStatus {
		t.Fatalf("status=all must not filter: status %d pendingOnly %v", rec.Code, svc.lastStatus)
	}
	rec = doRequest(t, router, http.MethodGet, path+"?status=bogus", token, "")
	if rec.Code != http.StatusUnprocessableEntity || errorCode(t, rec) != CodeStatusFilterInvalid {
		t.Fatalf("bogus status: %d %s", rec.Code, rec.Body.String())
	}
}

func TestAcceptUsesEmailFromLookupAndReturns201(t *testing.T) {
	member, err := domain.NewMember(uuid.New(), uuid.New(), domain.RoleMember, httpNow)
	if err != nil {
		t.Fatal(err)
	}
	svc := &fakeService{member: member}
	router := newTestRouter(svc, fakeEmails{email: "b@example.test"}, "")
	rec := doRequest(t, router, http.MethodPost, "/api/v1/invitations/tok/accept", tokenFor(t, uuid.New()), "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if svc.lastEmail != "b@example.test" {
		t.Fatalf("email passed = %q", svc.lastEmail)
	}
	keys := dataKeys(t, rec)
	for _, k := range []string{"organization_id", "user_id", "role", "joined_at"} {
		if _, ok := keys[k]; !ok {
			t.Errorf("membership misses %q", k)
		}
	}
}

func TestAcceptFailsWhenAccountIsGone(t *testing.T) {
	lookup := fakeEmails{err: apperror.New(apperror.KindUnauthorized, "not authenticated")}
	router := newTestRouter(&fakeService{}, lookup, "")
	rec := doRequest(t, router, http.MethodPost, "/api/v1/invitations/tok/accept", tokenFor(t, uuid.New()), "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestListMembersReturnsSnakeCaseViews(t *testing.T) {
	svc := &fakeService{views: []port.MemberView{{UserID: uuid.New(), Name: "Ana", Email: "a@example.test", Role: domain.RoleOwner, JoinedAt: httpNow}}}
	router := newTestRouter(svc, fakeEmails{}, "")
	rec := doRequest(t, router, http.MethodGet, "/api/v1/organizations/"+uuid.NewString()+"/members", tokenFor(t, uuid.New()), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"joined_at"`) || !strings.Contains(rec.Body.String(), `"user_id"`) {
		t.Fatalf("shape: %s", rec.Body.String())
	}
}

func TestListMembersExposesProfileFieldsWithoutAvatarKey(t *testing.T) {
	svc := &fakeService{views: []port.MemberView{{
		UserID: uuid.New(), Name: "Ana", DisplayName: "Ana M", Email: "a@example.test",
		JobTitle: "Ingeniera", Department: "Plataforma", HasAvatar: true, Role: domain.RoleMember, JoinedAt: httpNow,
	}}}
	router := newTestRouter(svc, fakeEmails{}, "")
	rec := doRequest(t, router, http.MethodGet, "/api/v1/organizations/"+uuid.NewString()+"/members", tokenFor(t, uuid.New()), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var body struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || len(body.Data) != 1 {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	m := body.Data[0]
	if m["display_name"] != "Ana M" || m["job_title"] != "Ingeniera" || m["department"] != "Plataforma" || m["has_avatar"] != true {
		t.Fatalf("profile fields: %v", m)
	}
	if _, ok := m["avatar_key"]; ok {
		t.Fatal("storage key must not be exposed")
	}
}

func TestGetPersonReturnsProfile(t *testing.T) {
	target := uuid.New()
	svc := &fakeService{profile: &port.MemberProfile{
		MemberView: port.MemberView{UserID: target, Name: "Ana", DisplayName: "Ana M", Email: "a@example.test",
			Role: domain.RoleAdmin, JoinedAt: httpNow},
		Timezone: "Europe/Madrid",
	}}
	router := newTestRouter(svc, fakeEmails{}, "")
	rec := doRequest(t, router, http.MethodGet, "/api/v1/organizations/"+uuid.NewString()+"/people/"+target.String(), tokenFor(t, uuid.New()), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := []string{"user_id", "name", "display_name", "email", "job_title", "department", "timezone", "has_avatar", "org_role", "joined_at"}
	for _, k := range want {
		if _, ok := body.Data[k]; !ok {
			t.Fatalf("missing %s in %v", k, body.Data)
		}
	}
	if body.Data["org_role"] != "admin" || body.Data["timezone"] != "Europe/Madrid" {
		t.Fatalf("values: %v", body.Data)
	}
}

func TestGetPersonErrors(t *testing.T) {
	router := newTestRouter(&fakeService{err: domain.ErrForbidden}, fakeEmails{}, "")
	rec := doRequest(t, router, http.MethodGet, "/api/v1/organizations/"+uuid.NewString()+"/people/"+uuid.NewString(), tokenFor(t, uuid.New()), "")
	if rec.Code != http.StatusForbidden || errorCode(t, rec) != "org.forbidden" {
		t.Fatalf("guest: status %d", rec.Code)
	}
	router = newTestRouter(&fakeService{err: domain.ErrMemberNotFound}, fakeEmails{}, "")
	rec = doRequest(t, router, http.MethodGet, "/api/v1/organizations/"+uuid.NewString()+"/people/"+uuid.NewString(), tokenFor(t, uuid.New()), "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("non-member target: status %d", rec.Code)
	}
	rec = doRequest(t, router, http.MethodGet, "/api/v1/organizations/"+uuid.NewString()+"/people/not-a-uuid", tokenFor(t, uuid.New()), "")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid id: status %d", rec.Code)
	}
	rec = doRequest(t, router, http.MethodGet, "/api/v1/organizations/"+uuid.NewString()+"/people/"+uuid.NewString(), "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token: status %d", rec.Code)
	}
}

func TestDeleteOrganizationRequiresConfirmation(t *testing.T) {
	router := newTestRouter(&fakeService{}, fakeEmails{}, "")
	path := "/api/v1/organizations/" + uuid.NewString()
	rec := doRequest(t, router, http.MethodDelete, path, tokenFor(t, uuid.New()), `{}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing confirm_slug: status %d", rec.Code)
	}
	rec = doRequest(t, router, http.MethodDelete, path, tokenFor(t, uuid.New()), `{"confirm_slug":"acme"}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d", rec.Code)
	}
}
