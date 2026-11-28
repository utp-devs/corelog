package postgres

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/dashboard/application/port"
	"github.com/jhonsferg/corelog/internal/dashboard/domain"
)

// DataSource computes the gadget data with SQL. Every query takes the viewer as $1 and the platform flag as $2 and
// only reads issues of projects the viewer can browse, so a gadget of a shared dashboard never shows more than
// the viewer could find on their own.
type DataSource struct {
	db querier
}

var _ port.DataSource = (*DataSource)(nil)

// NewDataSource builds the data source.
func NewDataSource(db *sqlx.DB) *DataSource { return &DataSource{db: db} }

func visible(alias string) string { return visibleProjectSQL(alias, "$1::uuid", "$2::boolean") }

// issueScope is the FROM and WHERE of the issues the viewer can browse, optionally of one project ($3) and
// optionally without the done ones ($4 = include done).
const issueScope = `
	FROM issues i
	JOIN projects p ON p.id = i.project_id AND p.deleted_at IS NULL
	%JOINS%
	WHERE i.deleted_at IS NULL AND %VISIBLE%
	  AND ($3::uuid IS NULL OR i.project_id = $3::uuid)
	  AND ($4::boolean OR corelog_issue_category(i.status_id, i.status) <> 'done')`

type axisSQL struct {
	joins string
	key   string
	label string
}

// axis returns the joins and expressions that group issues by a whitelisted field. suffix keeps the aliases of
// two axes apart.
func axis(field, suffix string) axisSQL {
	switch field {
	case "assignee":
		return axisSQL{
			joins: ` LEFT JOIN users au` + suffix + ` ON au` + suffix + `.id = i.assignee_id`,
			key:   `COALESCE(i.assignee_id::text, 'unassigned')`,
			label: `COALESCE(au` + suffix + `.display_name, 'Sin asignar')`,
		}
	case "priority":
		return axisSQL{
			joins: ` LEFT JOIN priorities pr` + suffix + ` ON pr` + suffix + `.id = i.priority_id`,
			key:   `COALESCE(pr` + suffix + `.name, i.priority)`,
			label: `COALESCE(pr` + suffix + `.name, i.priority)`,
		}
	case "type":
		return axisSQL{
			joins: ` LEFT JOIN issue_types ty` + suffix + ` ON ty` + suffix + `.id = i.issue_type_id`,
			key:   `COALESCE(ty` + suffix + `.name, i.issue_type)`,
			label: `COALESCE(ty` + suffix + `.name, i.issue_type)`,
		}
	case "label":
		return axisSQL{
			joins: ` LEFT JOIN issue_labels il` + suffix + ` ON il` + suffix + `.issue_id = i.id LEFT JOIN labels lb` + suffix + ` ON lb` + suffix + `.id = il` + suffix + `.label_id`,
			key:   `COALESCE(lb` + suffix + `.name, 'Sin etiqueta')`,
			label: `COALESCE(lb` + suffix + `.name, 'Sin etiqueta')`,
		}
	case "project":
		return axisSQL{key: `p.key`, label: `p.key`}
	default: // status
		return axisSQL{
			joins: ` LEFT JOIN statuses st` + suffix + ` ON st` + suffix + `.id = i.status_id`,
			key:   `COALESCE(st` + suffix + `.name, i.status)`,
			label: `COALESCE(st` + suffix + `.name, i.status)`,
		}
	}
}

func scope(joins string) string {
	s := issueScope
	return strings.NewReplacer("%JOINS%", joins, "%VISIBLE%", visible("p")).Replace(s)
}

func nullUUID(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return *id
}

type groupRow struct {
	Key   string `db:"k"`
	Label string `db:"label"`
	Count int    `db:"count"`
}

const maxGroupRows = 500

