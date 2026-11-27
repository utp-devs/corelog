package domain

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

func TestResolveAccess(t *testing.T) {
	cases := []struct {
		owner  bool
		shared Permission
		want   Access
	}{
		{true, "", AccessOwner}, {true, PermissionView, AccessOwner}, {false, PermissionEdit, AccessEdit},
		{false, PermissionView, AccessView}, {false, "", AccessNone}, {false, "bogus", AccessNone},
	}
	for _, tc := range cases {
		if got := ResolveAccess(tc.owner, tc.shared); got != tc.want {
			t.Errorf("ResolveAccess(%v, %q) = %v, want %v", tc.owner, tc.shared, got, tc.want)
		}
	}
	if AccessNone.CanView() || !AccessView.CanView() || AccessView.CanEdit() || !AccessEdit.CanEdit() || AccessEdit.CanManage() || !AccessOwner.CanManage() {
		t.Fatal("access ladder is wrong")
	}
}

func TestFilterFieldsNormalize(t *testing.T) {
	got, err := FilterFields{Name: "  Mi filtro ", Description: " d ", QueryText: " a = b ", Sort: " key ", Columns: []string{" key ", "KEY", "summary"}}.Normalize()
	if err != nil || got.Name != "Mi filtro" || got.QueryText != "a = b" || got.Sort != "key" || len(got.Columns) != 2 || got.Columns[0] != "key" {
		t.Fatalf("%v %+v", err, got)
	}
	long := func(n int) string { return strings.Repeat("a", n) }
	bad := []FilterFields{
		{Name: ""}, {Name: "   "}, {Name: long(121)}, {Name: "x", Description: long(501)}, {Name: "x", QueryText: long(4001)},
		{Name: "x", Sort: long(201)}, {Name: "x", Columns: []string{""}}, {Name: "x", Columns: []string{long(81)}},
	}
	many := make([]string, 31)
	for i := range many {
		many[i] = "c" + strings.Repeat("x", i)
	}
	bad = append(bad, FilterFields{Name: "x", Columns: many})
	for i, f := range bad {
		_, err := f.Normalize()
		if apperror.CodeOf(err) != CodeFilterInvalid || apperror.KindOf(err) != apperror.KindValidation || len(apperror.DetailsOf(err)) == 0 {
			t.Errorf("case %d accepted: %v", i, err)
		}
	}
}

func TestNormalizeShares(t *testing.T) {
	id := uuid.New()
	out, err := NormalizeShares([]Share{{Type: ShareUser, TargetID: id}, {Type: ShareUser, TargetID: id, Permission: PermissionEdit}, {Type: ShareTeam, TargetID: id, Permission: PermissionEdit}})
	if err != nil || len(out) != 2 || out[0].Permission != PermissionView || out[1].Type != ShareTeam {
		t.Fatalf("%v %+v", err, out)
	}
	for _, bad := range [][]Share{
		{{Type: "x", TargetID: id}}, {{Type: ShareUser, TargetID: uuid.Nil}}, {{Type: ShareUser, TargetID: id, Permission: "own"}},
	} {
		if _, err := NormalizeShares(bad); apperror.CodeOf(err) != CodeShareInvalid {
			t.Errorf("%+v accepted: %v", bad, err)
		}
	}
	if _, err := NormalizeShares(make([]Share, 101)); apperror.CodeOf(err) != CodeShareInvalid {
		t.Fatal("cap not enforced")
	}
	if !ShareProject.IsValid() || ShareType("z").IsValid() || !PermissionEdit.IsValid() || Permission("z").IsValid() {
		t.Fatal("validity helpers")
	}
}

