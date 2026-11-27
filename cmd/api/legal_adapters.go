package main

import (
	"context"

	"github.com/google/uuid"

	legalport "github.com/jhonsferg/corelog/internal/legal/application/port"
	legaldomain "github.com/jhonsferg/corelog/internal/legal/domain"
	organizationhttp "github.com/jhonsferg/corelog/internal/organization/adapter/in/http"
	userport "github.com/jhonsferg/corelog/internal/user/application/port"
)

// userConsentGate adapts the legal service to the consent port of the user module.
type userConsentGate struct{ legal legalport.Service }

var _ userport.ConsentGate = userConsentGate{}

func legalRefs(in []userport.ConsentRef) []legaldomain.Ref {
	out := make([]legaldomain.Ref, 0, len(in))
	for _, r := range in {
		out = append(out, legaldomain.Ref{Slug: legaldomain.Slug(r.Slug), Version: r.Version})
	}
	return out
}

func (g userConsentGate) Validate(ctx context.Context, refs []userport.ConsentRef) error {
	return g.legal.ValidateRequired(ctx, legalRefs(refs))
}

func (g userConsentGate) Record(ctx context.Context, userID uuid.UUID, refs []userport.ConsentRef, meta userport.ConsentMeta) error {
	_, err := g.legal.Record(ctx, userID, legalport.RecordInput{
		Accepted: legalRefs(refs), Source: legaldomain.Source(meta.Source), IP: meta.IP, UserAgent: meta.UserAgent,
	})
	return err
}

func (g userConsentGate) Pending(ctx context.Context, userID uuid.UUID) ([]userport.PendingConsent, error) {
	pending, err := g.legal.Pending(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]userport.PendingConsent, 0, len(pending))
	for _, p := range pending {
		out = append(out, userport.PendingConsent{Slug: string(p.Slug), Title: p.Title, Version: p.Version})
	}
	return out, nil
}

// invitationConsentGate adapts the legal service to the consent port of the invitation acceptance.
type invitationConsentGate struct{ legal legalport.Service }

var _ organizationhttp.ConsentGate = invitationConsentGate{}

// Require records the consents sent with the invitation, then fails with legal.consent_required unless the
// user has no required document pending.
func (g invitationConsentGate) Require(ctx context.Context, userID uuid.UUID, refs []organizationhttp.ConsentRef, source, ip, userAgent string) error {
	if len(refs) > 0 {
		accepted := make([]legaldomain.Ref, 0, len(refs))
		for _, r := range refs {
			accepted = append(accepted, legaldomain.Ref{Slug: legaldomain.Slug(r.Slug), Version: r.Version})
		}
		if _, err := g.legal.Record(ctx, userID, legalport.RecordInput{
			Accepted: accepted, Source: legaldomain.Source(source), IP: ip, UserAgent: userAgent,
		}); err != nil {
			return err
		}
	}
	pending, err := g.legal.Pending(ctx, userID)
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		return nil
	}
	missing := make([]legaldomain.Slug, 0, len(pending))
	for _, p := range pending {
		missing = append(missing, p.Slug)
	}
	return legaldomain.ErrConsentRequired(missing)
}
