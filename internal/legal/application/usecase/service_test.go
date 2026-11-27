package usecase

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/legal/application/port"
	"github.com/jhonsferg/corelog/internal/legal/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/events"
)

type fakeRepo struct {
	docs     []domain.Document
	consents []domain.Consent
}

func (f *fakeRepo) ListCurrent(_ context.Context, now time.Time) ([]domain.Document, error) {
	best := map[domain.Slug]domain.Document{}
	for _, d := range f.docs {
		if d.EffectiveFrom.After(now) {
			continue
		}
		if b, ok := best[d.Slug]; !ok || d.Version > b.Version {
			best[d.Slug] = d
		}
	}
	out := []domain.Document{}
	for _, d := range best {
		d.BodyMarkdown = ""
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out, nil
}

func (f *fakeRepo) GetCurrent(ctx context.Context, slug domain.Slug, now time.Time) (*domain.Document, error) {
	cur, _ := f.ListCurrent(ctx, now)
	for _, d := range cur {
		if d.Slug == slug {
			return &d, nil
		}
	}
	return nil, domain.ErrDocumentNotFound
}

func (f *fakeRepo) GetVersion(_ context.Context, slug domain.Slug, v int) (*domain.Document, error) {
	for _, d := range f.docs {
		if d.Slug == slug && d.Version == v {
			return &d, nil
		}
	}
	return nil, domain.ErrDocumentNotFound
}

func (f *fakeRepo) ListAll(context.Context) ([]domain.Document, error) { return f.docs, nil }

func (f *fakeRepo) Publish(_ context.Context, d *domain.Document, now time.Time) error {
	max := 0
	for _, x := range f.docs {
		if x.Slug == d.Slug && x.Version > max {
			max = x.Version
		}
	}
	d.ID, d.Version, d.EffectiveFrom, d.PublishedAt = uuid.New(), max+1, now, now
	f.docs = append(f.docs, *d)
	return nil
}

func (f *fakeRepo) ActiveConsents(_ context.Context, userID uuid.UUID) ([]domain.Accepted, error) {
	var out []domain.Accepted
	for _, c := range f.consents {
		if c.UserID == userID && c.RevokedAt == nil {
			out = append(out, domain.Accepted{Slug: c.DocumentSlug, Version: c.DocumentVersion, AcceptedAt: c.AcceptedAt})
		}
	}
	return out, nil
}

func (f *fakeRepo) SaveConsent(_ context.Context, c domain.Consent) (bool, error) {
	for _, x := range f.consents {
		if x.UserID == c.UserID && x.DocumentSlug == c.DocumentSlug && x.DocumentVersion == c.DocumentVersion && x.RevokedAt == nil {
			return false, nil
		}
	}
	c.ID = uuid.New()
	f.consents = append(f.consents, c)
	return true, nil
}

func (f *fakeRepo) RevokeConsent(_ context.Context, userID uuid.UUID, slug domain.Slug, at time.Time) (bool, error) {
	did := false
	for i := range f.consents {
		c := &f.consents[i]
		if c.UserID == userID && c.DocumentSlug == slug && c.RevokedAt == nil {
			c.RevokedAt = &at
			did = true
		}
	}
	return did, nil
}

func (f *fakeRepo) ListConsents(_ context.Context, userID *uuid.UUID, limit int) ([]domain.Consent, error) {
	var out []domain.Consent
	for _, c := range f.consents {
		if userID == nil || c.UserID == *userID {
			out = append(out, c)
		}
	}
	return out, nil
}

var t0 = time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)

func setup() (*Service, *fakeRepo, *events.MemoryPublisher) {
	repo := &fakeRepo{}
	for _, s := range []domain.Slug{domain.SlugPrivacyPolicy, domain.SlugDataProcessing, domain.SlugTermsOfUse, domain.SlugCookies, domain.SlugCommunications} {
		repo.docs = append(repo.docs, domain.Document{Slug: s, Version: 1, Title: string(s), BodyMarkdown: "x", Required: s.DefaultRequired(), EffectiveFrom: t0.Add(-time.Hour)})
	}
	pub := events.NewMemoryPublisher()
	return NewService(repo, &database.FakeTxManager{}, pub, clock.NewFixed(t0)), repo, pub
}