// Statistics counts the visible issues grouped by one field.
func (d *DataSource) Statistics(ctx context.Context, viewer port.Viewer, cfg domain.StatsConfig) (port.StatsData, error) {
	a := axis(cfg.GroupBy, "x")
	var rows []groupRow
	err := d.db.SelectContext(ctx, &rows, `SELECT `+a.key+` AS k, `+a.label+` AS label, count(DISTINCT i.id) AS count`+scope(a.joins)+`
		GROUP BY 1, 2 ORDER BY count DESC, label LIMIT `+itoa(maxGroupRows),
		viewer.UserID, viewer.PlatformAdmin, nullUUID(cfg.ProjectID), cfg.IncludeDone)
	if err != nil {
		return port.StatsData{}, internal("failed to compute the issue statistics", err)
	}
	var total int
	if err := d.db.GetContext(ctx, &total, `SELECT count(*)`+scope(""), viewer.UserID, viewer.PlatformAdmin, nullUUID(cfg.ProjectID), cfg.IncludeDone); err != nil {
		return port.StatsData{}, internal("failed to count the issues", err)
	}
	out := port.StatsData{GroupBy: cfg.GroupBy, Total: total, Groups: []port.StatGroup{}}
	for i, r := range rows {
		if i < cfg.Limit {
			out.Groups = append(out.Groups, port.StatGroup{Key: r.Key, Label: r.Label, Count: r.Count})
		} else {
			out.Other += r.Count
		}
	}
	return out, nil
}

type cellRow struct {
	XKey   string `db:"xk"`
	XLabel string `db:"xl"`
	YKey   string `db:"yk"`
	YLabel string `db:"yl"`
	Count  int    `db:"count"`
}

