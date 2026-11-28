package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func utcParams(kind Kind, from, to string) Params {
	p, err := ResolveParams(kind, RawParams{From: from, To: to}, time.UTC, ts("2024-03-01T12:00:00Z"))
	if err != nil {
		panic(err)
	}
	return p
}

func TestResolveParams(t *testing.T) {
	now := ts("2024-03-10T12:00:00Z")
	p, err := ResolveParams(KindCumulativeFlow, RawParams{}, time.UTC, now)
	if err != nil || p.FromDate.Format(dateLayout) != "2024-02-10" || p.ToDate.Format(dateLayout) != "2024-03-10" || p.Days() != 30 {
		t.Fatalf("defaults = %+v %v", p, err)
	}
	cases := map[string]struct {
		kind Kind
		raw  RawParams
		want error
	}{
		"bad from":       {KindControl, RawParams{From: "2024-13-01"}, ErrRangeInvalid},
		"bad to":         {KindControl, RawParams{To: "ayer"}, ErrRangeInvalid},
		"from after to":  {KindControl, RawParams{From: "2024-03-02", To: "2024-03-01"}, ErrRangeInvalid},
		"too long":       {KindControl, RawParams{From: "2023-01-01", To: "2024-01-02"}, ErrRangeTooLarge},
		"sql in from":    {KindControl, RawParams{From: "2024-01-01'; DROP TABLE issues;--"}, ErrRangeInvalid},
		"group_by":       {KindTimeTracking, RawParams{GroupBy: "week"}, ErrParamInvalid},
		"burn group":     {KindBurndown, RawParams{GroupBy: "hours"}, ErrParamInvalid},
		"interval":       {KindCreatedVsRes, RawParams{Interval: "year"}, ErrParamInvalid},
		"sprint uuid":    {KindBurndown, RawParams{Sprint: "no"}, ErrParamInvalid},
		"nil sprint":     {KindBurndown, RawParams{Sprint: uuid.Nil.String()}, ErrParamInvalid},
		"version uuid":   {KindVersion, RawParams{Version: "1 OR 1=1"}, ErrParamInvalid},
		"version needed": {KindVersion, RawParams{}, ErrVersionRequired},
		"limit zero":     {KindVelocity, RawParams{Limit: "0"}, ErrParamInvalid},
		"limit big":      {KindVelocity, RawParams{Limit: "21"}, ErrParamInvalid},
		"limit overflow": {KindVelocity, RawParams{Limit: "99999999999999999999"}, ErrParamInvalid},
	}
	for name, c := range cases {
		if _, err := ResolveParams(c.kind, c.raw, time.UTC, now); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", name, err, c.want)
		}
	}
	if _, err := ResolveParams(KindControl, RawParams{From: "2024-01-01", To: "2024-12-31"}, time.UTC, now); err != nil {
		t.Fatalf("366 days must be accepted: %v", err)
	}
	long := make([]rune, MaxFilterRunes+1)
	for i := range long {
		long[i] = 'a'
	}
	if _, err := ResolveParams(KindWorkload, RawParams{Filter: string(long)}, time.UTC, now); !errors.Is(err, ErrFilterInvalid) {
		t.Fatalf("long filter: %v", err)
	}
	if p, _ := ResolveParams(KindControl, RawParams{To: "2024-03-05"}, time.UTC, now); p.FromDate.Format(dateLayout) != "2024-02-05" {
		t.Fatalf("from defaults to 30 days before to: %v", p.FromDate)
	}
	echo, _ := ResolveParams(KindVelocity, RawParams{}, time.UTC, now)
	if echo.Echo()["limit"] != DefaultSprints {
		t.Fatalf("echo = %v", echo.Echo())
	}
}

