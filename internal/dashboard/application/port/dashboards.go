package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/dashboard/domain"
)

// Viewer is the caller. PlatformAdmin is the global administrator (support access to every live project).
type Viewer struct {
	UserID        uuid.UUID
	PlatformAdmin bool
}

// DashboardView is a dashboard with what the viewer can do with it.
type DashboardView struct {
	domain.Dashboard
	OwnerName  string
	Access     domain.Access
	IsFavorite bool
	Shared     bool
}

// ShareView is a share with the name of its target.
type ShareView struct {
	domain.Share
	TargetName string
}

// DashboardDetail is a dashboard with its shares (only for the owner).
type DashboardDetail struct {
	DashboardView
	Shares []ShareView
}

// ListQuery filters the dashboards a viewer can see. Scope is "mine", "shared" or "all" (default).
type ListQuery struct {
	Scope    string
	Query    string
	Page     int
	PageSize int
}

// FieldsPatch holds the dashboard fields to change; nil leaves a field as it is.
type FieldsPatch struct {
	Name        *string
	Description *string
	Layout      *domain.Layout
	IsDefault   *bool
}

// Repository persists dashboards, gadgets and shares. Reads only return what the viewer can see.
type Repository interface {
	OrganizationFor(ctx context.Context, userID, requested uuid.UUID) (uuid.UUID, error)
	List(ctx context.Context, viewer Viewer, q ListQuery) ([]DashboardView, int, error)
	// Get returns the dashboard with its gadgets, or domain.ErrNotFound when it is missing or hidden.
	Get(ctx context.Context, viewer Viewer, id uuid.UUID) (*DashboardView, error)
	// FindDefault returns the default dashboard of the user, or nil when there is none.
	FindDefault(ctx context.Context, viewer Viewer) (*DashboardView, error)
	// Create stores the dashboard with its gadgets in one transaction and fills the ids. When IsDefault is set it
	// replaces the previous default of the owner.
	Create(ctx context.Context, d *domain.Dashboard) error
	// UpdateFields changes the fields and stamps updatedAt. Setting IsDefault replaces the previous default.
	UpdateFields(ctx context.Context, id uuid.UUID, p FieldsPatch, updatedAt time.Time) error
	Delete(ctx context.Context, id uuid.UUID) error
	// ReplaceGadgets swaps the whole gadget list (and the layout when given) atomically and returns the stored
	// gadgets. Gadgets with an id keep it.
	ReplaceGadgets(ctx context.Context, id uuid.UUID, layout *domain.Layout, gadgets []domain.Gadget, at time.Time) ([]domain.Gadget, error)
	Shares(ctx context.Context, id uuid.UUID) ([]ShareView, error)
	// ReplaceShares swaps the shares; every target must exist in the organization of the dashboard.
	ReplaceShares(ctx context.Context, id, organizationID uuid.UUID, shares []domain.Share, at time.Time) error
}

// IssueRow is one issue of a result gadget.
type IssueRow struct {
	ID             uuid.UUID `json:"id"`
	Key            string    `json:"key"`
	Summary        string    `json:"summary"`
	Status         string    `json:"status"`
	StatusCategory string    `json:"status_category"`
	Priority       string    `json:"priority"`
	Type           string    `json:"type"`
	Assignee       string    `json:"assignee"`
	ProjectKey     string    `json:"project_key"`
	Due            *string   `json:"due"`
	Updated        time.Time `json:"updated"`
}

// IssueList is a page of issues returned by the issue runner. Columns holds the columns of a saved filter.
type IssueList struct {
	Total   int
	Items   []IssueRow
	Query   string
	Columns []string
}

// IssueRunner runs CoreQL queries and saved filters as the viewer. It restricts the result to the issues the
// viewer can browse. The root of composition wires it to the search module.
type IssueRunner interface {
	RunQuery(ctx context.Context, viewer Viewer, query string, limit int) (IssueList, error)
	RunFilter(ctx context.Context, viewer Viewer, filterID uuid.UUID, limit int) (IssueList, error)
}

// UpcomingItem is a calendar entry for the upcoming gadget.
type UpcomingItem struct {
	Kind     string    `json:"kind"`
	Title    string    `json:"title"`
	StartsAt time.Time `json:"starts_at"`
	EndsAt   time.Time `json:"ends_at"`
	AllDay   bool      `json:"all_day"`
	Link     string    `json:"link"`
	Project  string    `json:"project_key"`
}

// UpcomingSource lists the calendar entries of the viewer in a window, ordered by start. The root of composition
// wires it to the calendar module.
type UpcomingSource interface {
	Upcoming(ctx context.Context, viewer Viewer, from, to time.Time, limit int) ([]UpcomingItem, error)
}

// StatGroup is one bar or slice of a statistics gadget.
type StatGroup struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

// StatsData is the data of issue_statistics and pie. Other holds what did not fit in the limit.
type StatsData struct {
	GroupBy string      `json:"group_by"`
	Total   int         `json:"total"`
	Groups  []StatGroup `json:"groups"`
	Other   int         `json:"other"`
}

// MatrixCell is a count of the two-dimensional gadget.
type MatrixCell struct {
	X     string `json:"x"`
	Y     string `json:"y"`
	Count int    `json:"count"`
}

// MatrixData is the data of two_dimensional.
type MatrixData struct {
	XAxis string       `json:"x_axis"`
	YAxis string       `json:"y_axis"`
	Total int          `json:"total"`
	XKeys []StatGroup  `json:"x"`
	YKeys []StatGroup  `json:"y"`
	Cells []MatrixCell `json:"cells"`
}

// DayPoint is a day of the created-versus-resolved series.
type DayPoint struct {
	Date     string `json:"date"`
	Created  int    `json:"created"`
	Resolved int    `json:"resolved"`
}

