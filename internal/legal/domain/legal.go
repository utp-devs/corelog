// Package domain holds the legal document and consent model: versioned documents and the record of the
// consent each user gave to them.
package domain

import (
	"net/netip"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// Slug identifies a legal document kind.
type Slug string

const (
	SlugPrivacyPolicy  Slug = "privacy_policy"
	SlugDataProcessing Slug = "data_processing"
	SlugTermsOfUse     Slug = "terms_of_use"
	SlugCookies        Slug = "cookies"
	SlugCommunications Slug = "communications"
)

// AllSlugs lists every valid slug.
var AllSlugs = []Slug{SlugPrivacyPolicy, SlugDataProcessing, SlugTermsOfUse, SlugCookies, SlugCommunications}

// Valid reports whether s is a known slug.
func (s Slug) Valid() bool {
	for _, k := range AllSlugs {
		if k == s {
			return true
		}
	}
	return false
}

// DefaultRequired is the required flag a new document of this slug gets when the publisher does not choose.
func (s Slug) DefaultRequired() bool {
	return s == SlugPrivacyPolicy || s == SlugDataProcessing || s == SlugTermsOfUse
}

// Source says where a consent was given.
type Source string

const (
	SourceRegister      Source = "register"
	SourceInvite        Source = "invite"
	SourceLoginReprompt Source = "login_reprompt"
	SourceProfile       Source = "profile"
)

// Valid reports whether s is a known source.
func (s Source) Valid() bool {
	switch s {
	case SourceRegister, SourceInvite, SourceLoginReprompt, SourceProfile:
		return true
	}
	return false
}

const (
	MaxTitleLength   = 200
	MaxSummaryLength = 1000
	MaxBodyLength    = 200_000
	MaxUserAgent     = 255
)

// Document is one published version of a legal document.
type Document struct {
	ID            uuid.UUID
	Slug          Slug
	Version       int
	Title         string
	BodyMarkdown  string
	Summary       string
	Required      bool
	EffectiveFrom time.Time
	PublishedAt   time.Time
	CreatedBy     *uuid.UUID
}

// Ref names a document version.
type Ref struct {
	Slug    Slug
	Version int
}

// Consent is one acceptance row.
type Consent struct {
	ID              uuid.UUID
	UserID          uuid.UUID
	DocumentSlug    Slug
	DocumentVersion int
	AcceptedAt      time.Time
	IP              string
	UserAgent       string
	Source          Source
	RevokedAt       *time.Time
}

// Pending is a required document version the user has not accepted yet.
type Pending struct {
	Slug    Slug
	Title   string
	Version int
}

// Accepted is the active consent of a user for a slug.
type Accepted struct {
	Slug       Slug
	Version    int
	AcceptedAt time.Time
}

// Status is the consent situation of one user against the current documents.
type Status struct {
	Accepted []Accepted
	Pending  []Pending
}

var (
	ErrDocumentNotFound = apperror.WithCode(apperror.KindNotFound, "legal.document_not_found", "El documento legal no existe")
	ErrSlugInvalid      = apperror.WithCode(apperror.KindValidation, "legal.slug_invalid", "El tipo de documento legal no es valido")
	ErrVersionInvalid   = apperror.WithCode(apperror.KindValidation, "legal.version_invalid", "La version del documento no es valida")
	ErrSourceInvalid    = apperror.WithCode(apperror.KindValidation, "legal.source_invalid", "El origen del consentimiento no es valido")
	ErrConsentEmpty     = apperror.WithCode(apperror.KindValidation, "legal.consent_empty", "Debe indicar al menos un documento aceptado")
	ErrNotRevocable     = apperror.WithCode(apperror.KindConflict, "legal.consent_not_revocable", "Solo se puede revocar el consentimiento opcional de comunicaciones")
	ErrNoActiveConsent  = apperror.WithCode(apperror.KindNotFound, "legal.consent_not_found", "No hay un consentimiento activo para este documento")
	ErrPublishConflict  = apperror.WithCode(apperror.KindConflict, "legal.publish_conflict", "Otra publicacion se realizo al mismo tiempo; vuelva a intentarlo")
	ErrCommsRequired    = apperror.WithCode(apperror.KindValidation, "legal.communications_optional", "El consentimiento de comunicaciones no puede ser obligatorio")
)

// ErrConsentRequired lists the required documents that were not accepted at their current version.
func ErrConsentRequired(missing []Slug) *apperror.Error {
	err := apperror.WithCode(apperror.KindValidation, "legal.consent_required", "Debe aceptar los documentos legales vigentes para continuar")
	for _, s := range missing {
		err.WithDetails(apperror.FieldError{Field: string(s), Message: "Falta aceptar la version vigente"})
	}
	return err
}

// ErrConsentPending is returned in block mode while the user has pending required consents.
func ErrConsentPending(missing []Slug) *apperror.Error {
	err := apperror.WithCode(apperror.KindForbidden, "legal.consent_pending", "Debe aceptar los documentos legales vigentes antes de continuar")
	for _, s := range missing {
		err.WithDetails(apperror.FieldError{Field: string(s), Message: "Falta aceptar la version vigente"})
	}
	return err
}

// NewDraft validates the content of a new document version. The version and the effective date are assigned
// when it is published.
func NewDraft(slug Slug, title, summary, body string, required *bool, createdBy uuid.UUID) (*Document, error) {
	if !slug.Valid() {
		return nil, ErrSlugInvalid
	}
	title, summary = strings.TrimSpace(title), strings.TrimSpace(summary)
	fields := []apperror.FieldError{}
	if title == "" || utf8.RuneCountInString(title) > MaxTitleLength {
		fields = append(fields, apperror.FieldError{Field: "title", Message: "El titulo es obligatorio y admite hasta 200 caracteres"})
	}
	if utf8.RuneCountInString(summary) > MaxSummaryLength {
		fields = append(fields, apperror.FieldError{Field: "summary", Message: "El resumen admite hasta 1000 caracteres"})
	}
	if strings.TrimSpace(body) == "" || utf8.RuneCountInString(body) > MaxBodyLength {
		fields = append(fields, apperror.FieldError{Field: "body_markdown", Message: "El contenido es obligatorio y admite hasta 200000 caracteres"})
	}
	if len(fields) > 0 {
		return nil, apperror.WithCode(apperror.KindValidation, "validation.failed", "Datos del documento no validos").WithDetails(fields...)
	}
	req := slug.DefaultRequired()
	if required != nil {
		req = *required
	}
	if slug == SlugCommunications && req {
		return nil, ErrCommsRequired
	}
	d := &Document{Slug: slug, Title: title, Summary: summary, BodyMarkdown: body, Required: req}
	if createdBy != uuid.Nil {
		id := createdBy
		d.CreatedBy = &id
	}
	return d, nil
}

// NewConsent builds an acceptance row with the IP anonymized and the user agent truncated.
func NewConsent(userID uuid.UUID, ref Ref, source Source, ip, userAgent string, at time.Time) Consent {
	return Consent{
		UserID: userID, DocumentSlug: ref.Slug, DocumentVersion: ref.Version, AcceptedAt: at,
		IP: AnonymizeIP(ip), UserAgent: TruncateUserAgent(userAgent), Source: source,
	}
}

// AnonymizeIP zeroes the last octet of an IPv4 address and keeps only the /64 prefix of an IPv6 address.
// It returns "" for empty or unparsable input.
func AnonymizeIP(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	addr, err := netip.ParseAddr(raw)
	if err != nil {
		ap, perr := netip.ParseAddrPort(raw)
		if perr != nil {
			return ""
		}
		addr = ap.Addr()
	}
	addr = addr.Unmap().WithZone("")
	if addr.Is4() {
		b := addr.As4()
		b[3] = 0
		return netip.AddrFrom4(b).String()
	}
	b := addr.As16()
	for i := 8; i < 16; i++ {
		b[i] = 0
	}
	return netip.AddrFrom16(b).String()
}

// TruncateUserAgent limits the user agent to MaxUserAgent characters.
func TruncateUserAgent(ua string) string {
	r := []rune(strings.TrimSpace(ua))
	if len(r) > MaxUserAgent {
		r = r[:MaxUserAgent]
	}
	return string(r)
}

// ComputeStatus compares the current documents against the active consents of a user.
func ComputeStatus(current []Document, active []Accepted) Status {
	byslug := map[Slug]Accepted{}
	for _, a := range active {
		if prev, ok := byslug[a.Slug]; !ok || a.Version > prev.Version {
			byslug[a.Slug] = a
		}
	}
	st := Status{Accepted: []Accepted{}, Pending: []Pending{}}
	for _, d := range current {
		a, ok := byslug[d.Slug]
		if ok {
			st.Accepted = append(st.Accepted, a)
		}
		if d.Required && (!ok || a.Version < d.Version) {
			st.Pending = append(st.Pending, Pending{Slug: d.Slug, Title: d.Title, Version: d.Version})
		}
	}
	return st
}

// MissingRequired returns the slugs of the required current documents that refs do not cover at their
// current version.
func MissingRequired(current []Document, refs []Ref) []Slug {
	given := map[Slug]int{}
	for _, r := range refs {
		given[r.Slug] = r.Version
	}
	var missing []Slug
	for _, d := range current {
		if !d.Required {
			continue
		}
		if v, ok := given[d.Slug]; !ok || v != d.Version {
			missing = append(missing, d.Slug)
		}
	}
	return missing
}
