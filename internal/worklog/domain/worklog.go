package domain

import (
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// MaxFutureStart is how far ahead of now a worklog may start.
const MaxFutureStart = 24 * time.Hour

// RemainingMode says how a worklog changes the remaining estimate of its issue.
type RemainingMode string

const (
	// RemainingAuto reduces the remaining estimate by the time spent, never below zero.
	RemainingAuto RemainingMode = "auto"
	// RemainingSet replaces the remaining estimate with an explicit value.
	RemainingSet RemainingMode = "set"
	// RemainingReduce reduces the remaining estimate by an explicit amount, never below zero.
	RemainingReduce RemainingMode = "reduce"
	// RemainingLeave does not touch the remaining estimate.
	RemainingLeave RemainingMode = "leave"
)

// ValidRemainingMode reports whether m is one of the four modes.
func ValidRemainingMode(m RemainingMode) bool {
	switch m {
	case RemainingAuto, RemainingSet, RemainingReduce, RemainingLeave:
		return true
	}
	return false
}

// RemainingChange is the estimate change requested with a worklog. Value is used by set (zero allowed)
// and reduce (positive); it is ignored by auto and leave.
type RemainingChange struct {
	Mode  RemainingMode
	Value int64
}

// ApplyRemaining computes the remaining estimate after a worklog of spent seconds. current is nil when the
// issue has no remaining estimate; auto and reduce then leave it nil. The returned adjust is the amount
// removed from the estimate by this entry, which RestoreRemaining gives back when the entry goes away.
func ApplyRemaining(current *int64, spent int64, change RemainingChange) (next *int64, adjust int64, err error) {
	switch change.Mode {
	case RemainingLeave:
		return copyPtr(current), 0, nil
	case RemainingSet:
		if change.Value < 0 || change.Value > MaxEstimateSeconds {
			return nil, 0, ErrDurationRange
		}
		v := change.Value
		return &v, 0, nil
	case RemainingAuto:
		return reduce(current, spent)
	case RemainingReduce:
		if change.Value <= 0 {
			return nil, 0, ErrDurationRange
		}
		return reduce(current, change.Value)
	}
	return nil, 0, apperror.WithCode(apperror.KindValidation, "worklog.remaining_mode_invalid", "El modo de ajuste de la estimacion restante no es valido")
}

func reduce(current *int64, by int64) (*int64, int64, error) {
	if current == nil {
		return nil, 0, nil
	}
	removed := by
	if *current < removed {
		removed = *current
	}
	next := *current - removed
	return &next, removed, nil
}

// RestoreRemaining gives back the adjust removed by an entry that is deleted or re-priced. A nil estimate
// stays nil: the estimate was cleared after the entry and there is nothing to restore into.
func RestoreRemaining(current *int64, adjust int64) *int64 {
	if current == nil || adjust <= 0 {
		return copyPtr(current)
	}
	next := *current + adjust
	if next > MaxEstimateSeconds {
		next = MaxEstimateSeconds
	}
	return &next
}

func copyPtr(v *int64) *int64 {
	if v == nil {
		return nil
	}
	c := *v
	return &c
}

// Worklog is one block of time logged by a person on an issue.
type Worklog struct {
	ID                     uuid.UUID
	IssueID                uuid.UUID
	IssueKey               string
	IssueTitle             string
	ProjectID              uuid.UUID
	OrganizationID         uuid.UUID
	AuthorID               uuid.UUID
	AuthorName             string
	StartedAt              time.Time
	TimeSpentSeconds       int64
	Description            string
	RemainingMode          RemainingMode
	RemainingAdjustSeconds int64
	CreatedAt              time.Time
	UpdatedAt              time.Time
	DeletedAt              *time.Time
}

// ValidateEntry checks the fields of a worklog against the rules: positive bounded duration, a start at
// most a day in the future and a bounded description.
func ValidateEntry(seconds int64, startedAt, now time.Time, description string) error {
	if seconds <= 0 || seconds > MaxWorklogSeconds {
		return ErrDurationRange
	}
	if startedAt.After(now.Add(MaxFutureStart)) {
		return ErrStartedInFuture
	}
	if utf8.RuneCountInString(description) > MaxDescriptionRunes {
		return ErrDescriptionTooLong
	}
	return nil
}