func TestScheduleValidate(t *testing.T) {
	ok, err := Schedule{Frequency: FrequencyDaily, Hour: 9, Weekday: 4}.Validate()
	if err != nil || ok.Timezone != "UTC" || ok.Weekday != 0 {
		t.Fatalf("%v %+v", err, ok)
	}
	if w, err := (Schedule{Frequency: FrequencyWeekly, Hour: 23, Weekday: 6, Timezone: "Europe/Madrid"}).Validate(); err != nil || w.Weekday != 6 {
		t.Fatalf("%v %+v", err, w)
	}
	for _, bad := range []Schedule{
		{}, {Frequency: "monthly"}, {Frequency: FrequencyDaily, Hour: -1}, {Frequency: FrequencyDaily, Hour: 24},
		{Frequency: FrequencyWeekly, Weekday: -1}, {Frequency: FrequencyWeekly, Weekday: 7}, {Frequency: FrequencyDaily, Timezone: "Nowhere/City"},
	} {
		if _, err := bad.Validate(); apperror.CodeOf(err) != CodeSubscriptionInvalid {
			t.Errorf("%+v accepted: %v", bad, err)
		}
	}
}

func TestScheduleLastOccurrence(t *testing.T) {
	utc := func(y int, m time.Month, d, h, min int) time.Time { return time.Date(y, m, d, h, min, 0, 0, time.UTC) }
	daily := Schedule{Frequency: FrequencyDaily, Hour: 9, Timezone: "UTC"}
	cases := []struct {
		name string
		s    Schedule
		now  time.Time
		want time.Time
	}{
		{"daily after the slot", daily, utc(2026, 11, 27, 15, 30), utc(2026, 11, 27, 9, 0)},
		{"daily before the slot", daily, utc(2026, 11, 27, 8, 59), utc(2026, 11, 26, 9, 0)},
		{"daily at the slot", daily, utc(2026, 11, 27, 9, 0), utc(2026, 11, 27, 9, 0)},
		{"weekly same day after", Schedule{Frequency: FrequencyWeekly, Hour: 8, Weekday: 5, Timezone: "UTC"}, utc(2026, 11, 27, 15, 0), utc(2026, 11, 27, 8, 0)},
		{"weekly same day before", Schedule{Frequency: FrequencyWeekly, Hour: 8, Weekday: 5, Timezone: "UTC"}, utc(2026, 11, 27, 7, 0), utc(2026, 11, 20, 8, 0)},
		{"weekly earlier weekday", Schedule{Frequency: FrequencyWeekly, Hour: 8, Weekday: 1, Timezone: "UTC"}, utc(2026, 11, 27, 7, 0), utc(2026, 11, 23, 8, 0)},
		{"weekly later weekday", Schedule{Frequency: FrequencyWeekly, Hour: 8, Weekday: 0, Timezone: "UTC"}, utc(2026, 11, 27, 7, 0), utc(2026, 11, 22, 8, 0)},
	}
	for _, tc := range cases {
		if got := tc.s.LastOccurrence(tc.now); !got.Equal(tc.want) {
			t.Errorf("%s: %v, want %v", tc.name, got, tc.want)
		}
	}
	// Time zones shift the slot: 09:00 in Bogota (UTC-5) is 14:00 UTC.
	bogota := Schedule{Frequency: FrequencyDaily, Hour: 9, Timezone: "America/Bogota"}
	if got := bogota.LastOccurrence(utc(2026, 11, 27, 15, 0)); !got.Equal(utc(2026, 11, 27, 14, 0)) {
		t.Errorf("bogota: %v", got)
	}
	// An invalid zone falls back to UTC instead of failing.
	if got := (Schedule{Frequency: FrequencyDaily, Hour: 9, Timezone: "bad"}).LastOccurrence(utc(2026, 11, 27, 15, 0)); !got.Equal(utc(2026, 11, 27, 9, 0)) {
		t.Errorf("fallback: %v", got)
	}
}

func TestSubscriptionIsDue(t *testing.T) {
	created := time.Date(2026, 11, 27, 15, 30, 0, 0, time.UTC)
	sub := Subscription{Schedule: Schedule{Frequency: FrequencyDaily, Hour: 9, Timezone: "UTC"}, CreatedAt: created}
	if sub.IsDue(created) || sub.IsDue(created.Add(17*time.Hour)) {
		t.Fatal("a new subscription waits for its next slot")
	}
	next := time.Date(2026, 11, 28, 9, 0, 1, 0, time.UTC)
	if !sub.IsDue(next) {
		t.Fatal("due after the next slot")
	}
	sent := next
	sub.LastSentAt = &sent
	if sub.IsDue(next.Add(time.Hour)) {
		t.Fatal("not due again right after a delivery")
	}
	if !sub.IsDue(next.Add(24 * time.Hour)) {
		t.Fatal("due again at the following slot")
	}
	// A stale last_sent_at older than the creation does not reset the baseline.
	old := created.Add(-48 * time.Hour)
	sub.LastSentAt = &old
	if sub.IsDue(created.Add(time.Hour)) {
		t.Fatal("baseline is the creation time")
	}
}

