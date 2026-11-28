package domain

import (
	"sort"
	"time"
)

const controlWindow = 10

type resolvedIssue struct {
	issue IssueRow
	start time.Time
	days  float64
}

func resolvedIn(p Params, issues []IssueRow) []IssueRow {
	var out []IssueRow
	for _, i := range issues {
		if i.ResolvedAt != nil && !i.ResolvedAt.Before(p.Lo) && i.ResolvedAt.Before(p.Hi) {
			out = append(out, i)
		}
	}
	sort.SliceStable(out, func(a, b int) bool {
		if !out[a].ResolvedAt.Equal(*out[b].ResolvedAt) {
			return out[a].ResolvedAt.Before(*out[b].ResolvedAt)
		}
		return out[a].Key < out[b].Key
	})
	return out
}

func daysBetween(from, to time.Time) float64 {
	d := to.Sub(from).Hours() / hoursPerDay
	if d < 0 {
		return 0
	}
	return round2(d)
}

// BuildControl computes the cycle time (first entry into "in progress", or creation when there is none,
// until resolution) of the issues resolved in the range, with a trailing rolling average and standard
// deviation over the last 10 resolutions.
func BuildControl(p Params, issues []IssueRow, history []StatusChange) *Report {
	r := newReport(KindControl, col("key", "Clave", "text"), col("summary", "Resumen", "text"), col("started", "Inicio", "date"),
		col("resolved", "Resolución", "date"), col("days", "Días", "number"))
	hist := historyByIssue(history)
	var list []resolvedIssue
	for _, i := range resolvedIn(p, issues) {
		start := i.CreatedAt
		for _, c := range hist[i.ID] {
			if c.Category == categoryInProgress {
				start = c.At
				break
			}
		}
		if start.After(*i.ResolvedAt) {
			start = i.CreatedAt
		}
		list = append(list, resolvedIssue{i, start, daysBetween(start, *i.ResolvedAt)})
	}
	var cycle, avg, upper, lower []Point
	var values []float64
	for idx, it := range list {
		label := dayKey(*it.issue.ResolvedAt, p.Loc)
		values = append(values, it.days)
		from := idx + 1 - controlWindow
		if from < 0 {
			from = 0
		}
		window := values[from : idx+1]
		m, sd := mean(window), stdDev(window)
		low := m - sd
		if low < 0 {
			low = 0
		}
		cycle = append(cycle, Point{label, it.days})
		avg, upper, lower = append(avg, Point{label, round2(m)}), append(upper, Point{label, round2(m + sd)}), append(lower, Point{label, round2(low)})
		r.Table.Rows = append(r.Table.Rows, []any{it.issue.Key, it.issue.Summary, dayKey(it.start, p.Loc), label, it.days})
	}
	if len(list) > 0 {
		r.Series = append(r.Series, Series{"Tiempo de ciclo (días)", cycle}, Series{"Media móvil", avg},
			Series{"Límite superior", upper}, Series{"Límite inferior", lower})
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	r.Summary["count"] = len(values)
	r.Summary["average_days"], r.Summary["median_days"] = round2(mean(values)), round2(median(sorted))
	r.Summary["std_dev_days"], r.Summary["p85_days"] = round2(stdDev(values)), round2(percentile(sorted, 0.85))
	r.Summary["rolling_window"] = controlWindow
	return r
}

type groupStat struct {
	name string
	days []float64
}

// BuildResolutionTime summarizes the time from creation to resolution of the issues resolved in the
// range, grouped by issue type or priority.
func BuildResolutionTime(p Params, issues []IssueRow) *Report {
	label := "Tipo"
	if p.GroupBy == "priority" {
		label = "Prioridad"
	}
	r := newReport(KindResolutionTime, col("group", label, "text"), col("count", "Resueltas", "number"),
		col("average_days", "Promedio (días)", "number"), col("median_days", "Mediana (días)", "number"), col("p90_days", "P90 (días)", "number"))
	groups := map[string]*groupStat{}
	var order []string
	var all []float64
	for _, i := range resolvedIn(p, issues) {
		name := i.TypeName
		if p.GroupBy == "priority" {
			name = i.PriorityName
		}
		g := groups[name]
		if g == nil {
			g = &groupStat{name: name}
			groups[name] = g
			order = append(order, name)
		}
		d := daysBetween(i.CreatedAt, *i.ResolvedAt)
		g.days = append(g.days, d)
		all = append(all, d)
	}
	sort.Strings(order)
	var avg, med []Point
	for _, name := range order {
		g := groups[name]
		sorted := append([]float64(nil), g.days...)
		sort.Float64s(sorted)
		a, m, p90 := round2(mean(sorted)), round2(median(sorted)), round2(percentile(sorted, 0.9))
		avg, med = append(avg, Point{name, a}), append(med, Point{name, m})
		r.Table.Rows = append(r.Table.Rows, []any{name, len(sorted), a, m, p90})
	}
	if len(order) > 0 {
		r.Series = append(r.Series, Series{"Promedio (días)", avg}, Series{"Mediana (días)", med})
	}
	sort.Float64s(all)
	r.Summary["count"] = len(all)
	r.Summary["average_days"], r.Summary["median_days"], r.Summary["p90_days"] = round2(mean(all)), round2(median(all)), round2(percentile(all, 0.9))
	return r
}
