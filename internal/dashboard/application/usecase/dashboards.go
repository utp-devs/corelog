package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/dashboard/application/port"
	"github.com/jhonsferg/corelog/internal/dashboard/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
)

// DataCacheTTL is how long a computed gadget is reused for the same viewer.
const DataCacheTTL = 15 * time.Second

const maxCacheEntries = 512

// DashboardsDeps are the collaborators of the dashboards use case. Issues and Upcoming may be nil: the gadgets
// that need them then answer with an unavailable-source error.
type DashboardsDeps struct {
	Repository port.Repository
	Data       port.DataSource
	Issues     port.IssueRunner
	Upcoming   port.UpcomingSource
	Clock      clock.Clock
}

// DashboardsService implements port.DashboardService.
type DashboardsService struct {
	repo     port.Repository
	data     port.DataSource
	issues   port.IssueRunner
	upcoming port.UpcomingSource
	clock    clock.Clock

	mu    sync.Mutex
	cache map[cacheKey]cacheEntry
}

type cacheKey struct {
	viewer, gadget uuid.UUID
	stamp          int64 // updated_at of the dashboard: a change of the gadgets invalidates the entry
	admin          bool  // the support view of the platform administrator differs from the regular one
}

type cacheEntry struct {
	data    port.GadgetData
	expires time.Time
}

var _ port.DashboardService = (*DashboardsService)(nil)

// NewDashboardsService builds the service.
func NewDashboardsService(d DashboardsDeps) *DashboardsService {
	return &DashboardsService{repo: d.Repository, data: d.Data, issues: d.Issues, upcoming: d.Upcoming, clock: d.Clock, cache: map[cacheKey]cacheEntry{}}
}

func (s *DashboardsService) detail(ctx context.Context, view *port.DashboardView) (*port.DashboardDetail, error) {
	out := &port.DashboardDetail{DashboardView: *view}
	if view.Access.CanManage() {
		shares, err := s.repo.Shares(ctx, view.ID)
		if err != nil {
			return nil, err
		}
		out.Shares = shares
	}
	return out, nil
}

// List returns the dashboards the viewer can see.
func (s *DashboardsService) List(ctx context.Context, viewer port.Viewer, q port.ListQuery) ([]port.DashboardView, int, error) {
	switch q.Scope {
	case "", "all", "mine", "shared":
	default:
		return nil, 0, apperror.WithCode(apperror.KindValidation, domain.CodeInvalidDashboard, "scope debe ser mine, shared o all")
	}
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 {
		q.PageSize = 20
	}
	if q.PageSize > 100 {
		q.PageSize = 100
	}
	return s.repo.List(ctx, viewer, q)
}

// Create stores a new empty dashboard owned by the viewer.
func (s *DashboardsService) Create(ctx context.Context, viewer port.Viewer, in port.CreateInput) (*port.DashboardDetail, error) {
	d, err := s.build(in.Name, in.Description, in.Layout)
	if err != nil {
		return nil, err
	}
	return s.createFor(ctx, viewer, in.OrganizationID, d)
}

func (s *DashboardsService) build(name, description, layout string) (*domain.Dashboard, error) {
	name, err := domain.NormalizeName(name)
	if err != nil {
		return nil, err
	}
	description, err = domain.NormalizeDescription(description)
	if err != nil {
		return nil, err
	}
	l, err := domain.ParseLayout(layout)
	if err != nil {
		return nil, err
	}
	return &domain.Dashboard{Name: name, Description: description, Layout: l}, nil
}

func (s *DashboardsService) createFor(ctx context.Context, viewer port.Viewer, org uuid.UUID, d *domain.Dashboard) (*port.DashboardDetail, error) {
	orgID, err := s.repo.OrganizationFor(ctx, viewer.UserID, org)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	d.OwnerID, d.OrganizationID, d.CreatedAt, d.UpdatedAt = viewer.UserID, orgID, now, now
	if err := s.repo.Create(ctx, d); err != nil {
		return nil, err
	}
	return s.Get(ctx, viewer, d.ID)
}

// Get returns one dashboard the viewer can see. Only the owner receives the shares.
func (s *DashboardsService) Get(ctx context.Context, viewer port.Viewer, id uuid.UUID) (*port.DashboardDetail, error) {
	view, err := s.repo.Get(ctx, viewer, id)
	if err != nil {
		return nil, err
	}
	return s.detail(ctx, view)
}

func (s *DashboardsService) load(ctx context.Context, viewer port.Viewer, id uuid.UUID, needManage bool) (*port.DashboardView, error) {
	view, err := s.repo.Get(ctx, viewer, id)
	if err != nil {
		return nil, err
	}
	if needManage && !view.Access.CanManage() || !needManage && !view.Access.CanEdit() {
		return nil, domain.ErrForbidden
	}
	return view, nil
}

