package domain

import (
	"sort"
	"time"

	"github.com/google/uuid"
)

// maxBurndownDays acota la serie del burndown para sprints con fechas absurdas.
const maxBurndownDays = 120

// StatusChange es un cambio de estado de una incidencia: desde ese instante esta (o deja de estar) terminada.
type StatusChange struct {
	At   time.Time
	Done bool
}

// DoneTimeline sabe si una incidencia estaba terminada en un instante. Con historial manda el ultimo cambio
// anterior; sin historial se usa la fecha de cierre actual (DoneAt) si la incidencia esta terminada.
type DoneTimeline struct {
	Changes []StatusChange
	DoneAt  *time.Time
}

// DoneBy indica si la incidencia estaba terminada en el instante dado.
func (t DoneTimeline) DoneBy(at time.Time) bool {
	if len(t.Changes) == 0 {
		return t.DoneAt != nil && !t.DoneAt.After(at)
	}
	done := false
	for _, c := range t.Changes {
		if c.At.After(at) {
			break
		}
		done = c.Done
	}
	return done
}

// ReportIssue es una incidencia en una lista del informe, con los puntos que tenia en el momento relevante.
type ReportIssue struct {
	IssueID         uuid.UUID
	Key             string
	Summary         string
	Points          *float64
	StatusCategory  string
	AddedAfterStart bool
	// RemovedAt y RemovedKind solo se informan en la lista de retiradas.
	RemovedAt   *time.Time
	RemovedKind EventKind
}

// BurndownDay es un dia de la serie: puntos restantes, alcance total y linea ideal (nil sin fecha de fin).
type BurndownDay struct {
	Date      time.Time
	Ideal     *float64
	Remaining float64
	Scope     float64
	Completed float64
}

// Report es el informe de un sprint calculado a partir de su historial. AddedAfterStart es un subconjunto
// de Completed e Incomplete (las incidencias que no formaban parte del alcance comprometido).
type Report struct {
	Sprint           *Sprint
	CommittedPoints  float64
	CompletedPoints  float64
	IncompletePoints float64
	AddedPoints      float64
	RemovedPoints    float64
	Completed        []ReportIssue
	Incomplete       []ReportIssue
	Removed          []ReportIssue
	AddedAfterStart  []ReportIssue
	Burndown         []BurndownDay
}

type issueState struct {
	issue      uuid.UUID
	key        string
	summary    string
	inScope    bool
	everAdded  bool
	committed  bool
	points     *float64
	completion EventKind
	removedAt  *time.Time
	removedBy  EventKind
}

// BuildReport calcula el informe. events debe venir ordenado por (happened_at, id). categories da la
// categoria actual de cada incidencia viva: las que no estan (borradas) se ignoran. timelines da el
// historial de estados para el burndown.
func BuildReport(sprint *Sprint, events []Event, categories map[uuid.UUID]string, timelines map[uuid.UUID]DoneTimeline, now time.Time) *Report {
	r := &Report{Sprint: sprint}
	if sprint.Status == StatusPlanned {
		return r
	}
	states := map[uuid.UUID]*issueState{}
	var ordered []*issueState
	derivedCommitted := 0.0
	for _, e := range events {
		st := states[e.IssueID]
		if st == nil {
			st = &issueState{issue: e.IssueID, key: e.Key, summary: e.Summary}
			states[e.IssueID] = st
			ordered = append(ordered, st)
		}
		switch e.Kind {
		case EventAdded:
			st.inScope, st.everAdded, st.points, st.removedAt, st.removedBy = true, true, e.Points, nil, ""
			if !e.AfterStart {
				st.committed = true
				derivedCommitted += pointsOf(e.Points)
			}
		case EventRemoved, EventMovedOut:
			at := e.HappenedAt
			st.inScope, st.removedAt, st.removedBy = false, &at, e.Kind
			if e.Points != nil {
				st.points = e.Points
			}
		case EventPointsChanged:
			if st.inScope {
				st.points = e.Points
			}
		case EventCompleted, EventIncomplete:
			st.completion = e.Kind
			st.points = e.Points
		}
	}
	r.CommittedPoints = derivedCommitted
	if sprint.CommittedPoints != nil {
		r.CommittedPoints = *sprint.CommittedPoints
	}

	for _, st := range ordered {
		category, alive := categories[st.issue]
		if !alive {
			continue
		}
		item := ReportIssue{IssueID: st.issue, Key: st.key, Summary: st.summary, Points: st.points,
			StatusCategory: category, AddedAfterStart: st.inScope && !st.committed}
		switch {
		case !st.inScope && st.everAdded:
			item.RemovedAt, item.RemovedKind = st.removedAt, st.removedBy
			r.Removed = append(r.Removed, item)
			r.RemovedPoints += pointsOf(st.points)
		case st.inScope:
			done := category == CategoryDone
			if sprint.Status == StatusClosed {
				// A closed sprint keeps the verdict recorded when it was completed, whatever happened later.
				done = st.completion == EventCompleted
				if done {
					item.StatusCategory = CategoryDone
				}
			}
			if done {
				r.Completed = append(r.Completed, item)
				r.CompletedPoints += pointsOf(st.points)
			} else {
				r.Incomplete = append(r.Incomplete, item)
				r.IncompletePoints += pointsOf(st.points)
			}
			if item.AddedAfterStart {
				r.AddedAfterStart = append(r.AddedAfterStart, item)
				r.AddedPoints += pointsOf(st.points)
			}
		}
	}
	if sprint.Status == StatusClosed && sprint.CompletedPoints != nil {
		r.CompletedPoints = *sprint.CompletedPoints
	}
	r.Burndown = buildBurndown(sprint, events, timelines, categories, r.CommittedPoints, now)
	return r
}