// TwoDimensional counts the visible issues by two fields. Each axis keeps its busiest values up to the limit.
func (d *DataSource) TwoDimensional(ctx context.Context, viewer port.Viewer, cfg domain.TwoDimensionalConfig) (port.MatrixData, error) {
	x, y := axis(cfg.XAxis, "x"), axis(cfg.YAxis, "y")
	var rows []cellRow
	err := d.db.SelectContext(ctx, &rows, `SELECT `+x.key+` AS xk, `+x.label+` AS xl, `+y.key+` AS yk, `+y.label+` AS yl, count(DISTINCT i.id) AS count`+
		scope(x.joins+y.joins)+` GROUP BY 1, 2, 3, 4 ORDER BY count DESC LIMIT 5000`,
		viewer.UserID, viewer.PlatformAdmin, nullUUID(cfg.ProjectID), cfg.IncludeDone)
	if err != nil {
		return port.MatrixData{}, internal("failed to compute the two-dimensional statistics", err)
	}
	var total int
	if err := d.db.GetContext(ctx, &total, `SELECT count(*)`+scope(""), viewer.UserID, viewer.PlatformAdmin, nullUUID(cfg.ProjectID), cfg.IncludeDone); err != nil {
		return port.MatrixData{}, internal("failed to count the issues", err)
	}
	xTotals, yTotals := map[string]*port.StatGroup{}, map[string]*port.StatGroup{}
	for _, r := range rows {
		if g, ok := xTotals[r.XKey]; ok {
			g.Count += r.Count
		} else {
			xTotals[r.XKey] = &port.StatGroup{Key: r.XKey, Label: r.XLabel, Count: r.Count}
		}
		if g, ok := yTotals[r.YKey]; ok {
			g.Count += r.Count
		} else {
			yTotals[r.YKey] = &port.StatGroup{Key: r.YKey, Label: r.YLabel, Count: r.Count}
		}
	}
	top := func(m map[string]*port.StatGroup) ([]port.StatGroup, map[string]bool) {
		list := make([]port.StatGroup, 0, len(m))
		for _, g := range m {
			list = append(list, *g)
		}
		sort.Slice(list, func(i, j int) bool {
			if list[i].Count != list[j].Count {
				return list[i].Count > list[j].Count
			}
			return list[i].Label < list[j].Label
		})
		if len(list) > cfg.Limit {
			list = list[:cfg.Limit]
		}
		keep := map[string]bool{}
		for _, g := range list {
			keep[g.Key] = true
		}
		return list, keep
	}
	xs, xKeep := top(xTotals)
	ys, yKeep := top(yTotals)
	out := port.MatrixData{XAxis: cfg.XAxis, YAxis: cfg.YAxis, Total: total, XKeys: xs, YKeys: ys, Cells: []port.MatrixCell{}}
	for _, r := range rows {
		if xKeep[r.XKey] && yKeep[r.YKey] {
			out.Cells = append(out.Cells, port.MatrixCell{X: r.XKey, Y: r.YKey, Count: r.Count})
		}
	}
	sort.Slice(out.Cells, func(i, j int) bool {
		if out.Cells[i].X != out.Cells[j].X {
			return out.Cells[i].X < out.Cells[j].X
		}
		return out.Cells[i].Y < out.Cells[j].Y
	})
	return out, nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

type dayCount struct {
	Kind  string    `db:"kind"`
	Day   time.Time `db:"day"`
	Count int       `db:"count"`
}

// CreatedVsResolved counts, per UTC day of the last cfg.Days days, the issues created and resolved.
func (d *DataSource) CreatedVsResolved(ctx context.Context, viewer port.Viewer, cfg domain.CreatedVsResolvedConfig, today time.Time) (port.CreatedVsResolvedData, error) {
	end := time.Date(today.UTC().Year(), today.UTC().Month(), today.UTC().Day(), 0, 0, 0, 0, time.UTC)
	start := end.AddDate(0, 0, -(cfg.Days - 1))
	var rows []dayCount
	err := d.db.SelectContext(ctx, &rows, `
		WITH vis AS (
			SELECT i.created_at, i.resolved_at
			FROM issues i JOIN projects p ON p.id = i.project_id AND p.deleted_at IS NULL
			WHERE i.deleted_at IS NULL AND `+visible("p")+` AND ($3::uuid IS NULL OR i.project_id = $3::uuid)
			  AND (i.created_at >= $4::date OR i.resolved_at >= $4::date)
		)
		SELECT 'created' AS kind, (created_at AT TIME ZONE 'UTC')::date AS day, count(*) AS count FROM vis
		WHERE created_at >= $4::date AND created_at < ($5::date + 1) GROUP BY 2
		UNION ALL
		SELECT 'resolved', (resolved_at AT TIME ZONE 'UTC')::date, count(*) FROM vis
		WHERE resolved_at >= $4::date AND resolved_at < ($5::date + 1) GROUP BY 2`,
		viewer.UserID, viewer.PlatformAdmin, nullUUID(cfg.ProjectID), start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return port.CreatedVsResolvedData{}, internal("failed to compute created versus resolved", err)
	}
	created, resolved := map[string]int{}, map[string]int{}
	for _, r := range rows {
		key := r.Day.Format("2006-01-02")
		if r.Kind == "created" {
			created[key] = r.Count
		} else {
			resolved[key] = r.Count
		}
	}
	out := port.CreatedVsResolvedData{Days: cfg.Days, Series: make([]port.DayPoint, 0, cfg.Days)}
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		key := day.Format("2006-01-02")
		out.Series = append(out.Series, port.DayPoint{Date: key, Created: created[key], Resolved: resolved[key]})
		out.TotalCreated += created[key]
		out.TotalResolved += resolved[key]
	}
	out.CumulativeOpened = out.TotalCreated - out.TotalResolved
	return out, nil
}

type burnSprintRow struct {
	ID         uuid.UUID  `db:"id"`
	Name       string     `db:"name"`
	ProjectKey string     `db:"project_key"`
	StartDate  *time.Time `db:"start_date"`
	EndDate    *time.Time `db:"end_date"`
	StartedAt  *time.Time `db:"started_at"`
	CreatedAt  time.Time  `db:"created_at"`
}

type burnRow struct {
	Weight float64    `db:"weight"`
	DoneOn *time.Time `db:"done_on"`
}

const maxBurndownDays = 120

