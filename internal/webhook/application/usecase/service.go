// Package usecase implements webhook management, event capture and delivery.
package usecase

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/events"
	"github.com/jhonsferg/corelog/internal/webhook/application/port"
	"github.com/jhonsferg/corelog/internal/webhook/domain"
)

// Deps are the collaborators of the service.
type Deps struct {
	Repo      port.Repository
	Access    port.OrgAccess
	Box       port.SecretBox
	Sender    port.Sender
	Describer port.EventDescriber
	Matcher   port.IssueMatcher // optional: without it filters are not evaluated
	Tx        database.TxManager
	Publisher events.Publisher // optional
	Clock     clock.Clock
	// AllowPrivate lets endpoints use private addresses. It is meant for development only.
	AllowPrivate bool
}

// Service implements port.Service, the event consumer and the delivery worker.
type Service struct{ Deps }

var _ port.Service = (*Service)(nil)

// NewService returns the service.
func NewService(d Deps) *Service { return &Service{Deps: d} }

func (s *Service) audit(ctx context.Context, kind string, actor, orgID uuid.UUID, w domain.Webhook) error {
	if s.Publisher == nil {
		return nil
	}
	id := w.ID
	return s.Publisher.Publish(ctx, events.Event{
		OrgID: &orgID, ProjectID: w.ProjectID, ActorID: &actor, EntityID: &id, Type: kind, EntityType: "webhook",
		Payload: map[string]any{"name": w.Name, "url": w.URL}, CreatedAt: s.Clock.Now(),
	})
}

func (s *Service) checkFilter(ctx context.Context, q *string) (*string, error) {
	q, err := domain.ValidateFilter(q)
	if err != nil || q == nil || s.Matcher == nil {
		return q, err
	}
	if err := s.Matcher.Validate(ctx, *q); err != nil {
		return nil, apperror.WithCode(apperror.KindValidation, "webhook.invalid_filter", "El filtro no es una consulta valida").
			WithDetails(apperror.FieldError{Field: "filter_query", Message: "consulta no valida"})
	}
	return q, nil
}

// Create stores a webhook and returns its secret once.
func (s *Service) Create(ctx context.Context, actorID, orgID uuid.UUID, in port.CreateInput) (domain.Webhook, string, error) {
	if err := s.Access.RequireAdmin(ctx, actorID, orgID); err != nil {
		return domain.Webhook{}, "", err
	}
	name, err := domain.ValidateName(in.Name)
	if err != nil {
		return domain.Webhook{}, "", err
	}
	target, err := domain.ValidateURL(in.URL, s.AllowPrivate)
	if err != nil {
		return domain.Webhook{}, "", err
	}
	evs, err := domain.ValidateEvents(in.Events)
	if err != nil {
		return domain.Webhook{}, "", err
	}
	filter, err := s.checkFilter(ctx, in.FilterQuery)
	if err != nil {
		return domain.Webhook{}, "", err
	}
	if in.ProjectID != nil {
		ok, err := s.Repo.ProjectInOrg(ctx, orgID, *in.ProjectID)
		if err != nil {
			return domain.Webhook{}, "", err
		}
		if !ok {
			return domain.Webhook{}, "", domain.ErrProjectNotInOrg
		}
	}
	secret, err := domain.NewSecret()
	if err != nil {
		return domain.Webhook{}, "", apperror.Wrap(apperror.KindInternal, "no se pudo generar el secreto", err)
	}
	sealed, err := s.Box.Seal(secret)
	if err != nil {
		return domain.Webhook{}, "", apperror.Wrap(apperror.KindInternal, "no se pudo proteger el secreto", err)
	}
	now := s.Clock.Now()
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	stored := port.Stored{SecretEncrypted: sealed, Webhook: domain.Webhook{
		OrgID: orgID, ProjectID: in.ProjectID, Name: name, URL: target, Events: evs, FilterQuery: filter,
		Enabled: enabled, CreatedBy: &actorID, CreatedAt: now, UpdatedAt: now,
	}}
	err = s.Tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.Repo.Create(ctx, &stored); err != nil {
			return err
		}
		return s.audit(ctx, "webhook.created", actorID, orgID, stored.Webhook)
	})
	if err != nil {
		return domain.Webhook{}, "", err
	}
	return stored.Webhook, secret, nil
}

// List returns the webhooks of the organization.
func (s *Service) List(ctx context.Context, actorID, orgID uuid.UUID, projectID *uuid.UUID) ([]domain.Webhook, error) {
	if err := s.Access.RequireAdmin(ctx, actorID, orgID); err != nil {
		return nil, err
	}
	return s.Repo.List(ctx, orgID, projectID)
}

// Get returns one webhook.
func (s *Service) Get(ctx context.Context, actorID, orgID, id uuid.UUID) (domain.Webhook, error) {
	if err := s.Access.RequireAdmin(ctx, actorID, orgID); err != nil {
		return domain.Webhook{}, err
	}
	st, err := s.Repo.Get(ctx, orgID, id)
	return st.Webhook, err
}