// Update changes the name, description, layout or default flag.
func (s *DashboardsService) Update(ctx context.Context, viewer port.Viewer, id uuid.UUID, in port.UpdateInput) (*port.DashboardDetail, error) {
	view, err := s.load(ctx, viewer, id, false)
	if err != nil {
		return nil, err
	}
	patch := port.FieldsPatch{}
	if in.Name != nil {
		n, err := domain.NormalizeName(*in.Name)
		if err != nil {
			return nil, err
		}
		patch.Name = &n
	}
	if in.Description != nil {
		d, err := domain.NormalizeDescription(*in.Description)
		if err != nil {
			return nil, err
		}
		patch.Description = &d
	}
	if in.Layout != nil {
		l, err := domain.ParseLayout(*in.Layout)
		if err != nil {
			return nil, err
		}
		for _, g := range view.Gadgets {
			if g.ColumnIndex >= l.Columns() {
				return nil, apperror.WithCode(apperror.KindValidation, domain.CodeInvalidDashboard, "el diseño no tiene sitio para los gadgets de la columna "+fmt.Sprint(g.ColumnIndex+1)).
					WithDetails(apperror.FieldError{Field: "layout", Message: "mueve primero los gadgets de las columnas que desaparecen"})
			}
		}
		patch.Layout = &l
	}
	if in.IsDefault != nil {
		if !view.Access.CanManage() {
			return nil, domain.ErrForbidden
		}
		patch.IsDefault = in.IsDefault
	}
	if err := s.repo.UpdateFields(ctx, id, patch, s.clock.Now()); err != nil {
		return nil, err
	}
	return s.Get(ctx, viewer, id)
}