func requiredRefs() []domain.Ref {
	return []domain.Ref{{Slug: domain.SlugPrivacyPolicy, Version: 1}, {Slug: domain.SlugDataProcessing, Version: 1}, {Slug: domain.SlugTermsOfUse, Version: 1}}
}

func assertCode(t *testing.T, err error, kind apperror.Kind, code string) {
	t.Helper()
	if apperror.KindOf(err) != kind || apperror.CodeOf(err) != code {
		t.Fatalf("got kind=%s code=%q (%v), want kind=%s code=%q", apperror.KindOf(err), apperror.CodeOf(err), err, kind, code)
	}
}

func TestValidateRequiredMatrix(t *testing.T) {
	svc, _, _ := setup()
	ctx := context.Background()
	all := requiredRefs()
	if err := svc.ValidateRequired(ctx, all); err != nil {
		t.Fatalf("all present: %v", err)
	}
	extra := append(append([]domain.Ref{}, all...), domain.Ref{Slug: domain.SlugCommunications, Version: 1})
	if err := svc.ValidateRequired(ctx, extra); err != nil {
		t.Fatalf("optional extra: %v", err)
	}
	err := svc.ValidateRequired(ctx, all[:2])
	assertCode(t, err, apperror.KindValidation, "legal.consent_required")
	if d := apperror.DetailsOf(err); len(d) != 1 || d[0].Field != "terms_of_use" {
		t.Fatalf("details = %+v", d)
	}
	wrong := []domain.Ref{{Slug: domain.SlugPrivacyPolicy, Version: 9}, all[1], all[2]}
	err = svc.ValidateRequired(ctx, wrong)
	assertCode(t, err, apperror.KindValidation, "legal.consent_required")
	if d := apperror.DetailsOf(err); len(d) != 1 || d[0].Field != "privacy_policy" {
		t.Fatalf("details = %+v", d)
	}
	assertCode(t, svc.ValidateRequired(ctx, nil), apperror.KindValidation, "legal.consent_required")
}

func TestRecordIsIdempotentAndEmitsEventOnce(t *testing.T) {
	svc, repo, pub := setup()
	ctx := context.Background()
	user := uuid.New()
	in := port.RecordInput{Accepted: requiredRefs(), Source: domain.SourceRegister, IP: "198.51.100.23", UserAgent: "ua"}
	st, err := svc.Record(ctx, user, in)
	if err != nil || len(st.Pending) != 0 || len(st.Accepted) != 3 {
		t.Fatalf("first record: %+v %v", st, err)
	}
	if _, err := svc.Record(ctx, user, in); err != nil {
		t.Fatal(err)
	}
	if len(repo.consents) != 3 {
		t.Fatalf("rows = %d, want 3", len(repo.consents))
	}
	if ev := pub.Events(); len(ev) != 1 || ev[0].Type != EventConsentRecorded {
		t.Fatalf("events = %+v", ev)
	}
	if repo.consents[0].IP != "198.51.100.0" {
		t.Fatalf("ip stored as %q, want anonymized", repo.consents[0].IP)
	}
}

func TestRecordValidation(t *testing.T) {
	svc, _, _ := setup()
	ctx, user := context.Background(), uuid.New()
	_, err := svc.Record(ctx, user, port.RecordInput{Source: domain.SourceProfile})
	assertCode(t, err, apperror.KindValidation, "legal.consent_empty")
	_, err = svc.Record(ctx, user, port.RecordInput{Accepted: requiredRefs(), Source: "nope"})
	assertCode(t, err, apperror.KindValidation, "legal.source_invalid")
	_, err = svc.Record(ctx, user, port.RecordInput{Accepted: []domain.Ref{{Slug: "x", Version: 1}}, Source: domain.SourceProfile})
	assertCode(t, err, apperror.KindValidation, "legal.slug_invalid")
	_, err = svc.Record(ctx, user, port.RecordInput{Accepted: []domain.Ref{{Slug: domain.SlugCookies, Version: 3}}, Source: domain.SourceProfile})
	assertCode(t, err, apperror.KindValidation, "legal.version_invalid")
}