func TestParamsTimezoneWindow(t *testing.T) {
	bogota, _ := time.LoadLocation("America/Bogota")
	p, err := ResolveParams(KindControl, RawParams{From: "2024-01-01", To: "2024-01-01"}, bogota, ts("2024-03-01T00:00:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	if !p.Lo.Equal(ts("2024-01-01T05:00:00Z")) || !p.Hi.Equal(ts("2024-01-02T05:00:00Z")) {
		t.Fatalf("window = %v - %v", p.Lo, p.Hi)
	}
	// Daylight saving: a 23 hour day in Madrid.
	madrid, _ := time.LoadLocation("Europe/Madrid")
	p, _ = ResolveParams(KindControl, RawParams{From: "2024-03-31", To: "2024-03-31"}, madrid, ts("2024-06-01T00:00:00Z"))
	if p.Hi.Sub(p.Lo) != 23*time.Hour {
		t.Fatalf("dst day = %v", p.Hi.Sub(p.Lo))
	}
	if (ProjectInfo{Timezone: "No/Existe"}).Location() != time.UTC || (ProjectInfo{}).Location() != time.UTC {
		t.Fatal("unknown zone must fall back to UTC")
	}
}

func TestCatalogAndKinds(t *testing.T) {
	if len(Catalog()) != 11 {
		t.Fatalf("catalog = %d", len(Catalog()))
	}
	for _, d := range Catalog() {
		if k, ok := ParseKind(string(d.Kind)); !ok || k != d.Kind || d.Description == "" || TitleOf(k) != d.Title {
			t.Fatalf("kind %s", d.Kind)
		}
	}
	if _, ok := ParseKind("nope"); ok {
		t.Fatal("unknown kind accepted")
	}
	if TitleOf("x") != "x" {
		t.Fatal("unknown title")
	}
}

func TestCumulativeFlowGolden(t *testing.T) {
	// Range 01-01..01-03 UTC. A created 12-31, in progress 01-01 10:00, done 01-03 08:00.
	// B created 01-02 09:00 with no history (todo). C created 01-03 with no history but resolved 01-03 20:00.
	id := ids(3)
	issues := []IssueRow{
		{ID: id[0], CreatedAt: ts("2023-12-31T10:00:00Z")},
		{ID: id[1], CreatedAt: ts("2024-01-02T09:00:00Z")},
		{ID: id[2], CreatedAt: ts("2024-01-03T01:00:00Z"), ResolvedAt: tp("2024-01-03T20:00:00Z")},
	}
	history := []StatusChange{{id[0], ts("2024-01-01T10:00:00Z"), "in_progress"}, {id[0], ts("2024-01-03T08:00:00Z"), "done"}}
	r := BuildCumulativeFlow(utcParams(KindCumulativeFlow, "2024-01-01", "2024-01-03"), issues, history)
	eq(t, "todo", ys(r.Series[0]), []float64{0, 1, 1})
	eq(t, "in_progress", ys(r.Series[1]), []float64{1, 1, 0})
	eq(t, "done", ys(r.Series[2]), []float64{0, 0, 2})
	if r.Summary["total"] != 3 || r.Summary["done"] != 2 || r.Summary["average_in_progress"] != 0.67 {
		t.Fatalf("summary = %v", r.Summary)
	}
	empty := BuildCumulativeFlow(utcParams(KindCumulativeFlow, "2024-01-01", "2024-01-02"), nil, nil)
	eq(t, "empty", ys(empty.Series[0]), []float64{0, 0})
}

func TestCumulativeFlowReopen(t *testing.T) {
	id := uuid.New()
	issues := []IssueRow{{ID: id, CreatedAt: ts("2023-12-01T00:00:00Z")}}
	history := []StatusChange{{id, ts("2024-01-01T10:00:00Z"), "done"}, {id, ts("2024-01-02T10:00:00Z"), "in_progress"}}
	r := BuildCumulativeFlow(utcParams(KindCumulativeFlow, "2024-01-01", "2024-01-02"), issues, history)
	eq(t, "done", ys(r.Series[2]), []float64{1, 0})
	eq(t, "in_progress", ys(r.Series[1]), []float64{0, 1})
}

func TestCreatedVsResolved(t *testing.T) {
	// Week of 2024-01-01 (Monday) and the next one.
	issues := []IssueRow{
		{CreatedAt: ts("2024-01-01T10:00:00Z")},
		{CreatedAt: ts("2024-01-03T10:00:00Z"), ResolvedAt: tp("2024-01-09T10:00:00Z")},
		{CreatedAt: ts("2023-12-20T10:00:00Z"), ResolvedAt: tp("2024-01-02T10:00:00Z")},
		{CreatedAt: ts("2024-01-07T23:59:59Z")},
		{CreatedAt: ts("2024-01-15T00:00:00Z")}, // outside the range
	}
	r := BuildCreatedVsResolved(func() Params {
		p := utcParams(KindCreatedVsRes, "2024-01-01", "2024-01-14")
		p.Interval = "week"
		return p
	}(), issues)
	eq(t, "created", ys(r.Series[0]), []float64{3, 0})
	eq(t, "resolved", ys(r.Series[1]), []float64{1, 1})
	eq(t, "cumulative", ys(r.Series[2]), []float64{2, 1})
	if r.Summary["created"] != 3 || r.Summary["resolved"] != 2 || r.Summary["net"] != 1 {
		t.Fatalf("summary = %v", r.Summary)
	}
	p := utcParams(KindCreatedVsRes, "2024-01-30", "2024-02-02")
	p.Interval = "month"
	m := BuildCreatedVsResolved(p, nil)
	if len(m.Series[0].Points) != 2 || m.Series[0].Points[0].X != "2024-01" {
		t.Fatalf("months = %+v", m.Series[0])
	}
	p.Interval = "day"
	if d := BuildCreatedVsResolved(p, nil); len(d.Series[0].Points) != 4 {
		t.Fatalf("days = %d", len(d.Series[0].Points))
	}
}

func TestCreatedVsResolvedTimezoneBoundary(t *testing.T) {
	bogota, _ := time.LoadLocation("America/Bogota")
	p, _ := ResolveParams(KindCreatedVsRes, RawParams{From: "2024-01-01", To: "2024-01-02"}, bogota, ts("2024-03-01T00:00:00Z"))
	// 03:00Z on 01-02 is 22:00 local on 01-01; 05:00Z on 01-01 is the first local instant of the range.
	issues := []IssueRow{{CreatedAt: ts("2024-01-02T03:00:00Z")}, {CreatedAt: ts("2024-01-01T04:59:59Z")}}
	r := BuildCreatedVsResolved(p, issues)
	eq(t, "created", ys(r.Series[0]), []float64{1, 0})
}

func TestControlChartGolden(t *testing.T) {
	// Cycle times of 2, 4 and 6 days (start = first in_progress, or creation without one).
	id := ids(4)
	issues := []IssueRow{
		{ID: id[0], Key: "A-1", CreatedAt: ts("2024-01-01T00:00:00Z"), ResolvedAt: tp("2024-01-03T00:00:00Z")}, // 2 days from creation
		{ID: id[1], Key: "A-2", CreatedAt: ts("2024-01-01T00:00:00Z"), ResolvedAt: tp("2024-01-08T00:00:00Z")}, // started 01-04: 4 days
		{ID: id[2], Key: "A-3", CreatedAt: ts("2024-01-01T00:00:00Z"), ResolvedAt: tp("2024-01-12T00:00:00Z")}, // started 01-06: 6 days
		{ID: id[3], Key: "A-4", CreatedAt: ts("2024-01-01T00:00:00Z")},                                         // unresolved: ignored
	}
	history := []StatusChange{
		{id[1], ts("2024-01-04T00:00:00Z"), "in_progress"}, {id[1], ts("2024-01-08T00:00:00Z"), "done"},
		{id[2], ts("2024-01-06T00:00:00Z"), "in_progress"}, {id[2], ts("2024-01-12T00:00:00Z"), "done"},
	}
	r := BuildControl(utcParams(KindControl, "2024-01-01", "2024-01-31"), issues, history)
	eq(t, "cycle", ys(r.Series[0]), []float64{2, 4, 6})
	eq(t, "average", ys(r.Series[1]), []float64{2, 3, 4})
	// Population standard deviations: 0, 1, sqrt(8/3)=1.63.
	eq(t, "upper", ys(r.Series[2]), []float64{2, 4, 5.63})
	eq(t, "lower", ys(r.Series[3]), []float64{2, 2, 2.37})
	if r.Summary["count"] != 3 || r.Summary["average_days"] != 4.0 || r.Summary["median_days"] != 4.0 || r.Summary["std_dev_days"] != 1.63 || r.Summary["p85_days"] != 6.0 {
		t.Fatalf("summary = %v", r.Summary)
	}
	empty := BuildControl(utcParams(KindControl, "2024-01-01", "2024-01-31"), nil, nil)
	if len(empty.Series) != 0 || empty.Summary["count"] != 0 || empty.Summary["average_days"] != 0.0 {
		t.Fatalf("empty = %+v", empty)
	}
}

func TestControlWindowAndNegativeCycle(t *testing.T) {
	var issues []IssueRow
	var history []StatusChange
	for i := 0; i < 12; i++ {
		id := uuid.New()
		resolved := ts("2024-01-01T00:00:00Z").AddDate(0, 0, i+2)
		issues = append(issues, IssueRow{ID: id, Key: "K", CreatedAt: ts("2024-01-01T00:00:00Z"), ResolvedAt: &resolved})
	}
	// A transition into progress after resolution is ignored in favor of the creation date.
	history = append(history, StatusChange{issues[0].ID, ts("2024-02-01T00:00:00Z"), "in_progress"})
	r := BuildControl(utcParams(KindControl, "2024-01-01", "2024-01-31"), issues, history)
	if len(r.Series[1].Points) != 12 || r.Series[0].Points[0].Y != 2 {
		t.Fatalf("series = %+v", r.Series[0].Points[0])
	}
	// The window is the last 10 values: 4..13 average 8.5 at the end.
	if got := r.Series[1].Points[11].Y; got != 8.5 {
		t.Fatalf("rolling average = %v, want 8.5", got)
	}
}

func TestResolutionTime(t *testing.T) {
	mk := func(typ, prio string, days int) IssueRow {
		resolved := ts("2024-01-02T00:00:00Z").AddDate(0, 0, days)
		return IssueRow{TypeName: typ, PriorityName: prio, CreatedAt: ts("2024-01-02T00:00:00Z"), ResolvedAt: &resolved}
	}
	issues := []IssueRow{mk("Bug", "Alta", 1), mk("Bug", "Baja", 3), mk("Bug", "Alta", 8), mk("Tarea", "Baja", 10), {TypeName: "Bug", CreatedAt: ts("2024-01-02T00:00:00Z")}}
	p := utcParams(KindResolutionTime, "2024-01-01", "2024-02-28")
	r := BuildResolutionTime(p, issues)
	// Bug: 1, 3, 8 -> avg 4, median 3, p90 8. Tarea: 10.
	if len(r.Table.Rows) != 2 || r.Table.Rows[0][0] != "Bug" || r.Table.Rows[0][2] != 4.0 || r.Table.Rows[0][3] != 3.0 || r.Table.Rows[0][4] != 8.0 {
		t.Fatalf("rows = %v", r.Table.Rows)
	}
	// All four: 1, 3, 8, 10 -> avg 5.5, median 5.5, p90 10.
	if r.Summary["count"] != 4 || r.Summary["average_days"] != 5.5 || r.Summary["median_days"] != 5.5 || r.Summary["p90_days"] != 10.0 {
		t.Fatalf("summary = %v", r.Summary)
	}
	p.GroupBy = "priority"
	rp := BuildResolutionTime(p, issues)
	if rp.Table.Columns[0].Label != "Prioridad" || rp.Table.Rows[0][0] != "Alta" {
		t.Fatalf("by priority = %v", rp.Table.Rows)
	}
	if e := BuildResolutionTime(p, nil); len(e.Series) != 0 || e.Summary["average_days"] != 0.0 {
		t.Fatalf("empty = %+v", e)
	}
}

func TestTimeTracking(t *testing.T) {
	i1, i2 := uuid.New(), uuid.New()
	u1, u2 := uuid.New(), uuid.New()
	est := func(v int64) *int64 { return &v }
	rows := []WorklogRow{
		{IssueID: i1, IssueKey: "A-1", Summary: "uno", AuthorID: u1, AuthorName: "Ana", Seconds: 7200, OriginalEst: est(14400), RemainingEst: est(7200)},
		{IssueID: i1, IssueKey: "A-1", Summary: "uno", AuthorID: u2, AuthorName: "Beto", Seconds: 3600, OriginalEst: est(14400)},
		{IssueID: i2, IssueKey: "A-2", Summary: "dos", AuthorID: u1, AuthorName: "Ana", Seconds: 1800},
	}
	p := utcParams(KindTimeTracking, "2024-01-01", "2024-01-31")
	r := BuildTimeTracking(p, rows)
	// A-1: estimate 4h, spent 3h -> 75%; A-2: no estimate -> null.
	if r.Table.Rows[0][0] != "A-1" || r.Table.Rows[0][2] != 4.0 || r.Table.Rows[0][3] != 3.0 || r.Table.Rows[0][5] != 75.0 || r.Table.Rows[1][5] != nil {
		t.Fatalf("rows = %v", r.Table.Rows)
	}
	if r.Summary["spent_hours"] != 3.5 || r.Summary["estimate_hours"] != 4.0 || r.Summary["accuracy_percent"] != 87.5 || r.Summary["issues"] != 2 || r.Summary["users"] != 2 {
		t.Fatalf("summary = %v", r.Summary)
	}
	p.GroupBy = "user"
	ru := BuildTimeTracking(p, rows)
	if ru.Table.Rows[0][0] != "Ana" || ru.Table.Rows[0][1] != 2 || ru.Table.Rows[0][3] != 2.5 || len(ru.Series) != 1 {
		t.Fatalf("by user = %v", ru.Table.Rows)
	}
	e := BuildTimeTracking(p, nil)
	if len(e.Series) != 0 || e.Summary["accuracy_percent"] != nil || e.Summary["spent_hours"] != 0.0 {
		t.Fatalf("empty = %+v", e)
	}
}

func TestWorkload(t *testing.T) {
	ana, beto := uuid.New(), uuid.New()
	h := func(v int64) *int64 { return &v }
	issues := []IssueRow{
		{AssigneeID: &ana, AssigneeName: "Ana", Category: "todo", Points: fp(3), RemainingEst: h(7200)},
		{AssigneeID: &ana, AssigneeName: "Ana", Category: "in_progress", Points: fp(2)},
		{AssigneeID: &beto, AssigneeName: "Beto", Category: "todo", RemainingEst: h(3600)},
		{Category: "todo"},
		{AssigneeID: &beto, AssigneeName: "Beto", Category: "done", RemainingEst: h(99999)},
	}
	r := BuildWorkload(issues)
	if len(r.Table.Rows) != 3 || r.Table.Rows[0][0] != "Ana" || r.Table.Rows[0][1] != 2 || r.Table.Rows[0][2] != 1 || r.Table.Rows[0][3] != 5.0 || r.Table.Rows[0][4] != 2.0 || r.Table.Rows[0][5] != 1 {
		t.Fatalf("rows = %v", r.Table.Rows)
	}
	if r.Summary["open"] != 4 || r.Summary["unassigned"] != 1 || r.Summary["assignees"] != 2 || r.Summary["remaining_hours"] != 3.0 {
		t.Fatalf("summary = %v", r.Summary)
	}
	if e := BuildWorkload(nil); len(e.Series) != 0 || e.Summary["open"] != 0 {
		t.Fatalf("empty = %+v", e)
	}
}

func TestVersionReport(t *testing.T) {
	ana := uuid.New()
	issues := []IssueRow{
		{Key: "A-1", Category: "done", Points: fp(5), AssigneeID: &ana, AssigneeName: "Ana"},
		{Key: "A-2", Category: "in_progress", Points: fp(3)},
		{Key: "A-3", Category: "todo"},
		{Key: "A-4", Category: "todo", Points: fp(2)},
	}
	v := VersionInfo{ID: uuid.New(), Name: "1.0", Status: "unreleased", StartDate: tp("2024-01-01T00:00:00Z"), ReleaseDate: tp("2024-03-01T00:00:00Z")}
	now := ts("2024-03-04T10:00:00Z")
	r := BuildVersion(v, issues, time.UTC, now)
	if r.Summary["total"] != 4 || r.Summary["done"] != 1 || r.Summary["percent_done"] != 25.0 || r.Summary["points_total"] != 10.0 || r.Summary["percent_points_done"] != 50.0 {
		t.Fatalf("summary = %v", r.Summary)
	}
	if r.Summary["overdue"] != true || r.Summary["days_to_release"] != -3 || r.Summary["release_date"] != "2024-03-01" {
		t.Fatalf("dates = %v", r.Summary)
	}
	if r.Table.Rows[0][5] != "Ana" || r.Table.Rows[1][5] != "Sin asignar" || r.Table.Rows[0][3] != "Hecho" {
		t.Fatalf("rows = %v", r.Table.Rows)
	}
	v.Status = "released"
	if BuildVersion(v, nil, time.UTC, now).Summary["overdue"] != false {
		t.Fatal("released version is never overdue")
	}
	e := BuildVersion(VersionInfo{Name: "x"}, nil, time.UTC, now)
	if e.Summary["percent_done"] != 0.0 || e.Summary["release_date"] != nil || len(e.Series) != 2 {
		t.Fatalf("empty = %+v", e.Summary)
	}
}

func TestHelpers(t *testing.T) {
	if percentile(nil, 0.5) != 0 || percentile([]float64{1, 2}, 0) != 1 || percentile([]float64{1, 2}, 2) != 2 {
		t.Fatal("percentile bounds")
	}
	if median([]float64{1, 2}) != 1.5 || median(nil) != 0 || mean(nil) != 0 || stdDev(nil) != 0 {
		t.Fatal("stats")
	}
	if daysBetween(ts("2024-01-02T00:00:00Z"), ts("2024-01-01T00:00:00Z")) != 0 {
		t.Fatal("negative durations clamp to zero")
	}
	if bucketKey(ts("2024-01-07T00:00:00Z"), "week") != "2024-01-01" || bucketKey(ts("2024-01-08T00:00:00Z"), "week") != "2024-01-08" {
		t.Fatal("weeks start on Monday")
	}
}
