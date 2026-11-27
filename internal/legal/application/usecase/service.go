// Package usecase orchestrates the legal documents and consents.
package usecase

import (
	"context"
	"sort"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/legal/application/port"
	"github.com/jhonsferg/corelog/internal/legal/domain"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/events"
)

// Event types published by this hexagon.
const (
	EventConsentRecorded   = "legal.consent_recorded"
	EventConsentRevoked    = "legal.consent_revoked"
	EventDocumentPublished = "legal.document_published"
)

const maxAuditLimit = 500

// Service implements port.Service.
type Service struct {
	repo   port.Repository
	tx     database.TxManager
	events events.Publisher
	clock  clock.Clock
}

var _ port.Service = (*Service)(nil)

// NewService wires the repository, the transaction manager, the event publisher and the clock. tx and
// publisher may be nil in tests.
func NewService(repo port.Repository, tx database.TxManager, pub events.Publisher, clk clock.Clock) *Service {
	return &Service{repo: repo, tx: tx, events: pub, clock: clk}
}

func (s *Service) inTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if s.tx == nil {
		return fn(ctx)
	}
	return s.tx.InTx(ctx, fn)
}

func (s *Service) publish(ctx context.Context, e events.Event) error {
	if s.events == nil {
		return nil
	}
	e.CreatedAt = s.clock.Now()
	return s.events.Publish(ctx, e)
}

func (s *Service) ListCurrent(ctx context.Context) ([]domain.Document, error) {
	docs, err := s.repo.ListCurrent(ctx, s.clock.Now())
	if err != nil {
		return nil, err
	}
	sortDocs(docs)
	return docs, nil
}

func sortDocs(docs []domain.Document) {
	order := map[domain.Slug]int{}
	for i, s := range domain.AllSlugs {
		order[s] = i
	}
	sort.SliceStable(docs, func(i, j int) bool { return order[docs[i].Slug] < order[docs[j].Slug] })
}

func (s *Service) GetDocument(ctx context.Context, slug domain.Slug, version int) (*domain.Document, error) {
	if !slug.Valid() {
		return nil, domain.ErrDocumentNotFound
	}
	if version < 0 {
		return nil, domain.ErrVersionInvalid
	}
	if version == 0 {
		return s.repo.GetCurrent(ctx, slug, s.clock.Now())
	}
	return s.repo.GetVersion(ctx, slug, version)
}

func (s *Service) Status(ctx context.Context, userID uuid.UUID) (domain.Status, error) {
	current, err := s.repo.ListCurrent(ctx, s.clock.Now())
	if err != nil {
		return domain.Status{}, err
	}
	active, err := s.repo.ActiveConsents(ctx, userID)
	if err != nil {
		return domain.Status{}, err
	}
	sortDocs(current)
	return domain.ComputeStatus(current, active), nil
}

func (s *Service) Pending(ctx context.Context, userID uuid.UUID) ([]domain.Pending, error) {
	st, err := s.Status(ctx, userID)
	if err != nil {
		return nil, err
	}
	return st.Pending, nil
}

func (s *Service) ValidateRequired(ctx context.Context, refs []domain.Ref) error {
	current, err := s.repo.ListCurrent(ctx, s.clock.Now())
	if err != nil {
		return err
	}
	sortDocs(current)
	if missing := domain.MissingRequired(current, refs); len(missing) > 0 {
		return domain.ErrConsentRequired(missing)
	}
	return nil
}

func (s *Service) Record(ctx context.Context, userID uuid.UUID, in port.RecordInput) (domain.Status, error) {
	if !in.Source.Valid() {
		return domain.Status{}, domain.ErrSourceInvalid
	}
	if len(in.Accepted) == 0 {
		return domain.Status{}, domain.ErrConsentEmpty
	}
	now := s.clock.Now()
	current, err := s.repo.ListCurrent(ctx, now)
	if err != nil {
		return domain.Status{}, err
	}
	currentVersion := map[domain.Slug]int{}
	for _, d := range current {
		currentVersion[d.Slug] = d.Version
	}
	for _, ref := range in.Accepted {
		if !ref.Slug.Valid() {
			return domain.Status{}, domain.ErrSlugInvalid
		}
		if v, ok := currentVersion[ref.Slug]; !ok || v != ref.Version {
			return domain.Status{}, domain.ErrVersionInvalid
		}
	}

	err = s.inTx(ctx, func(ctx context.Context) error {
		var recorded []map[string]any
		for _, ref := range in.Accepted {
			inserted, err := s.repo.SaveConsent(ctx, domain.NewConsent(userID, ref, in.Source, in.IP, in.UserAgent, now))
			if err != nil {
				return err
			}
			if inserted {
				recorded = append(recorded, map[string]any{"slug": string(ref.Slug), "version": ref.Version})
			}
		}
		if len(recorded) == 0 {
			return nil
		}
		uid := userID
		return s.publish(ctx, events.Event{
			ActorID: &uid, EntityID: &uid, Type: EventConsentRecorded, EntityType: "user_consent",
			Payload: map[string]any{"source": string(in.Source), "documents": recorded},
		})
	})
	if err != nil {
		return domain.Status{}, err
	}
	return s.Status(ctx, userID)
}

func (s *Service) Revoke(ctx context.Context, userID uuid.UUID, slug domain.Slug) error {
	if !slug.Valid() {
		return domain.ErrSlugInvalid
	}
	if slug != domain.SlugCommunications {
		return domain.ErrNotRevocable
	}
	return s.inTx(ctx, func(ctx context.Context) error {
		revoked, err := s.repo.RevokeConsent(ctx, userID, slug, s.clock.Now())
		if err != nil {
			return err
		}
		if !revoked {
			return domain.ErrNoActiveConsent
		}
		uid := userID
		return s.publish(ctx, events.Event{
			ActorID: &uid, EntityID: &uid, Type: EventConsentRevoked, EntityType: "user_consent",
			Payload: map[string]any{"slug": string(slug)},
		})
	})
}

func (s *Service) AdminList(ctx context.Context) ([]domain.Document, error) {
	return s.repo.ListAll(ctx)
}

func (s *Service) Publish(ctx context.Context, actorID uuid.UUID, in port.PublishInput) (*domain.Document, error) {
	doc, err := domain.NewDraft(in.Slug, in.Title, in.Summary, in.BodyMarkdown, in.Required, actorID)
	if err != nil {
		return nil, err
	}
	err = s.inTx(ctx, func(ctx context.Context) error {
		if err := s.repo.Publish(ctx, doc, s.clock.Now()); err != nil {
			return err
		}
		actor := actorID
		return s.publish(ctx, events.Event{
			ActorID: &actor, EntityID: &doc.ID, Type: EventDocumentPublished, EntityType: "legal_document",
			Payload: map[string]any{"slug": string(doc.Slug), "version": doc.Version, "required": doc.Required},
		})
	})
	if err != nil {
		return nil, err
	}
	return doc, nil
}

func (s *Service) Audit(ctx context.Context, userID *uuid.UUID, limit int) ([]domain.Consent, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > maxAuditLimit {
		limit = maxAuditLimit
	}
	return s.repo.ListConsents(ctx, userID, limit)
}
