// Package domain holds the pure computations of the project reports: they take rows already read from
// the history tables and return a uniform report (series, table and summary). Nothing here touches the
// database, the clock or the network.
package domain

import (
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
)

// Kind names a report.
type Kind string

// Report kinds.
const (
	KindBurndown       Kind = "burndown"
	KindBurnup         Kind = "burnup"
	KindVelocity       Kind = "velocity"
	KindCumulativeFlow Kind = "cumulative-flow"
	KindControl        Kind = "control"
	KindCreatedVsRes   Kind = "created-vs-resolved"
	KindResolutionTime Kind = "resolution-time"
	KindTimeTracking   Kind = "time-tracking"
	KindWorkload       Kind = "workload"
	KindVersion        Kind = "version"
	KindSprintReport   Kind = "sprint-report"
)

const (
	categoryTodo       = "todo"
	categoryInProgress = "in_progress"
	categoryDone       = "done"
	unassignedLabel    = "Sin asignar"
	secondsPerHour     = 3600.0
	hoursPerDay        = 24.0
	dateLayout         = "2006-01-02"
)

// Descriptor documents one report for the gallery.
type Descriptor struct {
	Kind        Kind
	Title       string
	Description string
	// Params lists the query parameters the report understands.
	Params []string
}

// Catalog lists the available reports in display order.
func Catalog() []Descriptor {
	return []Descriptor{
		{KindBurndown, "Burndown del sprint", "Trabajo restante día a día frente a la línea ideal, con los cambios de alcance.", []string{"sprint", "group_by", "filter"}},
		{KindBurnup, "Burnup del sprint", "Alcance total y trabajo completado día a día.", []string{"sprint", "group_by", "filter"}},
		{KindVelocity, "Velocidad", "Puntos comprometidos y completados en los últimos sprints cerrados.", []string{"limit"}},
		{KindCumulativeFlow, "Flujo acumulado", "Incidencias por categoría de estado y día.", []string{"from", "to", "filter"}},
		{KindControl, "Gráfico de control", "Tiempo de ciclo de las incidencias resueltas, con media móvil y desviación estándar.", []string{"from", "to", "filter"}},
		{KindCreatedVsRes, "Creadas frente a resueltas", "Incidencias creadas y resueltas por día, semana o mes.", []string{"from", "to", "interval", "filter"}},
		{KindResolutionTime, "Tiempo de resolución", "Promedio, mediana y percentil 90 del tiempo hasta resolver, por tipo o prioridad.", []string{"from", "to", "group_by", "filter"}},
		{KindTimeTracking, "Seguimiento de tiempo", "Estimado frente a dedicado por incidencia o persona, con la precisión de la estimación.", []string{"from", "to", "group_by", "sprint", "filter"}},
		{KindWorkload, "Carga de trabajo", "Incidencias abiertas y estimación restante por persona.", []string{"filter"}},
		{KindVersion, "Versión", "Progreso y detalle de las incidencias de una versión.", []string{"version", "filter"}},
		{KindSprintReport, "Resumen del sprint", "Comprometido, completado, añadido y retirado de un sprint.", []string{"sprint"}},
	}
}

// ParseKind validates a report name.
func ParseKind(raw string) (Kind, bool) {
	for _, d := range Catalog() {
		if string(d.Kind) == raw {
			return d.Kind, true
		}
	}
	return "", false
}

// TitleOf returns the title of a kind.
func TitleOf(k Kind) string {
	for _, d := range Catalog() {
		if d.Kind == k {
			return d.Title
		}
	}
	return string(k)
}

// Point is one value of a series; X is a date, a period or a label.
type Point struct {
	X string
	Y float64
}

// Series is a named list of points.
type Series struct {
	Name   string
	Points []Point
}

// Column describes a table column. Type is one of text, number, percent, date.
type Column struct {
	Key   string
	Label string
	Type  string
}

// Table is the tabular form of a report: every row has one value per column.
type Table struct {
	Columns []Column
	Rows    [][]any
}

// Report is the uniform result of every computation.
type Report struct {
	Kind    Kind
	Title   string
	Series  []Series
	Table   Table
	Summary map[string]any
}

func newReport(k Kind, cols ...Column) *Report {
	return &Report{Kind: k, Title: TitleOf(k), Series: []Series{}, Table: Table{Columns: cols, Rows: [][]any{}}, Summary: map[string]any{}}
}

func col(key, label, typ string) Column { return Column{Key: key, Label: label, Type: typ} }

// ProjectInfo is what the reports need to know about a project.
type ProjectInfo struct {
	ID       uuid.UUID
	Key      string
	Name     string
	Timezone string
}

// Location resolves the organization time zone, falling back to UTC for an unknown name.
func (p ProjectInfo) Location() *time.Location {
	if p.Timezone == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(p.Timezone)
	if err != nil {
		return time.UTC
	}
	return loc
}

// IssueRow is a live issue of the project as the reports see it. ResolvedAt is set when the issue is done.
type IssueRow struct {
	ID           uuid.UUID
	Key          string
	Summary      string
	TypeName     string
	PriorityName string
	AssigneeID   *uuid.UUID
	AssigneeName string
	StatusName   string
	Category     string
	CreatedAt    time.Time
	ResolvedAt   *time.Time
	Points       *float64
	OriginalEst  *int64
	RemainingEst *int64
}

// StatusChange is a transition of an issue into a status of the given category.
type StatusChange struct {
	IssueID  uuid.UUID
	At       time.Time
	Category string
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

func hours(seconds int64) float64 { return round2(float64(seconds) / secondsPerHour) }

func dayKey(t time.Time, loc *time.Location) string { return t.In(loc).Format(dateLayout) }

// civil is a calendar date held as midnight UTC, so adding days never meets a daylight saving jump.
func civil(t time.Time, loc *time.Location) time.Time {
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// startOf is the instant at which the civil date begins in loc.
func startOf(date time.Time, loc *time.Location) time.Time {
	return time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, loc)
}

func historyByIssue(history []StatusChange) map[uuid.UUID][]StatusChange {
	out := map[uuid.UUID][]StatusChange{}
	for _, c := range history {
		out[c.IssueID] = append(out[c.IssueID], c)
	}
	for _, list := range out {
		sort.SliceStable(list, func(i, j int) bool { return list[i].At.Before(list[j].At) })
	}
	return out
}

// categoryAt is the category of the issue at the instant. Without history the issue is "done" from its
// resolution date and otherwise keeps its current category when that is "in_progress", else "todo".
func categoryAt(changes []StatusChange, issue IssueRow, at time.Time) string {
	if len(changes) > 0 {
		cat := categoryTodo
		for _, c := range changes {
			if c.At.After(at) {
				break
			}
			cat = c.Category
		}
		return cat
	}
	if issue.ResolvedAt != nil {
		if !issue.ResolvedAt.After(at) {
			return categoryDone
		}
		return categoryTodo
	}
	if issue.Category == categoryInProgress {
		return categoryInProgress
	}
	return categoryTodo
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func mean(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}

func stdDev(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	m := mean(v)
	s := 0.0
	for _, x := range v {
		s += (x - m) * (x - m)
	}
	return math.Sqrt(s / float64(len(v)))
}

func median(sorted []float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}
