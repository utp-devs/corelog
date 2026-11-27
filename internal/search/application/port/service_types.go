package port

import (
	"context"
	"github.com/google/uuid"
	"github.com/jhonsferg/corelog/internal/search/domain"
	"github.com/jhonsferg/corelog/internal/search/domain/coreql"
)

// SearchInput is one search request.
type SearchInput struct {
	Viewer    Viewer
	Query     string
	Order     string // optional ORDER BY list that replaces the one in Query
	ProjectID *uuid.UUID
	Page      int
	PageSize  int
	WithTotal bool
}

// SearchOutput is a page of results with the parse warnings.
type SearchOutput struct {
	Page       IssuePage
	PageNumber int
	PageSize   int
	Warnings   []domain.Warning
	Normalized string
}

// ValidationReport is the result of validating a query text.
type ValidationReport struct {
	Valid      bool
	Normalized string
	Problems   []domain.Problem
}

// SuggestInput asks for completions: of the query at a cursor (Query, Cursor) or of the values of a field.
type SuggestInput struct {
	Viewer Viewer
	Query  string
	Cursor int // rune offset; negative means the end of Query
	Field  string
	Prefix string
	Limit  int
}

// SuggestOutput carries the syntax completions or the value suggestions.
type SuggestOutput struct {
	Completions []coreql.Suggestion
	Values      []ValueSuggestion
}

// FilterDetail is a filter with the data only its audience sees: the shares (owner) and the viewer's own
// subscription.
type FilterDetail struct {
	FilterView
	Shares       []ShareView
	Subscription *domain.Subscription
}

// FilterPatch holds the fields to change; nil leaves a field as it is.
type FilterPatch struct {
	Name        *string
	Description *string
	QueryText   *string
	Columns     *[]string
	Sort        *string
}

// IssueSearchService is the issue search use case seen by the HTTP adapter.
type IssueSearchService interface {
	Search(ctx context.Context, in SearchInput) (SearchOutput, error)
	Validate(ctx context.Context, viewer Viewer, text string, project *uuid.UUID) (ValidationReport, error)
	Suggest(ctx context.Context, in SuggestInput) (SuggestOutput, error)
}

// FilterService is the saved filter use case seen by the HTTP adapter.
type FilterService interface {
	Create(ctx context.Context, viewer Viewer, organizationID uuid.UUID, fields domain.FilterFields) (*FilterDetail, error)
	Get(ctx context.Context, viewerID, id uuid.UUID) (*FilterDetail, error)
	List(ctx context.Context, viewerID uuid.UUID, q FilterListQuery) ([]FilterView, int, error)
	Update(ctx context.Context, viewer Viewer, id uuid.UUID, patch FilterPatch) (*FilterDetail, error)
	Delete(ctx context.Context, viewerID, id uuid.UUID) error
	SetShares(ctx context.Context, viewerID, id uuid.UUID, shares []domain.Share) ([]ShareView, error)
	Run(ctx context.Context, viewer Viewer, id uuid.UUID, page, pageSize int, withTotal bool) (SearchOutput, error)
	Subscribe(ctx context.Context, viewer Viewer, id uuid.UUID, schedule domain.Schedule) (domain.Subscription, error)
	Unsubscribe(ctx context.Context, viewerID, id uuid.UUID) error
	DueSubscriptions(ctx context.Context) ([]DueSubscription, error)
	MarkDelivered(ctx context.Context, subscriptionID uuid.UUID) error
}
