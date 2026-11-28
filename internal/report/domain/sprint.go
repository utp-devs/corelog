package domain

import (
	"sort"
	"time"

	"github.com/google/uuid"
)

// Scope event kinds recorded when the scope of a sprint changes.
const (
	EventAdded         = "added"
	EventRemoved       = "removed"
	EventMovedOut      = "moved_out"
	EventPointsChanged = "points_changed"
)

const defaultSprintDays = 14

// Metrics of the burndown and burnup.
const (
	MetricPoints = "points"
	MetricIssues = "issues"
)

// SprintInfo is a sprint with its lifecycle dates (calendar dates for the planned window, instants for
// start and completion).
type SprintInfo struct {
	ID          uuid.UUID
	Name        string
	Status      string // planned, active, closed
	StartDate   *time.Time
	EndDate     *time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
	Committed   *float64
	Completed   *float64
}

// ScopeEvent is a row of the scope history of a sprint, ordered by time.
type ScopeEvent struct {
	IssueID    uuid.UUID
	Kind       string
	Points     *float64
	At         time.Time
	AfterStart bool
}

func weightOf(metric string, points *float64) float64 {
	if metric == MetricIssues {
		return 1
	}
	if points == nil {
		return 0
	}
	return *points
}

// sprintDays lists the calendar days of the sprint chart and whether the sprint has a planned end.
func sprintDays(s SprintInfo, loc *time.Location, now time.Time) (days []time.Time, hasEnd bool, plannedDays int) {
	if s.Status == "planned" {
		return nil, false, 0
	}
	var first time.Time
	switch {
	case s.StartDate != nil:
		first = civilDate(*s.StartDate)
	case s.StartedAt != nil:
		first = civil(*s.StartedAt, loc)
	default:
		return nil, false, 0
	}
	plannedEnd := first.AddDate(0, 0, defaultSprintDays)
	if s.EndDate != nil {
		hasEnd = true
		plannedEnd = civilDate(*s.EndDate)
	}
	last := plannedEnd
	switch s.Status {
	case "closed":
		if s.CompletedAt != nil {
			last = civil(*s.CompletedAt, loc)
		}
	case "active":
		if today := civil(now, loc); today.Before(last) {
			last = today
		}
	}
	if last.Before(first) {
		last = first
	}
	plannedDays = int(plannedEnd.Sub(first).Hours()/hoursPerDay) + 1
	if plannedDays < 1 {
		plannedDays = 1
	}
	for d := first; !d.After(last) && len(days) < MaxRangeDays; d = d.AddDate(0, 0, 1) {
		days = append(days, d)
	}
	return days, hasEnd, plannedDays
}

// civilDate keeps the Y-M-D of a DATE column read as UTC midnight.
func civilDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

type scopeState struct {
	in bool
	w  float64
}

// BuildBurn computes the burndown (burnup false) or the burnup (burnup true) of a sprint from its scope
// events and the status history of its issues. Only issues present in issues (live and, when a filter is
// given, matching it) count. Days are local days of loc; the scope of a day is what stood at its end.
func BuildBurn(s SprintInfo, events []ScopeEvent, issues []IssueRow, history []StatusChange, metric string, burnup bool, loc *time.Location, now time.Time) *Report {
	kind := KindBurndown
	if burnup {
		kind = KindBurnup
	}
	r := newReport(kind, col("date", "Fecha", "date"), col("scope", "Alcance", "number"),
		col("completed", "Completado", "number"), col("remaining", "Restante", "number"), col("ideal", "Ideal", "number"))
	r.Summary["metric"] = metric
	r.Summary["sprint"] = map[string]any{"id": s.ID.String(), "name": s.Name, "status": s.Status}
	r.Summary["committed"], r.Summary["scope"], r.Summary["completed"], r.Summary["remaining"], r.Summary["scope_change"] = 0.0, 0.0, 0.0, 0.0, 0.0

	days, hasEnd, planned := sprintDays(s, loc, now)
	if len(days) == 0 {
		return r
	}
	rows := make(map[uuid.UUID]IssueRow, len(issues))
	for _, i := range issues {
		rows[i.ID] = i
	}
	hist := historyByIssue(history)

	committed := 0.0
	committedState := map[uuid.UUID]float64{}
	for _, e := range events {
		if _, ok := rows[e.IssueID]; !ok {
			continue
		}
		if e.Kind == EventAdded && !e.AfterStart {
			committedState[e.IssueID] = weightOf(metric, e.Points)
		}
	}
	for _, w := range committedState {
		committed += w
	}

	state := map[uuid.UUID]*scopeState{}
	next := 0
	var scopeS, doneS, remS, idealS []Point
	var scope, completed, remaining float64
	for i, day := range days {
		eod := startOf(day.AddDate(0, 0, 1), loc).Add(-time.Nanosecond)
		for next < len(events) && !events[next].At.After(eod) {
			e := events[next]
			next++
			st := state[e.IssueID]
			if st == nil {
				st = &scopeState{}
				state[e.IssueID] = st
			}
			switch e.Kind {
			case EventAdded:
				st.in, st.w = true, weightOf(metric, e.Points)
			case EventRemoved, EventMovedOut:
				st.in = false
			case EventPointsChanged:
				if st.in {
					st.w = weightOf(metric, e.Points)
				}
			}
		}
		scope, remaining = 0, 0
		for id, st := range state {
			row, ok := rows[id]
			if !ok || !st.in {
				continue
			}
			scope += st.w
			if categoryAt(hist[id], row, eod) != categoryDone {
				remaining += st.w
			}
		}
		completed = scope - remaining
		label := day.Format(dateLayout)
		scopeS = append(scopeS, Point{label, scope})
		doneS = append(doneS, Point{label, completed})
		remS = append(remS, Point{label, remaining})
		var ideal any
		if hasEnd {
			v := 0.0
			if i < planned {
				v = committed
				if planned > 1 {
					v = committed * float64(planned-1-i) / float64(planned-1)
				}
			}
			idealS = append(idealS, Point{label, round2(v)})
			ideal = round2(v)
		}
		r.Table.Rows = append(r.Table.Rows, []any{label, round2(scope), round2(completed), round2(remaining), ideal})
	}
	if burnup {
		r.Series = append(r.Series, Series{"Alcance", scopeS}, Series{"Completado", doneS})
	} else {
		r.Series = append(r.Series, Series{"Restante", remS})
		if hasEnd {
			r.Series = append(r.Series, Series{"Ideal", idealS})
		}
	}
	r.Summary["committed"], r.Summary["scope"] = round2(committed), round2(scope)
	r.Summary["completed"], r.Summary["remaining"] = round2(completed), round2(remaining)
	r.Summary["scope_change"] = round2(scope - committed)
	return r
}

