package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/search/application/port"
	"github.com/jhonsferg/corelog/internal/search/domain"
)

// FilterService manages saved filters. Reads of hidden filters are not-found; a visible filter the viewer
// cannot change is forbidden.
type FilterService struct {
	repo  port.FilterRepository
	jql   *JQLService
	clock clock.Clock
}

// NewFilterService builds the service.
func NewFilterService(repo port.FilterRepository, jql *JQLService, clk clock.Clock) *FilterService {
	if clk == nil {
		clk = clock.RealClock{}
	}
	return &FilterService{repo: repo, jql: jql, clock: clk}
}

// Create validates the query (syntax, names and build rules) and stores a filter owned by the viewer.
func (s *FilterService) Create(ctx context.Context, viewer port.Viewer, organizationID uuid.UUID, fields domain.FilterFields) (*port.FilterDetail, error) {
	fields, err := fields.Normalize()
	if err != nil {
		return nil, err
	}
	if _, err := s.jql.Check(ctx, viewer, fields.QueryText, fields.Sort, nil); err != nil {
		return nil, err
	}
	orgID, err := s.repo.OrganizationFor(ctx, viewer.UserID, organizationID)
	if err != nil {
		return nil, err
	}
	id, err := s.repo.Create(ctx, viewer.UserID, orgID, fields, s.clock.Now())
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, viewer.UserID, id)
}

// Get returns the filter with the shares when the viewer owns it.
func (s *FilterService) Get(ctx context.Context, viewerID, id uuid.UUID) (*port.FilterDetail, error) {
	view, err := s.repo.Get(ctx, viewerID, id)
	if err != nil {
		return nil, err
	}
	detail := &port.FilterDetail{FilterView: *view}
	if view.Access.CanManage() {
		if detail.Shares, err = s.repo.Shares(ctx, id); err != nil {
			return nil, err
		}
	}
	if view.HasSubscription {
		if detail.Subscription, err = s.repo.GetSubscription(ctx, id, viewerID); err != nil {
			return nil, err
		}
	}
	return detail, nil
}

// List returns a page of the filters visible to the viewer.
func (s *FilterService) List(ctx context.Context, viewerID uuid.UUID, q port.FilterListQuery) ([]port.FilterView, int, error) {
	switch q.Scope {
	case port.ScopeMine, port.ScopeShared, port.ScopeFavorite, port.ScopeAll:
	case "":
		q.Scope = port.ScopeAll
	default:
		return nil, 0, apperror.WithCode(apperror.KindValidation, domain.CodeFilterInvalid, "el ámbito debe ser all, mine, shared o favorite")
	}
	if q.PageSize <= 0 || q.PageSize > 100 {
		q.PageSize = 50
	}
	if q.Page < 1 {
		q.Page = 1
	}
	return s.repo.List(ctx, viewerID, q)
}

// Update changes a filter. Owners and editors can.
func (s *FilterService) Update(ctx context.Context, viewer port.Viewer, id uuid.UUID, patch port.FilterPatch) (*port.FilterDetail, error) {
	view, err := s.repo.Get(ctx, viewer.UserID, id)
	if err != nil {
		return nil, err
	}
	if !view.Access.CanEdit() {
		return nil, domain.ErrFilterForbidden
	}
	fields := domain.FilterFields{
		Name: view.Name, Description: view.Description, QueryText: view.QueryText, Columns: view.Columns, Sort: view.Sort,
	}
	if patch.Name != nil {
		fields.Name = *patch.Name
	}
	if patch.Description != nil {
		fields.Description = *patch.Description
	}
	if patch.QueryText != nil {
		fields.QueryText = *patch.QueryText
	}
	if patch.Columns != nil {
		fields.Columns = *patch.Columns
	}
	if patch.Sort != nil {
		fields.Sort = *patch.Sort
	}
	fields, err = fields.Normalize()
	if err != nil {
		return nil, err
	}
	if _, err := s.jql.Check(ctx, viewer, fields.QueryText, fields.Sort, nil); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, id, fields, s.clock.Now()); err != nil {
		return nil, err
	}
	return s.Get(ctx, viewer.UserID, id)
}