// Burndown computes the remaining points of a sprint per day. The sprint is the configured one, the active one
// of the configured project, or the first active sprint of the viewer's own projects.
func (d *DataSource) Burndown(ctx context.Context, viewer port.Viewer, cfg domain.BurndownConfig, today time.Time) (port.BurndownData, error) {
	var sp burnSprintRow
	err := d.db.GetContext(ctx, &sp, `
		SELECT s.id, s.name, p.key AS project_key, s.start_date, s.end_date, s.started_at, s.created_at
		FROM sprints s JOIN projects p ON p.id = s.project_id AND p.deleted_at IS NULL
		WHERE `+visible("p")+`
		  AND (CASE WHEN $3::uuid IS NOT NULL THEN s.id = $3::uuid ELSE s.status = 'active' END)
		  AND ($4::uuid IS NULL OR s.project_id = $4::uuid)
		ORDER BY EXISTS (SELECT 1 FROM project_members pm WHERE pm.project_id = p.id AND pm.user_id = $1::uuid) DESC,
		         s.end_date NULLS LAST, s.id LIMIT 1`,
		viewer.UserID, viewer.PlatformAdmin, nullUUID(cfg.SprintID), nullUUID(cfg.ProjectID))
	if errors.Is(err, sql.ErrNoRows) {
		return port.BurndownData{Points: []port.BurndownPoint{}}, nil
	}
	if err != nil {
		return port.BurndownData{}, internal("failed to find the sprint", err)
	}
	var issues []burnRow
	err = d.db.SelectContext(ctx, &issues, `
		SELECT COALESCE(i.story_points, 0)::float8 AS weight,
		       CASE WHEN corelog_issue_category(i.status_id, i.status) = 'done'
		            THEN (COALESCE(i.resolved_at, i.closed_at, i.updated_at) AT TIME ZONE 'UTC')::date END AS done_on
		FROM issues i WHERE i.sprint_id = $1 AND i.deleted_at IS NULL`, sp.ID)
	if err != nil {
		return port.BurndownData{}, internal("failed to read the sprint issues", err)
	}
	total := 0.0
	for _, i := range issues {
		total += i.Weight
	}
	unit := total == 0 // sprints without estimates count issues instead of points
	if unit {
		total = float64(len(issues))
	}
	utcDay := func(t time.Time) time.Time { return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC) }
	start := utcDay(sp.CreatedAt)
	switch {
	case sp.StartDate != nil:
		start = utcDay(*sp.StartDate)
	case sp.StartedAt != nil:
		start = utcDay(*sp.StartedAt)
	}
	end := start.AddDate(0, 0, 13)
	if sp.EndDate != nil {
		end = utcDay(*sp.EndDate)
	}
	if end.Before(start) {
		end = start
	}
	if end.Sub(start) > maxBurndownDays*24*time.Hour {
		end = start.AddDate(0, 0, maxBurndownDays)
	}
	now := utcDay(today)
	data := port.BurndownData{Total: total, Points: []port.BurndownPoint{}}
	data.Sprint = &port.BurndownSprint{ID: sp.ID, Name: sp.Name, ProjectKey: sp.ProjectKey, StartDate: dateStr(&start), EndDate: dateStr(&end)}
	span := end.Sub(start).Hours() / 24
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		ideal := total
		if span > 0 {
			ideal = total * (1 - (day.Sub(start).Hours()/24)/span)
		} else {
			ideal = 0
		}
		point := port.BurndownPoint{Date: day.Format("2006-01-02"), Ideal: round1(ideal)}
		if !day.After(now) {
			remaining := total
			for _, i := range issues {
				if i.DoneOn != nil && !utcDay(*i.DoneOn).After(day) {
					if unit {
						remaining--
					} else {
						remaining -= i.Weight
					}
				}
			}
			r := round1(remaining)
			point.Remaining = &r
		}
		data.Points = append(data.Points, point)
	}
	return data, nil
}

func dateStr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format("2006-01-02")
	return &s
}