// BuildVelocity compares committed and completed work across closed sprints, oldest first.
func BuildVelocity(sprints []SprintInfo) *Report {
	r := newReport(KindVelocity, col("sprint", "Sprint", "text"), col("committed", "Comprometido", "number"),
		col("completed", "Completado", "number"), col("ratio", "Cumplimiento", "percent"))
	list := append([]SprintInfo(nil), sprints...)
	sort.SliceStable(list, func(i, j int) bool {
		a, b := list[i].CompletedAt, list[j].CompletedAt
		if a == nil || b == nil {
			return a != nil && b == nil
		}
		return a.Before(*b)
	})
	var com, done []Point
	var sumC, sumD float64
	for _, s := range list {
		c, d := 0.0, 0.0
		if s.Committed != nil {
			c = *s.Committed
		}
		if s.Completed != nil {
			d = *s.Completed
		}
		com, done = append(com, Point{s.Name, c}), append(done, Point{s.Name, d})
		sumC, sumD = sumC+c, sumD+d
		var ratio any
		if c > 0 {
			ratio = round2(d / c * 100)
		}
		r.Table.Rows = append(r.Table.Rows, []any{s.Name, round2(c), round2(d), ratio})
	}
	n := float64(len(list))
	r.Summary["sprints"] = len(list)
	r.Summary["average_committed"], r.Summary["average_completed"] = 0.0, 0.0
	r.Summary["commitment_ratio"] = nil
	if len(list) > 0 {
		r.Series = append(r.Series, Series{"Comprometido", com}, Series{"Completado", done})
		r.Summary["average_committed"], r.Summary["average_completed"] = round2(sumC/n), round2(sumD/n)
		if sumC > 0 {
			r.Summary["commitment_ratio"] = round2(sumD / sumC * 100)
		}
	}
	return r
}

// SprintItem is an issue of the sprint summary with its outcome: completed, incomplete or removed.
type SprintItem struct {
	Key     string
	Summary string
	Points  *float64
	Outcome string
	Added   bool
}

// SprintSummary is the sprint report computed by the sprint module, as the report module needs it.
type SprintSummary struct {
	Name       string
	Status     string
	Committed  float64
	Completed  float64
	Incomplete float64
	Added      float64
	Removed    float64
	Items      []SprintItem
}

var outcomeLabel = map[string]string{"completed": "Completada", "incomplete": "Incompleta", "removed": "Retirada"}

// BuildSprintSummary shapes the sprint report into the uniform payload.
func BuildSprintSummary(s SprintSummary) *Report {
	r := newReport(KindSprintReport, col("key", "Clave", "text"), col("summary", "Resumen", "text"),
		col("outcome", "Resultado", "text"), col("points", "Puntos", "number"), col("added", "Añadida tras el inicio", "text"))
	r.Series = append(r.Series, Series{"Puntos", []Point{
		{"Comprometido", s.Committed}, {"Completado", s.Completed}, {"Incompleto", s.Incomplete},
		{"Añadido", s.Added}, {"Retirado", s.Removed},
	}})
	for _, it := range s.Items {
		var pts any
		if it.Points != nil {
			pts = *it.Points
		}
		added := "No"
		if it.Added {
			added = "Sí"
		}
		r.Table.Rows = append(r.Table.Rows, []any{it.Key, it.Summary, outcomeLabel[it.Outcome], pts, added})
	}
	r.Summary["sprint"] = map[string]any{"name": s.Name, "status": s.Status}
	r.Summary["committed"], r.Summary["completed"], r.Summary["incomplete"] = s.Committed, s.Completed, s.Incomplete
	r.Summary["added"], r.Summary["removed"] = s.Added, s.Removed
	r.Summary["completion_percent"] = nil
	if total := s.Completed + s.Incomplete; total > 0 {
		r.Summary["completion_percent"] = round2(s.Completed / total * 100)
	}
	return r
}
