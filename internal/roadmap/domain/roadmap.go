// Package domain holds the roadmap read model: bars for epics, versions and sprints with their derived dates and
// progress. It is pure: the repository supplies the raw rows.
package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// Stable error codes.
const (
	CodeInvalidRange = "roadmap.invalid_range"
	CodeInvalidQuery = "roadmap.invalid_query"
	CodeInvalidRank  = "roadmap.invalid_rank"
)

// MaxRangeDays bounds the window of a roadmap query.
const MaxRangeDays = 3660

// Group says what the bars of the roadmap are.
type Group string

const (
	GroupEpic    Group = "epic"
	GroupVersion Group = "version"
	GroupSprint  Group = "sprint"
)

// Filter narrows the bars by completion.
type Filter string

const (
	FilterAll  Filter = "all"
	FilterOpen Filter = "open"
	FilterDone Filter = "done"
)

// Source says where a bar date comes from.
type Source string

const (
	SourceOwn     Source = "own"
	SourceDerived Source = "derived"
	SourceNone    Source = "none"
)

// Progress is the completion of the children of a bar by status category.
type Progress struct {
	Total      int
	Todo       int
	InProgress int
	Done       int
	// Percent is Done over Total, rounded; a bar without children is 100 when it is done and 0 otherwise.
	Percent     int
	PointsTotal float64
	PointsDone  float64
}

// Bar is one bar of the timeline. Start and End are dates (UTC midnight) and nil only when the bar has no dates
// at all, in which case Dated is false and the client lists it apart.
type Bar struct {
	ID             uuid.UUID
	Kind           string
	Key            string
	Title          string
	Status         string
	StatusCategory string
	Assignee       string
	Start          *time.Time
	End            *time.Time
	StartSource    Source
	EndSource      Source
	Dated          bool
	Complete       bool
	Progress       Progress
}

// EpicRow is an epic as stored plus the aggregates of its direct children.
type EpicRow struct {
	ID             uuid.UUID
	Key            string
	Title          string
	StatusCategory string
	Assignee       string
	Start, Due     *time.Time
	Total          int
	Todo           int
	InProgress     int
	Done           int
	PointsTotal    float64
	PointsDone     float64
	ChildStart     *time.Time
	ChildDue       *time.Time
}

// PeriodRow is a version or a sprint as stored plus the progress of its issues.
type PeriodRow struct {
	ID          uuid.UUID
	Kind        string // version or sprint
	Name        string
	Status      string
	Start, End  *time.Time
	Total       int
	Todo        int
	InProgress  int
	Done        int
	PointsTotal float64
	PointsDone  float64
}

// Dependency says that From blocks To.
type Dependency struct {
	From uuid.UUID
	To   uuid.UUID
	Kind string
}

// ComputeProgress turns the counters into a Progress with its percentage.
func ComputeProgress(total, todo, inProgress, done int, pointsTotal, pointsDone float64, ownDone bool) Progress {
	p := Progress{Total: total, Todo: todo, InProgress: inProgress, Done: done, PointsTotal: pointsTotal, PointsDone: pointsDone}
	switch {
	case total > 0:
		p.Percent = (done*100 + total/2) / total
	case ownDone:
		p.Percent = 100
	}
	return p
}

// resolveDates derives the visible range of a bar: its own dates, else the dates derived from its children. A
// bar with only one end becomes a one-day bar on it, and an end before the start collapses onto the end.
func resolveDates(ownStart, ownEnd, childStart, childEnd *time.Time) (start, end *time.Time, startSrc, endSrc Source) {
	startSrc, endSrc = SourceNone, SourceNone
	switch {
	case ownStart != nil:
		start, startSrc = ownStart, SourceOwn
	case childStart != nil:
		start, startSrc = childStart, SourceDerived
	}
	switch {
	case ownEnd != nil:
		end, endSrc = ownEnd, SourceOwn
	case childEnd != nil:
		end, endSrc = childEnd, SourceDerived
	}
	if start == nil && end != nil {
		start, startSrc = end, SourceDerived
	}
	if end == nil && start != nil {
		end, endSrc = start, SourceDerived
	}
	if start != nil && end != nil && end.Before(*start) {
		start, startSrc = end, SourceDerived
	}
	return start, end, startSrc, endSrc
}