// buildBurndown recorre dia a dia (UTC) el alcance vigente y resta lo ya terminado ese dia.
func buildBurndown(sprint *Sprint, events []Event, timelines map[uuid.UUID]DoneTimeline, alive map[uuid.UUID]string, committed float64, now time.Time) []BurndownDay {
	if sprint.StartedAt == nil && sprint.StartDate == nil {
		return nil
	}
	var first time.Time
	if sprint.StartDate != nil {
		first = dateOf(*sprint.StartDate)
	} else {
		first = dateOf(*sprint.StartedAt)
	}
	plannedEnd := first.AddDate(0, 0, defaultSprintDays)
	hasEnd := sprint.EndDate != nil
	if hasEnd {
		plannedEnd = dateOf(*sprint.EndDate)
	}
	last := plannedEnd
	switch sprint.Status {
	case StatusClosed:
		if sprint.CompletedAt != nil {
			last = dateOf(*sprint.CompletedAt)
		}
	case StatusActive:
		if today := dateOf(now); today.Before(last) {
			last = today
		}
	}
	if last.Before(first) {
		last = first
	}
	idealDays := int(plannedEnd.Sub(first).Hours()/24) + 1
	if idealDays < 1 {
		idealDays = 1
	}

	type scope struct {
		in     bool
		points *float64
	}
	var days []BurndownDay
	state := map[uuid.UUID]*scope{}
	next := 0
	for i := 0; i <= int(last.Sub(first).Hours()/24) && i < maxBurndownDays; i++ {
		day := first.AddDate(0, 0, i)
		endOfDay := day.Add(24*time.Hour - time.Nanosecond)
		for next < len(events) && !events[next].HappenedAt.After(endOfDay) {
			e := events[next]
			next++
			s := state[e.IssueID]
			if s == nil {
				s = &scope{}
				state[e.IssueID] = s
			}
			switch e.Kind {
			case EventAdded:
				s.in, s.points = true, e.Points
			case EventRemoved, EventMovedOut:
				s.in = false
			case EventPointsChanged:
				if s.in {
					s.points = e.Points
				}
			}
		}
		total, remaining := 0.0, 0.0
		for id, s := range state {
			if _, ok := alive[id]; !ok || !s.in {
				continue
			}
			total += pointsOf(s.points)
			if !timelines[id].DoneBy(endOfDay) {
				remaining += pointsOf(s.points)
			}
		}
		d := BurndownDay{Date: day, Remaining: remaining, Scope: total, Completed: total - remaining}
		if hasEnd {
			ideal := 0.0
			if i < idealDays {
				ideal = committed
				if idealDays > 1 {
					ideal = committed * float64(idealDays-1-i) / float64(idealDays-1)
				}
			}
			d.Ideal = &ideal
		}
		days = append(days, d)
	}
	return days
}

// SortEvents ordena los eventos por instante y, a igualdad, por id.
func SortEvents(events []Event) {
	sort.SliceStable(events, func(i, j int) bool {
		if !events[i].HappenedAt.Equal(events[j].HappenedAt) {
			return events[i].HappenedAt.Before(events[j].HappenedAt)
		}
		return events[i].ID < events[j].ID
	})
}
