package domain

import (
	"time"

	"github.com/google/uuid"
)

// VersionInfo is a release of the project. Dates are calendar dates.
type VersionInfo struct {
	ID          uuid.UUID
	Name        string
	Description string
	Status      string // unreleased, released, archived
	StartDate   *time.Time
	ReleaseDate *time.Time
}

// BuildVersion reports the progress of the issues that fix the version.
func BuildVersion(v VersionInfo, issues []IssueRow, loc *time.Location, now time.Time) *Report {
	r := newReport(KindVersion, col("key", "Clave", "text"), col("summary", "Resumen", "text"), col("status", "Estado", "text"),
		col("category", "Categoría", "text"), col("points", "Puntos", "number"), col("assignee", "Responsable", "text"))
	counts := map[string]int{}
	points := map[string]float64{}
	totalPoints := 0.0
	for _, i := range issues {
		cat := i.Category
		if cat != categoryDone && cat != categoryInProgress {
			cat = categoryTodo
		}
		counts[cat]++
		var pts any
		if i.Points != nil {
			points[cat] += *i.Points
			totalPoints += *i.Points
			pts = *i.Points
		}
		assignee := i.AssigneeName
		if i.AssigneeID == nil {
			assignee = unassignedLabel
		}
		r.Table.Rows = append(r.Table.Rows, []any{i.Key, i.Summary, i.StatusName, categoryName(cat), pts, assignee})
	}
	var byCount, byPoints []Point
	for _, def := range categoryLabels {
		byCount = append(byCount, Point{def.label, float64(counts[def.key])})
		byPoints = append(byPoints, Point{def.label, round2(points[def.key])})
	}
	r.Series = append(r.Series, Series{"Incidencias", byCount}, Series{"Puntos", byPoints})
	total := len(issues)
	r.Summary["version"] = map[string]any{"id": v.ID.String(), "name": v.Name, "status": v.Status, "description": v.Description}
	r.Summary["total"], r.Summary["done"] = total, counts[categoryDone]
	r.Summary["in_progress"], r.Summary["todo"] = counts[categoryInProgress], counts[categoryTodo]
	r.Summary["percent_done"] = ratio(float64(counts[categoryDone]), float64(total))
	r.Summary["points_total"], r.Summary["points_done"] = round2(totalPoints), round2(points[categoryDone])
	r.Summary["percent_points_done"] = ratio(points[categoryDone], totalPoints)
	r.Summary["release_date"], r.Summary["days_to_release"], r.Summary["overdue"] = nil, nil, false
	if v.StartDate != nil {
		r.Summary["start_date"] = v.StartDate.Format(dateLayout)
	}
	if v.ReleaseDate != nil {
		release := civilDate(*v.ReleaseDate)
		days := int(release.Sub(civil(now, loc)).Hours() / hoursPerDay)
		r.Summary["release_date"] = release.Format(dateLayout)
		r.Summary["days_to_release"] = days
		r.Summary["overdue"] = v.Status == "unreleased" && days < 0
	}
	return r
}

func ratio(part, whole float64) float64 {
	if whole <= 0 {
		return 0
	}
	return round2(part / whole * 100)
}

func categoryName(c string) string {
	for _, def := range categoryLabels {
		if def.key == c {
			return def.label
		}
	}
	return c
}
