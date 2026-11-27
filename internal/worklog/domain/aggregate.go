package domain

import (
	"sort"
	"time"

	"github.com/google/uuid"
)

// Entry is a worklog row with the data the aggregates need. The organization of the project decides the
// timezone and the working-time config used for day boundaries and capacity.
type Entry struct {
	ID             uuid.UUID
	IssueID        uuid.UUID
	IssueKey       string
	IssueTitle     string
	ProjectID      uuid.UUID
	OrganizationID uuid.UUID
	AuthorID       uuid.UUID
	AuthorName     string
	StartedAt      time.Time
	Seconds        int64
	Description    string
}

// OrgTime is the working-time context of one organization.
type OrgTime struct {
	Location *time.Location
	Config   Config
}

// LocalDate returns the calendar day of t in loc as YYYY-MM-DD.
func LocalDate(t time.Time, loc *time.Location) string {
	return t.In(loc).Format("2006-01-02")
}

// DayWindow parses the inclusive from/to dates (YYYY-MM-DD) and returns the UTC instants that bracket them,
// widened by a day on each side. Callers select rows inside the window and then filter with InDays, which
// is exact for every organization timezone (offsets are below 24 hours).
func DayWindow(from, to string) (lo, hi time.Time, err error) {
	start, err := time.Parse("2006-01-02", from)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	end, err := time.Parse("2006-01-02", to)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return start.Add(-24 * time.Hour), end.Add(48 * time.Hour), nil
}

// InDays reports whether the local day of t in loc is within from..to (inclusive). YYYY-MM-DD strings
// compare lexically, so a plain string comparison is exact.
func InDays(t time.Time, loc *time.Location, from, to string) bool {
	d := LocalDate(t, loc)
	return d >= from && d <= to
}

// TimesheetDay groups the entries of one local day.
type TimesheetDay struct {
	Date            string
	TotalSeconds    int64
	CapacitySeconds int64
	Entries         []Entry
}

// Timesheet is the result of the per-user timesheet.
type Timesheet struct {
	From         string
	To           string
	TotalSeconds int64
	Days         []TimesheetDay
}

// BuildTimesheet groups entries by the local day of their organization. Days come in ascending order and
// entries inside a day in start order. Entries outside from..to are dropped.
func BuildTimesheet(entries []Entry, zones map[uuid.UUID]OrgTime, from, to string) Timesheet {
	ts := Timesheet{From: from, To: to, Days: []TimesheetDay{}}
	byDate := map[string]*TimesheetDay{}
	for _, e := range entries {
		zone := zoneFor(zones, e.OrganizationID)
		if !InDays(e.StartedAt, zone.Location, from, to) {
			continue
		}
		date := LocalDate(e.StartedAt, zone.Location)
		day, ok := byDate[date]
		if !ok {
			day = &TimesheetDay{Date: date, CapacitySeconds: int64(zone.Config.HoursPerDay * 3600)}
			byDate[date] = day
		}
		day.TotalSeconds += e.Seconds
		day.Entries = append(day.Entries, e)
		ts.TotalSeconds += e.Seconds
	}
	for _, day := range byDate {
		sort.SliceStable(day.Entries, func(i, j int) bool { return day.Entries[i].StartedAt.Before(day.Entries[j].StartedAt) })
		ts.Days = append(ts.Days, *day)
	}
	sort.Slice(ts.Days, func(i, j int) bool { return ts.Days[i].Date < ts.Days[j].Date })
	return ts
}

func zoneFor(zones map[uuid.UUID]OrgTime, org uuid.UUID) OrgTime {
	if z, ok := zones[org]; ok && z.Location != nil && z.Config.Valid() {
		return z
	}
	return OrgTime{Location: time.UTC, Config: DefaultConfig()}
}

// Group kinds accepted by the time summary.
const (
	GroupByUser  = "user"
	GroupByIssue = "issue"
	GroupByDay   = "day"
)

// ValidGroupBy reports whether g is a supported grouping.
func ValidGroupBy(g string) bool {
	return g == GroupByUser || g == GroupByIssue || g == GroupByDay
}

// SummaryGroup is one row of the time summary.
type SummaryGroup struct {
	Key          string
	Label        string
	TotalSeconds int64
	Count        int
}

// Summary is the project time summary grouped by user, issue or day.
type Summary struct {
	GroupBy      string
	From         string
	To           string
	TotalSeconds int64
	Count        int
	Groups       []SummaryGroup
}

// BuildSummary aggregates entries of one project. Groups by user and by issue are sorted by total time
// (descending, then label); groups by day are sorted by date ascending.
func BuildSummary(entries []Entry, zones map[uuid.UUID]OrgTime, groupBy, from, to string) Summary {
	s := Summary{GroupBy: groupBy, From: from, To: to, Groups: []SummaryGroup{}}
	index := map[string]int{}
	for _, e := range entries {
		zone := zoneFor(zones, e.OrganizationID)
		if !InDays(e.StartedAt, zone.Location, from, to) {
			continue
		}
		key, label := groupKey(e, groupBy, zone.Location)
		i, ok := index[key]
		if !ok {
			i = len(s.Groups)
			index[key] = i
			s.Groups = append(s.Groups, SummaryGroup{Key: key, Label: label})
		}
		s.Groups[i].TotalSeconds += e.Seconds
		s.Groups[i].Count++
		s.TotalSeconds += e.Seconds
		s.Count++
	}
	sort.SliceStable(s.Groups, func(i, j int) bool {
		if groupBy == GroupByDay {
			return s.Groups[i].Key < s.Groups[j].Key
		}
		if s.Groups[i].TotalSeconds != s.Groups[j].TotalSeconds {
			return s.Groups[i].TotalSeconds > s.Groups[j].TotalSeconds
		}
		return s.Groups[i].Label < s.Groups[j].Label
	})
	return s
}

func groupKey(e Entry, groupBy string, loc *time.Location) (key, label string) {
	switch groupBy {
	case GroupByUser:
		return e.AuthorID.String(), e.AuthorName
	case GroupByIssue:
		return e.IssueID.String(), e.IssueKey + " " + e.IssueTitle
	default:
		d := LocalDate(e.StartedAt, loc)
		return d, d
	}
}
