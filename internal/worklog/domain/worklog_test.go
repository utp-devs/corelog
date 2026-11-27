package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

func ptr(v int64) *int64 { return &v }

func TestApplyRemainingMatrix(t *testing.T) {
	cases := []struct {
		name        string
		current     *int64
		spent       int64
		change      RemainingChange
		wantNext    *int64
		wantAdjust  int64
		wantErrCode string
	}{
		{"auto reduces by spent", ptr(10000), 3000, RemainingChange{Mode: RemainingAuto}, ptr(7000), 3000, ""},
		{"auto floors at zero", ptr(1000), 3000, RemainingChange{Mode: RemainingAuto}, ptr(0), 1000, ""},
		{"auto without estimate stays nil", nil, 3000, RemainingChange{Mode: RemainingAuto}, nil, 0, ""},
		{"set replaces value", ptr(10000), 3000, RemainingChange{Mode: RemainingSet, Value: 500}, ptr(500), 0, ""},
		{"set zero is valid", ptr(10000), 3000, RemainingChange{Mode: RemainingSet, Value: 0}, ptr(0), 0, ""},
		{"set creates estimate when missing", nil, 3000, RemainingChange{Mode: RemainingSet, Value: 900}, ptr(900), 0, ""},
		{"set negative rejected", ptr(1), 1, RemainingChange{Mode: RemainingSet, Value: -1}, nil, 0, "range"},
		{"reduce by amount", ptr(10000), 3000, RemainingChange{Mode: RemainingReduce, Value: 2000}, ptr(8000), 2000, ""},
		{"reduce floors at zero", ptr(1500), 3000, RemainingChange{Mode: RemainingReduce, Value: 2000}, ptr(0), 1500, ""},
		{"reduce zero rejected", ptr(1500), 3000, RemainingChange{Mode: RemainingReduce, Value: 0}, nil, 0, "range"},
		{"reduce without estimate stays nil", nil, 3000, RemainingChange{Mode: RemainingReduce, Value: 2000}, nil, 0, ""},
		{"leave keeps value", ptr(10000), 3000, RemainingChange{Mode: RemainingLeave}, ptr(10000), 0, ""},
		{"leave keeps nil", nil, 3000, RemainingChange{Mode: RemainingLeave}, nil, 0, ""},
		{"unknown mode", ptr(1), 1, RemainingChange{Mode: "bogus"}, nil, 0, "worklog.remaining_mode_invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			next, adjust, err := ApplyRemaining(tc.current, tc.spent, tc.change)
			if tc.wantErrCode != "" {
				if err == nil {
					t.Fatalf("expected error")
				}
				if tc.wantErrCode == "range" {
					if apperror.CodeOf(err) != CodeDurationRange {
						t.Fatalf("code = %s", apperror.CodeOf(err))
					}
				} else if apperror.CodeOf(err) != tc.wantErrCode {
					t.Fatalf("code = %s", apperror.CodeOf(err))
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			if !equalPtr(next, tc.wantNext) || adjust != tc.wantAdjust {
				t.Fatalf("got next=%v adjust=%d, want next=%v adjust=%d", deref(next), adjust, deref(tc.wantNext), tc.wantAdjust)
			}
		})
	}
}

func TestRestoreRemaining(t *testing.T) {
	if got := RestoreRemaining(ptr(7000), 3000); *got != 10000 {
		t.Fatalf("restore = %d", *got)
	}
	if got := RestoreRemaining(nil, 3000); got != nil {
		t.Fatalf("nil estimate must stay nil")
	}
	if got := RestoreRemaining(ptr(7000), 0); *got != 7000 {
		t.Fatalf("zero adjust must not change value")
	}
	if got := RestoreRemaining(ptr(MaxEstimateSeconds), 10); *got != MaxEstimateSeconds {
		t.Fatalf("restore must cap at the estimate limit")
	}
}

func TestValidateEntry(t *testing.T) {
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		seconds int64
		start   time.Time
		desc    string
		code    string
	}{
		{"valid", 3600, now, "ok", ""},
		{"max duration", MaxWorklogSeconds, now, "", ""},
		{"over max duration", MaxWorklogSeconds + 1, now, "", CodeDurationRange},
		{"zero", 0, now, "", CodeDurationRange},
		{"start exactly one day ahead", 60, now.Add(24 * time.Hour), "", ""},
		{"start beyond one day ahead", 60, now.Add(24*time.Hour + time.Second), "", CodeStartedInFuture},
		{"description at limit", 60, now, string(make([]byte, MaxDescriptionRunes)), ""},
		{"description over limit", 60, now, string(make([]byte, MaxDescriptionRunes+1)), CodeDescriptionTooLong},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateEntry(tc.seconds, tc.start, now, tc.desc)
			if tc.code == "" {
				if err != nil {
					t.Fatalf("unexpected %v", err)
				}
				return
			}
			if apperror.CodeOf(err) != tc.code {
				t.Fatalf("code = %q, want %q", apperror.CodeOf(err), tc.code)
			}
		})
	}
}