func TestPendingAfterPublishingNewVersion(t *testing.T) {
	svc, _, _ := setup()
	ctx, user, admin := context.Background(), uuid.New(), uuid.New()
	if _, err := svc.Record(ctx, user, port.RecordInput{Accepted: requiredRefs(), Source: domain.SourceRegister}); err != nil {
		t.Fatal(err)
	}
	if p, _ := svc.Pending(ctx, user); len(p) != 0 {
		t.Fatalf("pending = %+v", p)
	}
	doc, err := svc.Publish(ctx, admin, port.PublishInput{Slug: domain.SlugPrivacyPolicy, Title: "Privacidad v2", BodyMarkdown: "nuevo"})
	if err != nil || doc.Version != 2 || !doc.Required {
		t.Fatalf("publish: %+v %v", doc, err)
	}
	p, _ := svc.Pending(ctx, user)
	if len(p) != 1 || p[0].Slug != domain.SlugPrivacyPolicy || p[0].Version != 2 || p[0].Title != "Privacidad v2" {
		t.Fatalf("pending = %+v", p)
	}
	// Registration now needs version 2.
	assertCode(t, svc.ValidateRequired(ctx, requiredRefs()), apperror.KindValidation, "legal.consent_required")
	// Accepting version 2 clears the pending list.
	st, err := svc.Record(ctx, user, port.RecordInput{Accepted: []domain.Ref{{Slug: domain.SlugPrivacyPolicy, Version: 2}}, Source: domain.SourceLoginReprompt})
	if err != nil || len(st.Pending) != 0 {
		t.Fatalf("after accept: %+v %v", st, err)
	}
}

func TestPendingForUserWithoutConsents(t *testing.T) {
	svc, _, _ := setup()
	p, err := svc.Pending(context.Background(), uuid.New())
	if err != nil || len(p) != 3 {
		t.Fatalf("existing users start with every required document pending: %+v %v", p, err)
	}
}

func TestRevokeRules(t *testing.T) {
	svc, _, pub := setup()
	ctx, user := context.Background(), uuid.New()
	assertCode(t, svc.Revoke(ctx, user, domain.SlugPrivacyPolicy), apperror.KindConflict, "legal.consent_not_revocable")
	assertCode(t, svc.Revoke(ctx, user, "zzz"), apperror.KindValidation, "legal.slug_invalid")
	assertCode(t, svc.Revoke(ctx, user, domain.SlugCommunications), apperror.KindNotFound, "legal.consent_not_found")
	if _, err := svc.Record(ctx, user, port.RecordInput{Accepted: []domain.Ref{{Slug: domain.SlugCommunications, Version: 1}}, Source: domain.SourceProfile}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Revoke(ctx, user, domain.SlugCommunications); err != nil {
		t.Fatal(err)
	}
	st, _ := svc.Status(ctx, user)
	if len(st.Accepted) != 0 {
		t.Fatalf("accepted after revoke = %+v", st.Accepted)
	}
	// Re-accepting after a revocation works.
	if _, err := svc.Record(ctx, user, port.RecordInput{Accepted: []domain.Ref{{Slug: domain.SlugCommunications, Version: 1}}, Source: domain.SourceProfile}); err != nil {
		t.Fatal(err)
	}
	if n := len(pub.Events()); n != 3 {
		t.Fatalf("events = %d, want recorded+revoked+recorded", n)
	}
}

func TestGetDocument(t *testing.T) {
	svc, _, _ := setup()
	ctx := context.Background()
	if d, err := svc.GetDocument(ctx, domain.SlugCookies, 0); err != nil || d.Version != 1 {
		t.Fatalf("current: %+v %v", d, err)
	}
	_, err := svc.GetDocument(ctx, domain.SlugCookies, 7)
	assertCode(t, err, apperror.KindNotFound, "legal.document_not_found")
	_, err = svc.GetDocument(ctx, "zzz", 0)
	assertCode(t, err, apperror.KindNotFound, "legal.document_not_found")
	_, err = svc.GetDocument(ctx, domain.SlugCookies, -1)
	assertCode(t, err, apperror.KindValidation, "legal.version_invalid")
}

func TestFutureDocumentIsNotCurrent(t *testing.T) {
	svc, repo, _ := setup()
	repo.docs = append(repo.docs, domain.Document{Slug: domain.SlugPrivacyPolicy, Version: 2, Title: "futuro", Required: true, EffectiveFrom: t0.Add(time.Hour)})
	docs, _ := svc.ListCurrent(context.Background())
	for _, d := range docs {
		if d.Slug == domain.SlugPrivacyPolicy && d.Version != 1 {
			t.Fatalf("version %d is not in effect yet", d.Version)
		}
	}
}