// EpicBar builds the bar of an epic.
func EpicBar(r EpicRow) Bar {
	start, end, ss, es := resolveDates(r.Start, r.Due, r.ChildStart, r.ChildDue)
	progress := ComputeProgress(r.Total, r.Todo, r.InProgress, r.Done, r.PointsTotal, r.PointsDone, r.StatusCategory == "done")
	return Bar{
		ID: r.ID, Kind: "epic", Key: r.Key, Title: r.Title, Status: r.StatusCategory, StatusCategory: r.StatusCategory, Assignee: r.Assignee,
		Start: start, End: end, StartSource: ss, EndSource: es, Dated: start != nil && end != nil,
		Complete: r.StatusCategory == "done" || (r.Total > 0 && r.Done == r.Total), Progress: progress,
	}
}

// PeriodBar builds the bar of a version or a sprint.
func PeriodBar(r PeriodRow) Bar {
	start, end, ss, es := resolveDates(r.Start, r.End, nil, nil)
	progress := ComputeProgress(r.Total, r.Todo, r.InProgress, r.Done, r.PointsTotal, r.PointsDone, false)
	complete := r.Status == "released" || r.Status == "closed"
	return Bar{
		ID: r.ID, Kind: r.Kind, Title: r.Name, Status: r.Status, Start: start, End: end, StartSource: ss, EndSource: es,
		Dated: start != nil && end != nil, Complete: complete, Progress: progress,
	}
}

// Query selects what the roadmap returns.
type Query struct {
	From, To *time.Time
	Group    Group
	Filter   Filter
	Text     string
}

// ParseGroup validates a group_by value; empty means epic.
func ParseGroup(s string) (Group, error) {
	switch Group(s) {
	case "", GroupEpic:
		return GroupEpic, nil
	case GroupVersion, GroupSprint:
		return Group(s), nil
	}
	return "", apperror.WithCode(apperror.KindValidation, CodeInvalidQuery, "group_by debe ser epic, version o sprint")
}

// ParseFilter validates a filter value; empty means all.
func ParseFilter(s string) (Filter, error) {
	switch Filter(s) {
	case "", FilterAll:
		return FilterAll, nil
	case FilterOpen, FilterDone:
		return Filter(s), nil
	}
	return "", apperror.WithCode(apperror.KindValidation, CodeInvalidQuery, "filter debe ser all, open o done")
}

// ValidateRange checks an optional window.
func ValidateRange(from, to *time.Time) error {
	if from != nil && to != nil {
		if to.Before(*from) {
			return apperror.WithCode(apperror.KindValidation, CodeInvalidRange, "el rango no es válido: from no puede ser posterior a to")
		}
		if to.Sub(*from) > MaxRangeDays*24*time.Hour {
			return apperror.WithCode(apperror.KindValidation, CodeInvalidRange, "el rango no es válido: como máximo 10 años")
		}
	}
	return nil
}

// InRange reports whether a bar overlaps the window; undated bars always do so the client can list them apart.
func (b Bar) InRange(from, to *time.Time) bool {
	if !b.Dated {
		return true
	}
	if from != nil && b.End.Before(*from) {
		return false
	}
	if to != nil && b.Start.After(*to) {
		return false
	}
	return true
}

// Keep reports whether the bar passes the completion filter and the text search (key or title, case insensitive).
func (b Bar) Keep(f Filter, text string) bool {
	switch f {
	case FilterOpen:
		if b.Complete {
			return false
		}
	case FilterDone:
		if !b.Complete {
			return false
		}
	}
	if text != "" {
		t := strings.ToLower(text)
		if !strings.Contains(strings.ToLower(b.Title), t) && !strings.Contains(strings.ToLower(b.Key), t) {
			return false
		}
	}
	return true
}

// FilterBars applies the window, the completion filter and the text search, keeping the order.
func FilterBars(bars []Bar, q Query) []Bar {
	out := make([]Bar, 0, len(bars))
	for _, b := range bars {
		if b.InRange(q.From, q.To) && b.Keep(q.Filter, q.Text) {
			out = append(out, b)
		}
	}
	return out
}

// ValidateRankIDs checks the ordered epic ids of a rank request: no duplicates, at most max.
func ValidateRankIDs(ids []uuid.UUID, max int) error {
	if len(ids) > max {
		return apperror.WithCode(apperror.KindValidation, CodeInvalidRank, "demasiadas épicas en el orden")
	}
	seen := map[uuid.UUID]bool{}
	for _, id := range ids {
		if seen[id] {
			return apperror.WithCode(apperror.KindValidation, CodeInvalidRank, "el orden repite una épica")
		}
		seen[id] = true
	}
	return nil
}