// CreatedVsResolvedData is the data of created_vs_resolved.
type CreatedVsResolvedData struct {
	Days             int        `json:"days"`
	Series           []DayPoint `json:"series"`
	TotalCreated     int        `json:"total_created"`
	TotalResolved    int        `json:"total_resolved"`
	CumulativeOpened int        `json:"net_open_change"`
}

// BurndownPoint is a day of the sprint burndown.
type BurndownPoint struct {
	Date      string   `json:"date"`
	Remaining *float64 `json:"remaining"` // nil for days after today
	Ideal     float64  `json:"ideal"`
}

// BurndownData is the data of sprint_burndown. Sprint is nil when the viewer has no sprint to show.
type BurndownData struct {
	Sprint *BurndownSprint `json:"sprint"`
	Total  float64         `json:"total_points"`
	Points []BurndownPoint `json:"points"`
}

// BurndownSprint names a sprint.
type BurndownSprint struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	ProjectKey string    `json:"project_key"`
	StartDate  *string   `json:"start_date"`
	EndDate    *string   `json:"end_date"`
}

// ActivityItem is one entry of the activity stream.
type ActivityItem struct {
	ID         uuid.UUID `json:"id"`
	IssueID    uuid.UUID `json:"issue_id"`
	IssueKey   string    `json:"issue_key"`
	Summary    string    `json:"summary"`
	Actor      string    `json:"actor"`
	Action     string    `json:"action"`
	Field      string    `json:"field"`
	OldValue   string    `json:"old_value"`
	NewValue   string    `json:"new_value"`
	OccurredAt time.Time `json:"occurred_at"`
}

// ProjectItem is a project of the recent projects gadget.
type ProjectItem struct {
	ID       uuid.UUID `json:"id"`
	Key      string    `json:"key"`
	Name     string    `json:"name"`
	Color    string    `json:"color"`
	ViewedAt time.Time `json:"viewed_at"`
}

// FavoriteItem is a starred entity.
type FavoriteItem struct {
	Type      string    `json:"type"`
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Link      string    `json:"link"`
	StarredAt time.Time `json:"starred_at"`
}

// DataSource computes the gadget data that comes straight from SQL. Every method restricts what it reads to the
// projects the viewer can browse.
type DataSource interface {
	Statistics(ctx context.Context, viewer Viewer, cfg domain.StatsConfig) (StatsData, error)
	TwoDimensional(ctx context.Context, viewer Viewer, cfg domain.TwoDimensionalConfig) (MatrixData, error)
	CreatedVsResolved(ctx context.Context, viewer Viewer, cfg domain.CreatedVsResolvedConfig, today time.Time) (CreatedVsResolvedData, error)
	Burndown(ctx context.Context, viewer Viewer, cfg domain.BurndownConfig, today time.Time) (BurndownData, error)
	Activity(ctx context.Context, viewer Viewer, cfg domain.ActivityConfig) ([]ActivityItem, error)
	RecentProjects(ctx context.Context, viewer Viewer, limit int) ([]ProjectItem, error)
	Favorites(ctx context.Context, viewer Viewer, cfg domain.FavoritesConfig) ([]FavoriteItem, error)
}

// GadgetData is the computed content of a gadget. Data is one of the types above, an IssueListData, a
// TextData or an UpcomingData.
type GadgetData struct {
	GadgetID    uuid.UUID
	Type        string
	GeneratedAt time.Time
	Data        any
}

// IssueListData is the data of assigned_to_me, created_by_me and filter_results.
type IssueListData struct {
	Total   int        `json:"total"`
	Columns []string   `json:"columns"`
	Query   string     `json:"query"`
	Items   []IssueRow `json:"items"`
}

// TextData is the data of text.
type TextData struct {
	Markdown string `json:"markdown"`
}

// UpcomingData is the data of calendar_upcoming.
type UpcomingData struct {
	From  time.Time      `json:"from"`
	To    time.Time      `json:"to"`
	Items []UpcomingItem `json:"items"`
}

// GadgetInput is a gadget of a layout replacement.
type GadgetInput = domain.GadgetInput

// CreateInput is the body of a new dashboard.
type CreateInput struct {
	OrganizationID uuid.UUID
	Name           string
	Description    string
	Layout         string
}

// UpdateInput is a partial update; nil fields stay as they are.
type UpdateInput struct {
	Name        *string
	Description *string
	Layout      *string
	IsDefault   *bool
}

// DashboardService is the dashboard use case seen by the HTTP adapter.
type DashboardService interface {
	List(ctx context.Context, viewer Viewer, q ListQuery) ([]DashboardView, int, error)
	Create(ctx context.Context, viewer Viewer, in CreateInput) (*DashboardDetail, error)
	Get(ctx context.Context, viewer Viewer, id uuid.UUID) (*DashboardDetail, error)
	Update(ctx context.Context, viewer Viewer, id uuid.UUID, in UpdateInput) (*DashboardDetail, error)
	Delete(ctx context.Context, viewer Viewer, id uuid.UUID) error
	ReplaceGadgets(ctx context.Context, viewer Viewer, id uuid.UUID, layout *string, gadgets []GadgetInput) (*DashboardDetail, error)
	SetShares(ctx context.Context, viewer Viewer, id uuid.UUID, shares []domain.Share) (*DashboardDetail, error)
	Duplicate(ctx context.Context, viewer Viewer, id uuid.UUID, name string) (*DashboardDetail, error)
	// Default returns the personal default dashboard, creating it with the standard gadgets on first use.
	Default(ctx context.Context, viewer Viewer) (*DashboardDetail, error)
	GadgetData(ctx context.Context, viewer Viewer, dashboardID, gadgetID uuid.UUID) (GadgetData, error)
}
