package domain

import (
	"sort"
	"strings"

	"github.com/google/uuid"
)

// CardRef points to another issue shown on a card (epic or parent).
type CardRef struct {
	ID      uuid.UUID
	Key     string
	Summary string
	Type    string
}

// Card is an issue as the board shows it.
type Card struct {
	ID      uuid.UUID
	Key     string
	Summary string

	Type          string
	TypeKey       string
	TypeIcon      string
	TypeColor     string
	Priority      string
	PriorityKey   string
	PriorityIcon  string
	PriorityColor string

	StatusID       uuid.UUID
	StatusName     string
	StatusCategory string

	AssigneeID   *uuid.UUID
	AssigneeName string
	Labels       []string

	StoryPoints      *float64
	RemainingSeconds *int64
	Flagged          bool
	Rank             string
	SprintID         *uuid.UUID

	Epic   *CardRef
	Parent *CardRef

	SubtaskTotal int
	SubtaskDone  int
}

// Matcher decides whether a card satisfies a compiled query.
type Matcher interface {
	Match(card Card, ctx QueryContext) bool
}

// WIPState is the state of a column against its limits.
type WIPState string

const (
	WIPOk       WIPState = "ok"
	WIPBelowMin WIPState = "below_min"
	WIPAtMax    WIPState = "at_max"
	WIPOverMax  WIPState = "over_max"
)

// EvaluateWIP compares the number of cards of a column with its limits.
func EvaluateWIP(count int, min, max *int) WIPState {
	switch {
	case max != nil && count > *max:
		return WIPOverMax
	case max != nil && count == *max:
		return WIPAtMax
	case min != nil && count < *min:
		return WIPBelowMin
	}
	return WIPOk
}

// Identifiers of the implicit lanes.
const (
	UnmappedColumnID  = "unmapped"
	LaneUnassigned    = "unassigned"
	LaneNoEpic        = "no-epic"
	LaneNoParent      = "no-parent"
	LaneOther         = "other"
	unmappedColumnTag = "Sin asignar"
)

// PlacedCard is a card in a column with the swimlane it belongs to ("" without swimlanes).
type PlacedCard struct {
	Card
	SwimlaneID string
}

// ViewColumn is a column with its cards and counters.
type ViewColumn struct {
	Column
	Cards []PlacedCard
	// Count is the number of cards shown (after quick filters); Total is the number of cards of the column
	// before them. The WIP state is evaluated on Total.
	Count    int
	Total    int
	Estimate float64
	WIP      WIPState
}

// ViewLane is a swimlane of the view.
type ViewLane struct {
	ID    string
	Name  string
	Kind  SwimlaneMode
	Count int
}

// View is the assembled board: columns with cards, swimlanes and the implicit unmapped column.
type View struct {
	Columns   []ViewColumn
	Swimlanes []ViewLane
	// Unmapped holds the cards whose status no column maps.
	Unmapped      ViewColumn
	UnmappedCount int
}

// AssembleView places the cards into the columns of the board, applies the quick filters (all of them must
// match) and groups the cards into swimlanes. laneQueries are aligned with board.Swimlanes.
func AssembleView(board *Board, cards []Card, quick []Matcher, laneQueries []Matcher, ctx QueryContext) View {
	columnOf := map[uuid.UUID]int{}
	view := View{Columns: make([]ViewColumn, len(board.Columns))}
	for i, column := range board.Columns {
		view.Columns[i] = ViewColumn{Column: column, Cards: []PlacedCard{}}
		for _, statusID := range column.StatusIDs {
			columnOf[statusID] = i
		}
	}
	view.Unmapped = ViewColumn{Column: Column{Name: unmappedColumnTag}, Cards: []PlacedCard{}}

	lanes := newLaneSet(board, laneQueries)
	for _, card := range cards {
		target := &view.Unmapped
		if index, ok := columnOf[card.StatusID]; ok {
			target = &view.Columns[index]
		}
		target.Total++
		shown := true
		for _, m := range quick {
			if !m.Match(card, ctx) {
				shown = false
				break
			}
		}
		if !shown {
			continue
		}
		target.Count++
		switch board.Estimation {
		case EstimationIssueCount:
			target.Estimate++
		case EstimationTime:
			if card.RemainingSeconds != nil {
				target.Estimate += float64(*card.RemainingSeconds) / 3600
			}
		default:
			if card.StoryPoints != nil {
				target.Estimate += *card.StoryPoints
			}
		}
		target.Cards = append(target.Cards, PlacedCard{Card: card, SwimlaneID: lanes.assign(card, ctx)})
	}
	for i := range view.Columns {
		c := &view.Columns[i]
		c.WIP = EvaluateWIP(c.Total, c.MinIssues, c.WIPLimit)
	}
	view.Unmapped.WIP = WIPOk
	view.UnmappedCount = view.Unmapped.Total
	view.Swimlanes = lanes.list()
	return view
}