// Update changes a webhook.
func (s *Service) Update(ctx context.Context, actorID, orgID, id uuid.UUID, in port.UpdateInput) (domain.Webhook, error) {
	if err := s.Access.RequireAdmin(ctx, actorID, orgID); err != nil {
		return domain.Webhook{}, err
	}
	st, err := s.Repo.Get(ctx, orgID, id)
	if err != nil {
		return domain.Webhook{}, err
	}
	w := st.Webhook
	if in.Name != nil {
		if w.Name, err = domain.ValidateName(*in.Name); err != nil {
			return domain.Webhook{}, err
		}
	}
	if in.URL != nil {
		if w.URL, err = domain.ValidateURL(*in.URL, s.AllowPrivate); err != nil {
			return domain.Webhook{}, err
		}
	}
	if in.Events != nil {
		if w.Events, err = domain.ValidateEvents(in.Events); err != nil {
			return domain.Webhook{}, err
		}
	}
	if in.FilterQuery != nil {
		// An empty text removes the filter.
		if w.FilterQuery, err = s.checkFilter(ctx, in.FilterQuery); err != nil {
			return domain.Webhook{}, err
		}
	}
	if in.Enabled != nil {
		w.Enabled = *in.Enabled
	}
	w.UpdatedAt = s.Clock.Now()
	err = s.Tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.Repo.Update(ctx, w); err != nil {
			return err
		}
		return s.audit(ctx, "webhook.updated", actorID, orgID, w)
	})
	return w, err
}

// Delete removes a webhook and its deliveries.
func (s *Service) Delete(ctx context.Context, actorID, orgID, id uuid.UUID) error {
	if err := s.Access.RequireAdmin(ctx, actorID, orgID); err != nil {
		return err
	}
	st, err := s.Repo.Get(ctx, orgID, id)
	if err != nil {
		return err
	}
	return s.Tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.Repo.Delete(ctx, orgID, id); err != nil {
			return err
		}
		return s.audit(ctx, "webhook.deleted", actorID, orgID, st.Webhook)
	})
}

// RotateSecret replaces the signing secret and returns the new one once.
func (s *Service) RotateSecret(ctx context.Context, actorID, orgID, id uuid.UUID) (string, error) {
	if err := s.Access.RequireAdmin(ctx, actorID, orgID); err != nil {
		return "", err
	}
	st, err := s.Repo.Get(ctx, orgID, id)
	if err != nil {
		return "", err
	}
	secret, err := domain.NewSecret()
	if err != nil {
		return "", apperror.Wrap(apperror.KindInternal, "no se pudo generar el secreto", err)
	}
	sealed, err := s.Box.Seal(secret)
	if err != nil {
		return "", apperror.Wrap(apperror.KindInternal, "no se pudo proteger el secreto", err)
	}
	err = s.Tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.Repo.UpdateSecret(ctx, orgID, id, sealed, s.Clock.Now()); err != nil {
			return err
		}
		return s.audit(ctx, "webhook.updated", actorID, orgID, st.Webhook)
	})
	if err != nil {
		return "", err
	}
	return secret, nil
}

// Deliveries lists the latest deliveries of a webhook.
func (s *Service) Deliveries(ctx context.Context, actorID, orgID, id uuid.UUID, limit int) ([]domain.Delivery, error) {
	if err := s.Access.RequireAdmin(ctx, actorID, orgID); err != nil {
		return nil, err
	}
	if _, err := s.Repo.Get(ctx, orgID, id); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.Repo.ListDeliveries(ctx, id, limit)
}

// Test sends a ping once, without retries.
func (s *Service) Test(ctx context.Context, actorID, orgID, id uuid.UUID) (domain.Delivery, error) {
	if err := s.Access.RequireAdmin(ctx, actorID, orgID); err != nil {
		return domain.Delivery{}, err
	}
	st, err := s.Repo.Get(ctx, orgID, id)
	if err != nil {
		return domain.Delivery{}, err
	}
	now := s.Clock.Now()
	payload, err := json.Marshal(map[string]any{
		"version": domain.PayloadVersion, "id": "ping_" + uuid.NewString(), "event": domain.EventPing,
		"occurred_at": now.Format(time.RFC3339), "organization_id": orgID.String(),
		"actor": map[string]any{"id": actorID.String()}, "webhook_id": id.String(),
	})
	if err != nil {
		return domain.Delivery{}, apperror.Wrap(apperror.KindInternal, "no se pudo crear el ping", err)
	}
	d := domain.Delivery{WebhookID: id, EventType: domain.EventPing, Payload: payload, Status: domain.StatusPending, NextAttemptAt: &now, CreatedAt: now}
	if _, err := s.Repo.InsertDelivery(ctx, &d); err != nil {
		return domain.Delivery{}, err
	}
	due := port.DueDelivery{Delivery: d, URL: st.URL, SecretEncrypted: st.SecretEncrypted}
	if err := s.deliver(ctx, due, false); err != nil {
		return domain.Delivery{}, err
	}
	return s.Repo.GetDelivery(ctx, id, d.ID)
}

// Redeliver queues a new delivery with the payload of an earlier one.
func (s *Service) Redeliver(ctx context.Context, actorID, orgID, id, deliveryID uuid.UUID) (domain.Delivery, error) {
	if err := s.Access.RequireAdmin(ctx, actorID, orgID); err != nil {
		return domain.Delivery{}, err
	}
	if _, err := s.Repo.Get(ctx, orgID, id); err != nil {
		return domain.Delivery{}, err
	}
	old, err := s.Repo.GetDelivery(ctx, id, deliveryID)
	if err != nil {
		return domain.Delivery{}, err
	}
	now := s.Clock.Now()
	d := domain.Delivery{WebhookID: id, EventType: old.EventType, Payload: old.Payload, Status: domain.StatusPending, NextAttemptAt: &now, CreatedAt: now}
	if _, err := s.Repo.InsertDelivery(ctx, &d); err != nil {
		return domain.Delivery{}, err
	}
	return d, nil
}

// eventID renders the stable id of an event for payloads.
func eventID(id int64) string { return "evt_" + strconv.FormatInt(id, 10) }