// Delete removes a filter. Only the owner can.
func (s *FilterService) Delete(ctx context.Context, viewerID, id uuid.UUID) error {
	view, err := s.repo.Get(ctx, viewerID, id)
	if err != nil {
		return err
	}
	if !view.Access.CanManage() {
		return domain.ErrFilterForbidden
	}
	return s.repo.Delete(ctx, id)
}

// SetShares replaces the shares of a filter. Only the owner can; targets must belong to the filter's
// organization.
func (s *FilterService) SetShares(ctx context.Context, viewerID, id uuid.UUID, shares []domain.Share) ([]port.ShareView, error) {
	view, err := s.repo.Get(ctx, viewerID, id)
	if err != nil {
		return nil, err
	}
	if !view.Access.CanManage() {
		return nil, domain.ErrFilterForbidden
	}
	shares, err = domain.NormalizeShares(shares)
	if err != nil {
		return nil, err
	}
	if err := s.repo.ReplaceShares(ctx, id, view.OrganizationID, shares, s.clock.Now()); err != nil {
		return nil, err
	}
	return s.repo.Shares(ctx, id)
}

// Run executes the filter for the viewer. The query is intersected with what the viewer can browse, whatever
// the filter says.
func (s *FilterService) Run(ctx context.Context, viewer port.Viewer, id uuid.UUID, page, pageSize int, withTotal bool) (port.SearchOutput, error) {
	view, err := s.repo.Get(ctx, viewer.UserID, id)
	if err != nil {
		return port.SearchOutput{}, err
	}
	return s.jql.Search(ctx, port.SearchInput{
		Viewer: viewer, Query: view.QueryText, Order: view.Sort, Page: page, PageSize: pageSize, WithTotal: withTotal,
	})
}

// Subscribe creates or replaces the viewer's subscription to a filter they can see.
func (s *FilterService) Subscribe(ctx context.Context, viewer port.Viewer, id uuid.UUID, schedule domain.Schedule) (domain.Subscription, error) {
	if _, err := s.repo.Get(ctx, viewer.UserID, id); err != nil {
		return domain.Subscription{}, err
	}
	if schedule.Timezone == "" && s.jql.viewers != nil {
		tz, err := s.jql.viewers.Timezone(ctx, viewer.UserID)
		if err != nil {
			return domain.Subscription{}, err
		}
		schedule.Timezone = tz
	}
	schedule, err := schedule.Validate()
	if err != nil {
		return domain.Subscription{}, err
	}
	return s.repo.UpsertSubscription(ctx, id, viewer.UserID, schedule, s.clock.Now())
}

// Unsubscribe removes the viewer's subscription.
func (s *FilterService) Unsubscribe(ctx context.Context, viewerID, id uuid.UUID) error {
	if _, err := s.repo.Get(ctx, viewerID, id); err != nil {
		return err
	}
	return s.repo.DeleteSubscription(ctx, id, viewerID)
}

// DueSubscriptions returns the subscriptions whose delivery is pending now and whose subscriber can still see
// the filter. It sends nothing: the scheduler delivers and then records the delivery.
func (s *FilterService) DueSubscriptions(ctx context.Context) ([]port.DueSubscription, error) {
	now := s.clock.Now()
	var due []port.DueSubscription
	after := uuid.Nil
	for {
		batch, err := s.repo.ListSubscriptions(ctx, after, 500)
		if err != nil {
			return nil, err
		}
		for _, sub := range batch {
			if !sub.IsDue(now) {
				continue
			}
			view, err := s.repo.Get(ctx, sub.UserID, sub.FilterID)
			if err != nil {
				if errors.Is(err, domain.ErrFilterNotFound) {
					continue
				}
				return nil, err
			}
			due = append(due, port.DueSubscription{Subscription: sub, Filter: view.Filter})
		}
		if len(batch) < 500 {
			return due, nil
		}
		after = batch[len(batch)-1].ID
	}
}

// MarkDelivered records that the scheduler delivered a subscription now, so it is not due again until its next
// scheduled slot.
func (s *FilterService) MarkDelivered(ctx context.Context, subscriptionID uuid.UUID) error {
	return s.repo.MarkSubscriptionSent(ctx, subscriptionID, s.clock.Now())
}
