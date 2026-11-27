package http

import (
	"time"

	"github.com/jhonsferg/corelog/internal/legal/domain"
)

type documentSummary struct {
	Slug          string    `json:"slug"`
	Title         string    `json:"title"`
	Version       int       `json:"version"`
	Summary       string    `json:"summary"`
	Required      bool      `json:"required"`
	EffectiveFrom time.Time `json:"effective_from"`
}

type documentDetail struct {
	documentSummary
	BodyMarkdown string    `json:"body_markdown"`
	PublishedAt  time.Time `json:"published_at"`
	CreatedBy    *string   `json:"created_by"`
}

func toSummary(d domain.Document) documentSummary {
	return documentSummary{Slug: string(d.Slug), Title: d.Title, Version: d.Version, Summary: d.Summary, Required: d.Required, EffectiveFrom: d.EffectiveFrom}
}

func toDetail(d domain.Document) documentDetail {
	out := documentDetail{documentSummary: toSummary(d), BodyMarkdown: d.BodyMarkdown, PublishedAt: d.PublishedAt}
	if d.CreatedBy != nil {
		s := d.CreatedBy.String()
		out.CreatedBy = &s
	}
	return out
}

type consentRef struct {
	Slug    string `json:"slug"`
	Version int    `json:"version"`
}

type recordRequest struct {
	Accepted []consentRef `json:"accepted"`
	Source   string       `json:"source"`
}

type publishRequest struct {
	Slug         string `json:"slug"`
	Title        string `json:"title"`
	Summary      string `json:"summary"`
	BodyMarkdown string `json:"body_markdown"`
	Required     *bool  `json:"required"`
}

type acceptedResponse struct {
	Slug       string    `json:"slug"`
	Version    int       `json:"version"`
	AcceptedAt time.Time `json:"accepted_at"`
}

type pendingResponse struct {
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	Version int    `json:"version"`
}

type statusResponse struct {
	Accepted []acceptedResponse `json:"accepted"`
	Pending  []pendingResponse  `json:"pending"`
}

func toStatus(st domain.Status) statusResponse {
	out := statusResponse{Accepted: make([]acceptedResponse, 0, len(st.Accepted)), Pending: make([]pendingResponse, 0, len(st.Pending))}
	for _, a := range st.Accepted {
		out.Accepted = append(out.Accepted, acceptedResponse{Slug: string(a.Slug), Version: a.Version, AcceptedAt: a.AcceptedAt})
	}
	for _, p := range st.Pending {
		out.Pending = append(out.Pending, pendingResponse{Slug: string(p.Slug), Title: p.Title, Version: p.Version})
	}
	return out
}

type consentResponse struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	Slug       string     `json:"slug"`
	Version    int        `json:"version"`
	AcceptedAt time.Time  `json:"accepted_at"`
	IP         *string    `json:"ip"`
	UserAgent  *string    `json:"user_agent"`
	Source     string     `json:"source"`
	RevokedAt  *time.Time `json:"revoked_at"`
}

func toConsent(c domain.Consent) consentResponse {
	out := consentResponse{
		ID: c.ID.String(), UserID: c.UserID.String(), Slug: string(c.DocumentSlug), Version: c.DocumentVersion,
		AcceptedAt: c.AcceptedAt, Source: string(c.Source), RevokedAt: c.RevokedAt,
	}
	if c.IP != "" {
		ip := c.IP
		out.IP = &ip
	}
	if c.UserAgent != "" {
		ua := c.UserAgent
		out.UserAgent = &ua
	}
	return out
}
