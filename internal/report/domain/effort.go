package domain

import (
	"sort"
	"time"

	"github.com/google/uuid"
)

// WorklogRow is a live worklog with the estimates of its issue.
type WorklogRow struct {
	IssueID      uuid.UUID
	IssueKey     string
	Summary      string
	AuthorID     uuid.UUID
	AuthorName   string
	StartedAt    time.Time
	Seconds      int64
	OriginalEst  *int64
	RemainingEst *int64
}

type effortGroup struct {
	name    string
	est     int64
	rem     int64
	spent   int64
	entries int
	issues  map[uuid.UUID]bool
}

func accuracy(spent, est int64) any {
	if est <= 0 {
		return nil
	}
	return round2(float64(spent) / float64(est) * 100)
}

// BuildTimeTracking compares the estimate with the time logged inside the range, per issue or per person.
// The accuracy is spent divided by the original estimate; it is null when there is no estimate.
func BuildTimeTracking(p Params, rows []WorklogRow) *Report {
	byUser := p.GroupBy == "user"
	var r *Report
	if byUser {
		r = newReport(KindTimeTracking, col("user", "Persona", "text"), col("issues", "Incidencias", "number"),
			col("entries", "Registros", "number"), col("spent_hours", "Dedicado (h)", "number"))
	} else {
		r = newReport(KindTimeTracking, col("key", "Clave", "text"), col("summary", "Resumen", "text"),
			col("estimate_hours", "Estimado (h)", "number"), col("spent_hours", "Dedicado (h)", "number"),
			col("remaining_hours", "Restante (h)", "number"), col("accuracy", "Precisión", "percent"))
	}
	groups := map[string]*effortGroup{}
	summaries := map[string]string{}
	var order []string
	var totalSpent, totalEst int64
	issuesSeen := map[uuid.UUID]bool{}
	users := map[uuid.UUID]bool{}
	for _, w := range rows {
		key, name := w.IssueID.String(), w.IssueKey
		if byUser {
			key, name = w.AuthorID.String(), w.AuthorName
		}
		g := groups[key]
		if g == nil {
			g = &effortGroup{name: name, issues: map[uuid.UUID]bool{}}
			groups[key] = g
			summaries[key] = w.Summary
			order = append(order, key)
			if !byUser && w.OriginalEst != nil {
				g.est = *w.OriginalEst
			}
			if !byUser && w.RemainingEst != nil {
				g.rem = *w.RemainingEst
			}
		}
		g.spent += w.Seconds
		g.entries++
		g.issues[w.IssueID] = true
		totalSpent += w.Seconds
		users[w.AuthorID] = true
		if !issuesSeen[w.IssueID] {
			issuesSeen[w.IssueID] = true
			if w.OriginalEst != nil {
				totalEst += *w.OriginalEst
			}
		}
	}
	sort.SliceStable(order, func(i, j int) bool {
		a, b := groups[order[i]], groups[order[j]]
		if a.spent != b.spent {
			return a.spent > b.spent
		}
		return a.name < b.name
	})
	var estS, spentS []Point
	for _, k := range order {
		g := groups[k]
		if byUser {
			spentS = append(spentS, Point{g.name, hours(g.spent)})
			r.Table.Rows = append(r.Table.Rows, []any{g.name, len(g.issues), g.entries, hours(g.spent)})
			continue
		}
		estS, spentS = append(estS, Point{g.name, hours(g.est)}), append(spentS, Point{g.name, hours(g.spent)})
		r.Table.Rows = append(r.Table.Rows, []any{g.name, summaries[k], hours(g.est), hours(g.spent), hours(g.rem), accuracy(g.spent, g.est)})
	}
	if len(order) > 0 {
		if byUser {
			r.Series = append(r.Series, Series{"Dedicado (h)", spentS})
		} else {
			r.Series = append(r.Series, Series{"Estimado (h)", estS}, Series{"Dedicado (h)", spentS})
		}
	}
	r.Summary["spent_hours"], r.Summary["estimate_hours"] = hours(totalSpent), hours(totalEst)
	r.Summary["accuracy_percent"] = accuracy(totalSpent, totalEst)
	r.Summary["entries"], r.Summary["issues"], r.Summary["users"] = len(rows), len(issuesSeen), len(users)
	return r
}

type loadGroup struct {
	name       string
	open       int
	inProgress int
	points     float64
	remaining  int64
	unestimate int
}

// BuildWorkload lists, per assignee, the open issues, the story points and the remaining estimate.
func BuildWorkload(issues []IssueRow) *Report {
	r := newReport(KindWorkload, col("assignee", "Persona", "text"), col("open", "Abiertas", "number"),
		col("in_progress", "En curso", "number"), col("points", "Puntos", "number"),
		col("remaining_hours", "Estimación restante (h)", "number"), col("unestimated", "Sin estimar", "number"))
	groups := map[string]*loadGroup{}
	var order []string
	unassigned, total := 0, 0
	var remaining int64
	for _, i := range issues {
		if i.Category == categoryDone {
			continue
		}
		key, name := "", unassignedLabel
		if i.AssigneeID != nil {
			key, name = i.AssigneeID.String(), i.AssigneeName
		} else {
			unassigned++
		}
		g := groups[key]
		if g == nil {
			g = &loadGroup{name: name}
			groups[key] = g
			order = append(order, key)
		}
		g.open++
		total++
		if i.Category == categoryInProgress {
			g.inProgress++
		}
		if i.Points != nil {
			g.points += *i.Points
		}
		if i.RemainingEst != nil {
			g.remaining += *i.RemainingEst
			remaining += *i.RemainingEst
		} else {
			g.unestimate++
		}
	}
	sort.SliceStable(order, func(a, b int) bool {
		x, y := groups[order[a]], groups[order[b]]
		if x.open != y.open {
			return x.open > y.open
		}
		return x.name < y.name
	})
	var openS, remS []Point
	for _, k := range order {
		g := groups[k]
		openS, remS = append(openS, Point{g.name, float64(g.open)}), append(remS, Point{g.name, hours(g.remaining)})
		r.Table.Rows = append(r.Table.Rows, []any{g.name, g.open, g.inProgress, round2(g.points), hours(g.remaining), g.unestimate})
	}
	if len(order) > 0 {
		r.Series = append(r.Series, Series{"Incidencias abiertas", openS}, Series{"Estimación restante (h)", remS})
	}
	r.Summary["open"], r.Summary["unassigned"], r.Summary["remaining_hours"] = total, unassigned, hours(remaining)
	r.Summary["assignees"] = len(order) - boolInt(unassigned > 0)
	return r
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