func round1(f float64) float64 {
	if f < 0 {
		return 0
	}
	return float64(int(f*10+0.5)) / 10
}

type activityRow struct {
	ID       uuid.UUID `db:"id"`
	IssueID  uuid.UUID `db:"issue_id"`
	IssueKey string    `db:"issue_key"`
	Summary  string    `db:"summary"`
	Actor    string    `db:"actor"`
	Action   string    `db:"action"`
	Field    string    `db:"field_name"`
	Old      string    `db:"old_value"`
	New      string    `db:"new_value"`
	At       time.Time `db:"created_at"`
}

// Activity lists the latest activity on the visible issues.
func (d *DataSource) Activity(ctx context.Context, viewer port.Viewer, cfg domain.ActivityConfig) ([]port.ActivityItem, error) {
	var rows []activityRow
	err := d.db.SelectContext(ctx, &rows, `
		SELECT a.id, a.issue_id, p.key || '-' || i.key_number AS issue_key, i.summary, u.display_name AS actor, a.action,
		       COALESCE(a.field_name, '') AS field_name, COALESCE(a.old_value, '') AS old_value, COALESCE(a.new_value, '') AS new_value, a.created_at
		FROM activities a
		JOIN issues i ON i.id = a.issue_id AND i.deleted_at IS NULL
		JOIN projects p ON p.id = i.project_id AND p.deleted_at IS NULL AND `+visible("p")+`
		JOIN users u ON u.id = a.actor_id
		WHERE ($3::uuid IS NULL OR i.project_id = $3::uuid)
		ORDER BY a.created_at DESC, a.id LIMIT $4`,
		viewer.UserID, viewer.PlatformAdmin, nullUUID(cfg.ProjectID), cfg.Limit)
	if err != nil {
		return nil, internal("failed to read the activity", err)
	}
	out := make([]port.ActivityItem, len(rows))
	for i, r := range rows {
		out[i] = port.ActivityItem{ID: r.ID, IssueID: r.IssueID, IssueKey: r.IssueKey, Summary: r.Summary, Actor: r.Actor, Action: r.Action,
			Field: r.Field, OldValue: r.Old, NewValue: r.New, OccurredAt: r.At}
	}
	return out, nil
}

type projectRow struct {
	ID       uuid.UUID `db:"id"`
	Key      string    `db:"key"`
	Name     string    `db:"name"`
	Color    string    `db:"color"`
	ViewedAt time.Time `db:"viewed_at"`
}

// RecentProjects lists the projects the viewer opened last that they can still browse.
func (d *DataSource) RecentProjects(ctx context.Context, viewer port.Viewer, limit int) ([]port.ProjectItem, error) {
	var rows []projectRow
	err := d.db.SelectContext(ctx, &rows, `
		SELECT p.id, p.key, p.name, p.color, ri.viewed_at
		FROM recent_items ri
		JOIN projects p ON p.id = ri.entity_id AND p.deleted_at IS NULL AND `+visible("p")+`
		WHERE ri.user_id = $1::uuid AND ri.entity_type = 'project'
		ORDER BY ri.viewed_at DESC LIMIT $3`, viewer.UserID, viewer.PlatformAdmin, limit)
	if err != nil {
		return nil, internal("failed to read the recent projects", err)
	}
	out := make([]port.ProjectItem, len(rows))
	for i, r := range rows {
		out[i] = port.ProjectItem{ID: r.ID, Key: r.Key, Name: r.Name, Color: r.Color, ViewedAt: r.ViewedAt}
	}
	return out, nil
}

type favoriteRow struct {
	Type string    `db:"entity_type"`
	ID   uuid.UUID `db:"entity_id"`
	Name string    `db:"name"`
	At   time.Time `db:"created_at"`
}