// Delete removes a dashboard; only its owner can.
func (s *DashboardsService) Delete(ctx context.Context, viewer port.Viewer, id uuid.UUID) error {
	if _, err := s.load(ctx, viewer, id, true); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

// ReplaceGadgets validates and swaps the gadget list (and the layout when given) atomically.
func (s *DashboardsService) ReplaceGadgets(ctx context.Context, viewer port.Viewer, id uuid.UUID, layout *string, gadgets []port.GadgetInput) (*port.DashboardDetail, error) {
	view, err := s.load(ctx, viewer, id, false)
	if err != nil {
		return nil, err
	}
	target := view.Layout
	var newLayout *domain.Layout
	if layout != nil {
		l, err := domain.ParseLayout(*layout)
		if err != nil {
			return nil, err
		}
		target, newLayout = l, &l
	}
	existing := map[uuid.UUID]bool{}
	for _, g := range view.Gadgets {
		existing[g.ID] = true
	}
	list, err := domain.NormalizeGadgets(target, gadgets, existing)
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.ReplaceGadgets(ctx, id, newLayout, list, s.clock.Now()); err != nil {
		return nil, err
	}
	return s.Get(ctx, viewer, id)
}

// SetShares replaces the shares of a dashboard; only its owner can.
func (s *DashboardsService) SetShares(ctx context.Context, viewer port.Viewer, id uuid.UUID, shares []domain.Share) (*port.DashboardDetail, error) {
	view, err := s.load(ctx, viewer, id, true)
	if err != nil {
		return nil, err
	}
	list, err := domain.NormalizeShares(shares, view.OrganizationID, view.OwnerID)
	if err != nil {
		return nil, err
	}
	if err := s.repo.ReplaceShares(ctx, id, view.OrganizationID, list, s.clock.Now()); err != nil {
		return nil, err
	}
	return s.Get(ctx, viewer, id)
}

// Duplicate copies a visible dashboard, with its gadgets, into a new one owned by the viewer. The shares are not
// copied. A name already in use gets a numeric suffix.
func (s *DashboardsService) Duplicate(ctx context.Context, viewer port.Viewer, id uuid.UUID, name string) (*port.DashboardDetail, error) {
	source, err := s.repo.Get(ctx, viewer, id)
	if err != nil {
		return nil, err
	}
	base := strings.TrimSpace(name)
	if base == "" {
		base = "Copia de " + source.Name
	}
	if len([]rune(base)) > domain.MaxName-6 {
		base = string([]rune(base)[:domain.MaxName-6])
	}
	gadgets := make([]domain.Gadget, len(source.Gadgets))
	for i, g := range source.Gadgets {
		g.ID = uuid.Nil
		gadgets[i] = g
	}
	for attempt := 1; attempt <= 20; attempt++ {
		candidate := base
		if attempt > 1 {
			candidate = fmt.Sprintf("%s (%d)", base, attempt)
		}
		d := &domain.Dashboard{Name: candidate, Description: source.Description, Layout: source.Layout, Gadgets: gadgets}
		detail, err := s.createFor(ctx, viewer, source.OrganizationID, d)
		if errors.Is(err, domain.ErrNameTaken) || apperror.CodeOf(err) == domain.CodeNameTaken {
			continue
		}
		return detail, err
	}
	return nil, domain.ErrNameTaken
}

// defaultGadgets is the standard set of a new personal dashboard.
func defaultGadgets() []domain.Gadget {
	mk := func(t string, col, pos int, cfg string) domain.Gadget {
		return domain.Gadget{Type: t, Title: domain.DefaultTitle(t), Config: json.RawMessage(cfg), ColumnIndex: col, Position: pos}
	}
	return []domain.Gadget{
		mk(domain.GadgetAssignedToMe, 0, 0, `{"limit":10,"include_done":false}`),
		mk(domain.GadgetActivityStream, 0, 1, `{"project_id":null,"limit":10}`),
		mk(domain.GadgetCalendarUpcoming, 0, 2, `{"days":14,"limit":10}`),
		mk(domain.GadgetCreatedVsResolved, 1, 0, `{"project_id":null,"days":30}`),
		mk(domain.GadgetRecentProjects, 1, 1, `{"limit":10}`),
		mk(domain.GadgetFavorites, 1, 2, `{"types":[],"limit":10}`),
	}
}

// Default returns the personal default dashboard and creates it with the standard gadgets on first use.
func (s *DashboardsService) Default(ctx context.Context, viewer port.Viewer) (*port.DashboardDetail, error) {
	found, err := s.repo.FindDefault(ctx, viewer)
	if err != nil {
		return nil, err
	}
	if found != nil {
		return s.detail(ctx, found)
	}
	d := &domain.Dashboard{Name: "Mi panel", Layout: domain.Layout2, IsDefault: true, Gadgets: defaultGadgets()}
	detail, err := s.createFor(ctx, viewer, uuid.Nil, d)
	if err != nil && apperror.CodeOf(err) == domain.CodeNameTaken {
		// The name is used by a dashboard that is not the default: pick another one.
		d.Name = "Mi panel principal"
		detail, err = s.createFor(ctx, viewer, uuid.Nil, d)
	}
	if err != nil {
		// A concurrent first request may have created it already.
		if again, ferr := s.repo.FindDefault(ctx, viewer); ferr == nil && again != nil {
			return s.detail(ctx, again)
		}
		return nil, err
	}
	return detail, nil
}

// GadgetData computes the data of one gadget as the viewer. Results are reused for DataCacheTTL for the same
// viewer, so a shared dashboard never serves one viewer's data to another.
func (s *DashboardsService) GadgetData(ctx context.Context, viewer port.Viewer, dashboardID, gadgetID uuid.UUID) (port.GadgetData, error) {
	view, err := s.repo.Get(ctx, viewer, dashboardID)
	if err != nil {
		return port.GadgetData{}, err
	}
	var gadget *domain.Gadget
	for i := range view.Gadgets {
		if view.Gadgets[i].ID == gadgetID {
			gadget = &view.Gadgets[i]
		}
	}
	if gadget == nil {
		return port.GadgetData{}, domain.ErrGadgetNotFound
	}
	now := s.clock.Now()
	key := cacheKey{viewer: viewer.UserID, gadget: gadgetID, stamp: view.UpdatedAt.UnixNano(), admin: viewer.PlatformAdmin}
	s.mu.Lock()
	if e, ok := s.cache[key]; ok && now.Before(e.expires) {
		s.mu.Unlock()
		return e.data, nil
	}
	s.mu.Unlock()

	data, err := s.compute(ctx, viewer, *gadget, now)
	if err != nil {
		return port.GadgetData{}, err
	}
	out := port.GadgetData{GadgetID: gadget.ID, Type: gadget.Type, GeneratedAt: now, Data: data}
	s.mu.Lock()
	if len(s.cache) >= maxCacheEntries {
		s.cache = map[cacheKey]cacheEntry{}
	}
	s.cache[key] = cacheEntry{data: out, expires: now.Add(DataCacheTTL)}
	s.mu.Unlock()
	return out, nil
}

func unavailable(what string) error {
	return apperror.WithCode(apperror.KindInternal, domain.CodeSourceFailed, what+" no está disponible")
}

func (s *DashboardsService) compute(ctx context.Context, viewer port.Viewer, g domain.Gadget, now time.Time) (any, error) {
	decode := func(dst any) error {
		if err := json.Unmarshal(g.Config, dst); err != nil {
			return apperror.Wrap(apperror.KindInternal, "stored gadget configuration is not valid", err)
		}
		return nil
	}
	switch g.Type {
	case domain.GadgetAssignedToMe, domain.GadgetCreatedByMe:
		var c domain.ListConfig
		if err := decode(&c); err != nil {
			return nil, err
		}
		field := "assignee"
		if g.Type == domain.GadgetCreatedByMe {
			field = "reporter"
		}
		query := field + " = currentUser()"
		if !c.IncludeDone {
			query += " AND statusCategory != done"
		}
		query += " ORDER BY updated DESC"
		return s.runQuery(ctx, viewer, query, c.Limit, []string{"key", "summary", "status", "priority", "due"})
	case domain.GadgetFilterResults:
		var c domain.FilterResultsConfig
		if err := decode(&c); err != nil {
			return nil, err
		}
		if s.issues == nil {
			return nil, unavailable("la búsqueda de incidencias")
		}
		var list port.IssueList
		var err error
		if c.FilterID != nil {
			list, err = s.issues.RunFilter(ctx, viewer, *c.FilterID, c.Limit)
		} else {
			list, err = s.issues.RunQuery(ctx, viewer, c.Query, c.Limit)
		}
		if err != nil {
			return nil, err
		}
		return port.IssueListData{Total: list.Total, Columns: c.Columns, Query: list.Query, Items: nonNilRows(list.Items)}, nil
	case domain.GadgetIssueStatistics, domain.GadgetPie:
		var c domain.StatsConfig
		if err := decode(&c); err != nil {
			return nil, err
		}
		return s.data.Statistics(ctx, viewer, c)
	case domain.GadgetTwoDimensional:
		var c domain.TwoDimensionalConfig
		if err := decode(&c); err != nil {
			return nil, err
		}
		return s.data.TwoDimensional(ctx, viewer, c)
	case domain.GadgetCreatedVsResolved:
		var c domain.CreatedVsResolvedConfig
		if err := decode(&c); err != nil {
			return nil, err
		}
		return s.data.CreatedVsResolved(ctx, viewer, c, now)
	case domain.GadgetSprintBurndown:
		var c domain.BurndownConfig
		if err := decode(&c); err != nil {
			return nil, err
		}
		return s.data.Burndown(ctx, viewer, c, now)
	case domain.GadgetActivityStream:
		var c domain.ActivityConfig
		if err := decode(&c); err != nil {
			return nil, err
		}
		items, err := s.data.Activity(ctx, viewer, c)
		if items == nil {
			items = []port.ActivityItem{}
		}
		return map[string]any{"items": items}, err
	case domain.GadgetRecentProjects:
		var c domain.RecentProjectsConfig
		if err := decode(&c); err != nil {
			return nil, err
		}
		items, err := s.data.RecentProjects(ctx, viewer, c.Limit)
		if items == nil {
			items = []port.ProjectItem{}
		}
		return map[string]any{"items": items}, err
	case domain.GadgetFavorites:
		var c domain.FavoritesConfig
		if err := decode(&c); err != nil {
			return nil, err
		}
		items, err := s.data.Favorites(ctx, viewer, c)
		if items == nil {
			items = []port.FavoriteItem{}
		}
		return map[string]any{"items": items}, err
	case domain.GadgetText:
		var c domain.TextConfig
		if err := decode(&c); err != nil {
			return nil, err
		}
		return port.TextData{Markdown: c.Markdown}, nil
	case domain.GadgetCalendarUpcoming:
		var c domain.CalendarUpcomingConfig
		if err := decode(&c); err != nil {
			return nil, err
		}
		if s.upcoming == nil {
			return nil, unavailable("el calendario")
		}
		from := now
		to := now.AddDate(0, 0, c.Days)
		items, err := s.upcoming.Upcoming(ctx, viewer, from, to, c.Limit)
		if err != nil {
			return nil, err
		}
		if items == nil {
			items = []port.UpcomingItem{}
		}
		return port.UpcomingData{From: from, To: to, Items: items}, nil
	}
	return nil, domain.ErrGadgetNotFound
}

func (s *DashboardsService) runQuery(ctx context.Context, viewer port.Viewer, query string, limit int, columns []string) (any, error) {
	if s.issues == nil {
		return nil, unavailable("la búsqueda de incidencias")
	}
	list, err := s.issues.RunQuery(ctx, viewer, query, limit)
	if err != nil {
		return nil, err
	}
	return port.IssueListData{Total: list.Total, Columns: columns, Query: query, Items: nonNilRows(list.Items)}, nil
}

func nonNilRows(rows []port.IssueRow) []port.IssueRow {
	if rows == nil {
		return []port.IssueRow{}
	}
	return rows
}
