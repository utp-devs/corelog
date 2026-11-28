package domain

import (
	"time"
)

var categoryLabels = []struct{ key, label string }{
	{categoryTodo, "Por hacer"}, {categoryInProgress, "En curso"}, {categoryDone, "Hecho"},
}

// BuildCumulativeFlow counts, for every local day of the range, the issues in each status category at the
// end of that day. An issue counts from the day it was created.
func BuildCumulativeFlow(p Params, issues []IssueRow, history []StatusChange) *Report {
	r := newReport(KindCumulativeFlow, col("date", "Fecha", "date"), col("todo", "Por hacer", "number"),
		col("in_progress", "En curso", "number"), col("done", "Hecho", "number"), col("total", "Total", "number"))
	n := p.Days()
	counts := make([][3]int, n)
	hist := historyByIssue(history)
	for _, issue := range issues {
		changes := hist[issue.ID]
		for d := 0; d < n; d++ {
			eod := startOf(p.FromDate.AddDate(0, 0, d+1), p.Loc).Add(-time.Nanosecond)
			if issue.CreatedAt.After(eod) {
				continue
			}
			switch categoryAt(changes, issue, eod) {
			case categoryDone:
				counts[d][2]++
			case categoryInProgress:
				counts[d][1]++
			default:
				counts[d][0]++
			}
		}
	}
	series := make([][]Point, 3)
	wip := 0.0
	for d := 0; d < n; d++ {
		label := p.FromDate.AddDate(0, 0, d).Format(dateLayout)
		total := counts[d][0] + counts[d][1] + counts[d][2]
		for c := 0; c < 3; c++ {
			series[c] = append(series[c], Point{label, float64(counts[d][c])})
		}
		wip += float64(counts[d][1])
		r.Table.Rows = append(r.Table.Rows, []any{label, counts[d][0], counts[d][1], counts[d][2], total})
	}
	for c, def := range categoryLabels {
		r.Series = append(r.Series, Series{def.label, series[c]})
	}
	last := counts[n-1]
	r.Summary["todo"], r.Summary["in_progress"], r.Summary["done"] = last[0], last[1], last[2]
	r.Summary["total"] = last[0] + last[1] + last[2]
	r.Summary["average_in_progress"] = round2(wip / float64(n))
	return r
}

// bucketKey is the label of the period (day, week starting Monday, or month) that contains the date.
func bucketKey(date time.Time, interval string) string {
	switch interval {
	case "week":
		offset := (int(date.Weekday()) + 6) % 7
		return date.AddDate(0, 0, -offset).Format(dateLayout)
	case "month":
		return date.Format("2006-01")
	}
	return date.Format(dateLayout)
}

// BuildCreatedVsResolved counts the issues created and resolved in every period of the range and the
// cumulative difference between both.
func BuildCreatedVsResolved(p Params, issues []IssueRow) *Report {
	r := newReport(KindCreatedVsRes, col("period", "Periodo", "text"), col("created", "Creadas", "number"),
		col("resolved", "Resueltas", "number"), col("net", "Neto", "number"), col("cumulative", "Acumulado", "number"))
	var order []string
	created, resolved := map[string]int{}, map[string]int{}
	for d := p.FromDate; !d.After(p.ToDate); d = d.AddDate(0, 0, 1) {
		k := bucketKey(d, p.Interval)
		if len(order) == 0 || order[len(order)-1] != k {
			order = append(order, k)
		}
	}
	inRange := func(t time.Time) bool { return !t.Before(p.Lo) && t.Before(p.Hi) }
	totalCreated, totalResolved := 0, 0
	for _, i := range issues {
		if inRange(i.CreatedAt) {
			created[bucketKey(civil(i.CreatedAt, p.Loc), p.Interval)]++
			totalCreated++
		}
		if i.ResolvedAt != nil && inRange(*i.ResolvedAt) {
			resolved[bucketKey(civil(*i.ResolvedAt, p.Loc), p.Interval)]++
			totalResolved++
		}
	}
	var cs, rs, ds []Point
	cum := 0
	for _, k := range order {
		c, v := created[k], resolved[k]
		cum += c - v
		cs, rs, ds = append(cs, Point{k, float64(c)}), append(rs, Point{k, float64(v)}), append(ds, Point{k, float64(cum)})
		r.Table.Rows = append(r.Table.Rows, []any{k, c, v, c - v, cum})
	}
	r.Series = append(r.Series, Series{"Creadas", cs}, Series{"Resueltas", rs}, Series{"Diferencia acumulada", ds})
	r.Summary["created"], r.Summary["resolved"], r.Summary["net"] = totalCreated, totalResolved, totalCreated-totalResolved
	return r
}