// Favorites lists the starred entities the viewer can still see.
func (d *DataSource) Favorites(ctx context.Context, viewer port.Viewer, cfg domain.FavoritesConfig) ([]port.FavoriteItem, error) {
	var types any
	if len(cfg.Types) > 0 {
		types = textArray(cfg.Types)
	}
	var rows []favoriteRow
	err := d.db.SelectContext(ctx, &rows, `
		SELECT entity_type, entity_id, name, created_at FROM (
			SELECT f.entity_type, f.entity_id, p.name AS name, f.created_at FROM favorites f
			JOIN projects p ON f.entity_type = 'project' AND p.id = f.entity_id AND p.deleted_at IS NULL AND `+visible("p")+`
			WHERE f.user_id = $1::uuid
			UNION ALL
			SELECT f.entity_type, f.entity_id, p.key || '-' || i.key_number || ' ' || i.summary, f.created_at FROM favorites f
			JOIN issues i ON f.entity_type = 'issue' AND i.id = f.entity_id AND i.deleted_at IS NULL
			JOIN projects p ON p.id = i.project_id AND p.deleted_at IS NULL AND `+visible("p")+`
			WHERE f.user_id = $1::uuid
			UNION ALL
			SELECT f.entity_type, f.entity_id, b.name, f.created_at FROM favorites f
			JOIN boards b ON f.entity_type = 'board' AND b.id = f.entity_id
			JOIN projects p ON p.id = b.project_id AND p.deleted_at IS NULL AND `+visible("p")+`
			WHERE f.user_id = $1::uuid
			UNION ALL
			SELECT f.entity_type, f.entity_id, fl.name, f.created_at FROM favorites f
			JOIN filters fl ON f.entity_type = 'filter' AND fl.id = f.entity_id
			WHERE f.user_id = $1::uuid AND (fl.owner_id = $1::uuid OR EXISTS (
				SELECT 1 FROM filter_shares fs
				JOIN organization_members fm ON fm.organization_id = fl.organization_id AND fm.user_id = $1::uuid
				WHERE fs.filter_id = fl.id AND (
					(fs.share_type = 'user' AND fs.target_id = $1::uuid)
					OR (fs.share_type = 'team' AND fs.target_id IN (SELECT tm.team_id FROM team_members tm WHERE tm.user_id = $1::uuid))
					OR (fs.share_type = 'organization' AND fs.target_id = fl.organization_id))))
			UNION ALL
			SELECT f.entity_type, f.entity_id, ds.name, f.created_at FROM favorites f
			JOIN dashboards ds ON f.entity_type = 'dashboard' AND ds.id = f.entity_id
			WHERE f.user_id = $1::uuid AND (ds.owner_id = $1::uuid OR EXISTS (
				SELECT 1 FROM dashboard_shares dsh
				JOIN organization_members dm ON dm.organization_id = ds.organization_id AND dm.user_id = $1::uuid
				WHERE dsh.dashboard_id = ds.id AND (
					(dsh.share_type = 'user' AND dsh.target_id = $1::uuid)
					OR (dsh.share_type = 'team' AND dsh.target_id IN (SELECT tm.team_id FROM team_members tm WHERE tm.user_id = $1::uuid))
					OR (dsh.share_type = 'organization' AND dsh.target_id = ds.organization_id))))
		) fav WHERE ($3::text[] IS NULL OR entity_type = ANY($3::text[]))
		ORDER BY created_at DESC, entity_id LIMIT $4`, viewer.UserID, viewer.PlatformAdmin, types, cfg.Limit)
	if err != nil {
		return nil, internal("failed to read the favorites", err)
	}
	out := make([]port.FavoriteItem, len(rows))
	for i, r := range rows {
		out[i] = port.FavoriteItem{Type: r.Type, ID: r.ID, Name: r.Name, Link: favoriteLink(r.Type, r.ID), StarredAt: r.At}
	}
	return out, nil
}

func favoriteLink(kind string, id uuid.UUID) string {
	switch kind {
	case "project":
		return "/projects/" + id.String()
	case "filter":
		return "/filters/" + id.String()
	case "board":
		return "/boards/" + id.String()
	case "dashboard":
		return "/dashboards/" + id.String()
	}
	return "/issues/" + id.String()
}