type laneSet struct {
	mode    SwimlaneMode
	queries []Matcher
	board   *Board
	byID    map[string]*ViewLane
	order   []string
}

func newLaneSet(board *Board, queries []Matcher) *laneSet {
	set := &laneSet{mode: board.SwimlaneMode, queries: queries, board: board, byID: map[string]*ViewLane{}}
	if set.mode == SwimlanesQuery {
		for i, lane := range board.Swimlanes {
			if i < len(queries) {
				set.add(lane.ID.String(), lane.Name)
			}
		}
	}
	return set
}

func (s *laneSet) add(id, name string) *ViewLane {
	if lane, ok := s.byID[id]; ok {
		return lane
	}
	s.byID[id] = &ViewLane{ID: id, Name: name, Kind: s.mode}
	s.order = append(s.order, id)
	return s.byID[id]
}

func (s *laneSet) assign(card Card, ctx QueryContext) string {
	var id, name string
	switch s.mode {
	case SwimlanesAssignee:
		if card.AssigneeID != nil {
			id, name = card.AssigneeID.String(), card.AssigneeName
		} else {
			id, name = LaneUnassigned, "Sin asignar"
		}
	case SwimlanesEpic:
		if card.Epic != nil {
			id, name = card.Epic.ID.String(), card.Epic.Key+" "+card.Epic.Summary
		} else {
			id, name = LaneNoEpic, "Sin épica"
		}
	case SwimlanesStory:
		if card.Parent != nil && card.Parent.Type != "epic" {
			id, name = card.Parent.ID.String(), card.Parent.Key+" "+card.Parent.Summary
		} else {
			id, name = LaneNoParent, "Sin historia padre"
		}
	case SwimlanesQuery:
		id, name = LaneOther, "Otras"
		for i, lane := range s.board.Swimlanes {
			if i < len(s.queries) && s.queries[i].Match(card, ctx) {
				id = lane.ID.String()
				break
			}
		}
	default:
		return ""
	}
	lane := s.add(id, name)
	lane.Count++
	return id
}

// list returns the lanes: configured order for query lanes, otherwise by name with the implicit lane last.
func (s *laneSet) list() []ViewLane {
	out := make([]ViewLane, 0, len(s.order))
	for _, id := range s.order {
		out = append(out, *s.byID[id])
	}
	if s.mode == SwimlanesNone {
		return out
	}
	implicit := func(id string) bool {
		return id == LaneUnassigned || id == LaneNoEpic || id == LaneNoParent || id == LaneOther
	}
	if s.mode != SwimlanesQuery {
		sort.SliceStable(out, func(i, j int) bool {
			if implicit(out[i].ID) != implicit(out[j].ID) {
				return implicit(out[j].ID)
			}
			return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
		})
		return out
	}
	// Query lanes keep their configured order; the catch-all lane only appears when it has cards.
	kept := out[:0]
	for _, lane := range out {
		if lane.ID != LaneOther || lane.Count > 0 {
			kept = append(kept, lane)
		}
	}
	return kept
}