func TestQueryError(t *testing.T) {
	qe := NewQueryError([]Problem{{Pos: 3, Len: 2, Code: CodeUnknownField, Message: "campo desconocido"}})
	if apperror.CodeOf(qe) != CodeUnknownField || apperror.KindOf(qe) != apperror.KindValidation || qe.Error() != "campo desconocido" {
		t.Fatalf("%v", qe)
	}
	var target *QueryError
	if !errors.As(error(qe), &target) || len(target.Problems) != 1 {
		t.Fatal("errors.As")
	}
	empty := NewQueryError(nil)
	if apperror.CodeOf(empty) != CodeInvalidQuery || empty.Error() == "" {
		t.Fatalf("%v", empty)
	}
	noCode := NewQueryError([]Problem{{Message: "x"}})
	if apperror.CodeOf(noCode) != CodeInvalidQuery {
		t.Fatalf("%v", noCode)
	}
}

func TestParseKinds(t *testing.T) {
	cases := map[string][]Kind{
		"": DefaultKinds, "all": AllKinds, "nonsense": DefaultKinds, "issue": {KindIssue},
		"person, teams ,Filter": {KindPerson, KindTeam, KindFilter}, "notes,project,issue,issue": {KindProject, KindIssue, KindNote},
		"users,all": AllKinds, "projects": {KindProject},
	}
	for in, want := range cases {
		got := ParseKinds(in)
		if len(got) != len(want) {
			t.Errorf("ParseKinds(%q) = %v, want %v", in, got, want)
			continue
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("ParseKinds(%q) = %v, want %v", in, got, want)
			}
		}
	}
}

func TestHighlightHTML(t *testing.T) {
	cases := []struct {
		text  string
		terms []string
		want  string
	}{
		{"Login falla", []string{"login"}, "<mark>Login</mark> falla"},
		{"a <b>login</b> & login", []string{"login"}, "a &lt;b&gt;<mark>login</mark>&lt;/b&gt; &amp; <mark>login</mark>"},
		{"móvil MÓVIL", []string{"móvil"}, "<mark>móvil</mark> <mark>MÓVIL</mark>"},
		{"abcabc", []string{"bc", "abc"}, "<mark>abcabc</mark>"},
		{"overlap", []string{"over", "erl"}, "<mark>overl</mark>ap"},
		{"nada", nil, "nada"},
		{"nada", []string{""}, "nada"},
		{"x", []string{"longer than text"}, "x"},
		{`"quoted" 'single'`, []string{"zz"}, "&#34;quoted&#34; &#39;single&#39;"},
	}
	for _, tc := range cases {
		if got := HighlightHTML(tc.text, tc.terms); got != tc.want {
			t.Errorf("HighlightHTML(%q, %v) = %q, want %q", tc.text, tc.terms, got, tc.want)
		}
	}
}

func TestSnippetHTML(t *testing.T) {
	if got := SnippetHTML("hola " + MarkStart + "mundo" + MarkEnd + " <script>x</script>\x00\x01 fin  \n  ok"); got != "hola <mark>mundo</mark> &lt;script&gt;x&lt;/script&gt; fin ok" {
		t.Fatalf("%q", got)
	}
	if SnippetHTML("") != "" {
		t.Fatal("empty")
	}
}

func TestSearchTerms(t *testing.T) {
	got := SearchTerms(`  login "falla móvil" -ruido OR x  Login a`)
	want := []string{"login", "falla", "móvil", "ruido"}
	if len(got) != len(want) {
		t.Fatalf("%v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%v", got)
		}
	}
}