func TestBuildTimesheetAcrossTimezoneBoundary(t *testing.T) {
	org := uuid.New()
	loc, err := time.LoadLocation("America/Bogota") // UTC-5, no DST
	if err != nil {
		t.Fatal(err)
	}
	zones := map[uuid.UUID]OrgTime{org: {Location: loc, Config: DefaultConfig()}}
	entries := []Entry{
		// 2026-11-26 23:30 in Bogota is 2026-11-27 04:30 UTC: belongs to the 26th locally.
		{OrganizationID: org, StartedAt: time.Date(2026, 11, 27, 4, 30, 0, 0, time.UTC), Seconds: 1800},
		// 2026-11-27 04:59 local is 2026-11-27 09:59 UTC.
		{OrganizationID: org, StartedAt: time.Date(2026, 11, 27, 9, 59, 0, 0, time.UTC), Seconds: 3600},
		// 2026-11-28 00:00 local is 2026-11-28 05:00 UTC: outside 26..27 range.
		{OrganizationID: org, StartedAt: time.Date(2026, 11, 28, 5, 0, 0, 0, time.UTC), Seconds: 600},
	}
	ts := BuildTimesheet(entries, zones, "2026-11-26", "2026-11-27")
	if len(ts.Days) != 2 || ts.Days[0].Date != "2026-11-26" || ts.Days[1].Date != "2026-11-27" {
		t.Fatalf("days = %+v", ts.Days)
	}
	if ts.Days[0].TotalSeconds != 1800 || ts.Days[1].TotalSeconds != 3600 || ts.TotalSeconds != 5400 {
		t.Fatalf("totals = %+v", ts)
	}
	if ts.Days[0].CapacitySeconds != 8*3600 {
		t.Fatalf("capacity = %d", ts.Days[0].CapacitySeconds)
	}
}

func TestBuildTimesheetDefaultsToUTCForUnknownOrg(t *testing.T) {
	e := Entry{OrganizationID: uuid.New(), StartedAt: time.Date(2026, 11, 27, 23, 0, 0, 0, time.UTC), Seconds: 60}
	ts := BuildTimesheet([]Entry{e}, nil, "2026-11-27", "2026-11-27")
	if len(ts.Days) != 1 || ts.Days[0].Date != "2026-11-27" {
		t.Fatalf("days = %+v", ts.Days)
	}
}

func TestDayWindowBracketsRange(t *testing.T) {
	lo, hi, err := DayWindow("2026-11-01", "2026-11-30")
	if err != nil {
		t.Fatal(err)
	}
	if !lo.Before(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)) || !hi.After(time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("window %v..%v does not bracket the range", lo, hi)
	}
	if _, _, err := DayWindow("2026-13-01", "2026-11-30"); err == nil {
		t.Fatalf("invalid from must fail")
	}
}

func TestBuildSummaryGroupings(t *testing.T) {
	org := uuid.New()
	alice, bob := uuid.New(), uuid.New()
	issueA, issueB := uuid.New(), uuid.New()
	base := time.Date(2026, 11, 20, 15, 0, 0, 0, time.UTC)
	entries := []Entry{
		{OrganizationID: org, AuthorID: alice, AuthorName: "Alice", IssueID: issueA, IssueKey: "P-1", IssueTitle: "A", StartedAt: base, Seconds: 3600},
		{OrganizationID: org, AuthorID: bob, AuthorName: "Bob", IssueID: issueA, IssueKey: "P-1", IssueTitle: "A", StartedAt: base.Add(time.Hour), Seconds: 7200},
		{OrganizationID: org, AuthorID: alice, AuthorName: "Alice", IssueID: issueB, IssueKey: "P-2", IssueTitle: "B", StartedAt: base.Add(24 * time.Hour), Seconds: 1800},
		// Outside the range: dropped.
		{OrganizationID: org, AuthorID: alice, AuthorName: "Alice", IssueID: issueB, IssueKey: "P-2", IssueTitle: "B", StartedAt: base.AddDate(0, 0, 30), Seconds: 9999},
	}
	byUser := BuildSummary(entries, nil, GroupByUser, "2026-11-01", "2026-11-30")
	if byUser.TotalSeconds != 12600 || byUser.Count != 3 || len(byUser.Groups) != 2 {
		t.Fatalf("by user = %+v", byUser)
	}
	if byUser.Groups[0].Label != "Bob" || byUser.Groups[0].TotalSeconds != 7200 {
		t.Fatalf("by user order = %+v", byUser.Groups)
	}
	byIssue := BuildSummary(entries, nil, GroupByIssue, "2026-11-01", "2026-11-30")
	if len(byIssue.Groups) != 2 || byIssue.Groups[0].TotalSeconds != 10800 {
		t.Fatalf("by issue = %+v", byIssue.Groups)
	}
	byDay := BuildSummary(entries, nil, GroupByDay, "2026-11-01", "2026-11-30")
	if len(byDay.Groups) != 2 || byDay.Groups[0].Key != "2026-11-20" || byDay.Groups[1].Key != "2026-11-21" {
		t.Fatalf("by day = %+v", byDay.Groups)
	}
	if ValidGroupBy("week") || !ValidGroupBy(GroupByDay) {
		t.Fatalf("group validation")
	}
}

func equalPtr(a, b *int64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func deref(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}
