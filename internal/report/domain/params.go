package domain

import (
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Limits of every report.
const (
	MaxRangeDays   = 366
	MaxFilterRunes = 2000
	MaxSprints     = 20
	DefaultSprints = 6
	defaultDays    = 30
)

// RawParams are the query parameters as text, before validation.
type RawParams struct {
	From, To, GroupBy, Filter, Sprint, Version, Interval, Limit string
}

// Params are the validated parameters. FromDate and ToDate are calendar dates (inclusive); Lo and Hi are
// the instants [Lo, Hi) that those dates cover in the organization time zone.
type Params struct {
	Kind             Kind
	FromDate, ToDate time.Time
	Lo, Hi           time.Time
	Loc              *time.Location
	GroupBy          string
	Interval         string
	Filter           string
	SprintID         *uuid.UUID
	VersionID        *uuid.UUID
	Limit            int
}

// Days is the number of calendar days of the range.
func (p Params) Days() int { return int(p.ToDate.Sub(p.FromDate).Hours()/hoursPerDay) + 1 }

// Echo lists the parameters the report used, for the response.
func (p Params) Echo() map[string]any {
	out := map[string]any{"timezone": p.Loc.String()}
	if usesRange(p.Kind) {
		out["from"], out["to"] = p.FromDate.Format(dateLayout), p.ToDate.Format(dateLayout)
	}
	if p.GroupBy != "" {
		out["group_by"] = p.GroupBy
	}
	if p.Interval != "" {
		out["interval"] = p.Interval
	}
	if p.Filter != "" && acceptsFilter(p.Kind) {
		out["filter"] = p.Filter
	}
	if p.SprintID != nil {
		out["sprint"] = p.SprintID.String()
	}
	if p.VersionID != nil {
		out["version"] = p.VersionID.String()
	}
	if p.Kind == KindVelocity {
		out["limit"] = p.Limit
	}
	return out
}

func usesRange(k Kind) bool {
	switch k {
	case KindCumulativeFlow, KindControl, KindCreatedVsRes, KindResolutionTime, KindTimeTracking:
		return true
	}
	return false
}

func acceptsFilter(k Kind) bool { return k != KindVelocity && k != KindSprintReport }

// AcceptsFilter reports whether the kind can be narrowed with a CoreQL filter.
func AcceptsFilter(k Kind) bool { return acceptsFilter(k) }

func allowedGroups(k Kind) (def string, allowed []string) {
	switch k {
	case KindBurndown, KindBurnup:
		return "points", []string{"points", "issues"}
	case KindTimeTracking:
		return "issue", []string{"issue", "user"}
	case KindResolutionTime:
		return "type", []string{"type", "priority"}
	}
	return "", nil
}

// ResolveParams validates the raw parameters of a report. now is the current instant; the default range
// is the last 30 days ending today in the organization time zone.
func ResolveParams(kind Kind, raw RawParams, loc *time.Location, now time.Time) (Params, error) {
	p := Params{Kind: kind, Loc: loc}
	today := civil(now, loc)
	from, to := today.AddDate(0, 0, -(defaultDays-1)), today
	var err error
	if raw.To != "" {
		if to, err = parseDate(raw.To); err != nil {
			return p, ErrRangeInvalid
		}
	}
	if raw.From != "" {
		if from, err = parseDate(raw.From); err != nil {
			return p, ErrRangeInvalid
		}
	} else if raw.To != "" {
		from = to.AddDate(0, 0, -(defaultDays - 1))
	}
	if from.After(to) {
		return p, ErrRangeInvalid
	}
	if to.Sub(from).Hours()/hoursPerDay >= MaxRangeDays {
		return p, ErrRangeTooLarge
	}
	p.FromDate, p.ToDate = from, to
	p.Lo, p.Hi = startOf(from, loc), startOf(to.AddDate(0, 0, 1), loc)

	if utf8.RuneCountInString(raw.Filter) > MaxFilterRunes {
		return p, ErrFilterInvalid
	}
	p.Filter = strings.TrimSpace(raw.Filter)

	def, allowed := allowedGroups(kind)
	p.GroupBy = def
	if raw.GroupBy != "" && allowed != nil {
		if !contains(allowed, raw.GroupBy) {
			return p, ErrParamInvalid
		}
		p.GroupBy = raw.GroupBy
	}
	if kind == KindCreatedVsRes {
		p.Interval = "day"
		if raw.Interval != "" {
			if !contains([]string{"day", "week", "month"}, raw.Interval) {
				return p, ErrParamInvalid
			}
			p.Interval = raw.Interval
		}
	}
	if raw.Sprint != "" {
		id, err := uuid.Parse(raw.Sprint)
		if err != nil || id == uuid.Nil {
			return p, ErrParamInvalid
		}
		p.SprintID = &id
	}
	if raw.Version != "" {
		id, err := uuid.Parse(raw.Version)
		if err != nil || id == uuid.Nil {
			return p, ErrParamInvalid
		}
		p.VersionID = &id
	}
	if kind == KindVersion && p.VersionID == nil {
		return p, ErrVersionRequired
	}
	p.Limit = DefaultSprints
	if raw.Limit != "" {
		n, err := strconv.Atoi(raw.Limit)
		if err != nil || n < 1 || n > MaxSprints {
			return p, ErrParamInvalid
		}
		p.Limit = n
	}
	return p, nil
}

func parseDate(s string) (time.Time, error) {
	return time.Parse(dateLayout, s)
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}
