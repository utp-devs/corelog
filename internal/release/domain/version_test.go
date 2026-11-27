package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

var (
	testNow     = time.Date(2026, 11, 27, 10, 30, 0, 0, time.UTC)
	testProject = uuid.MustParse("00000000-0000-4000-8000-000000000001")
)

func day(y int, m time.Month, d int) *time.Time {
	t := time.Date(y, m, d, 15, 0, 0, 0, time.UTC)
	return &t
}

func codeOf(err error) string { return apperror.CodeOf(err) }

func TestNewVersionValidation(t *testing.T) {
	long := strings.Repeat("a", MaxVersionNameLength+1)
	tests := []struct {
		name    string
		input   string
		start   *time.Time
		release *time.Time
		code    string
	}{
		{"valid", "1.0.0", nil, nil, ""},
		{"blank name", "   ", nil, nil, "version.invalid"},
		{"name too long", long, nil, nil, "version.invalid"},
		{"release before start", "2.0", day(2026, 5, 2), day(2026, 5, 1), "version.invalid_dates"},
		{"same day is fine", "2.1", day(2026, 5, 1), day(2026, 5, 1), ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v, err := NewVersion(testProject, tc.input, "", tc.start, tc.release, testNow)
			if tc.code == "" {
				if err != nil {
					t.Fatalf("unexpected error %v", err)
				}
				if v.Status != StatusUnreleased || v.Name != strings.TrimSpace(tc.input) {
					t.Fatalf("bad version %+v", v)
				}
				return
			}
			if codeOf(err) != tc.code {
				t.Fatalf("code = %q, want %q (err %v)", codeOf(err), tc.code, err)
			}
		})
	}
}

func TestNewVersionRejectsNilProject(t *testing.T) {
	if _, err := NewVersion(uuid.Nil, "1.0", "", nil, nil, testNow); codeOf(err) != "version.invalid" {
		t.Fatalf("code = %q", codeOf(err))
	}
}

// TestVersionLifecycle walks the states: unreleased -> released -> archived, plus the reopen path and the
// transitions that must be refused.
func TestVersionLifecycle(t *testing.T) {
	type step struct {
		name  string
		apply func(v *Version) error
		want  Status
		code  string
	}
	release := func(v *Version) error { return v.Release(nil, testNow) }
	archive := func(v *Version) error { return v.Archive(testNow) }
	unrelease := func(v *Version) error { return v.Unrelease(testNow) }
	tests := []struct {
		name  string
		steps []step
	}{
		{"unreleased to released to archived", []step{
			{"release", release, StatusReleased, ""},
			{"archive", archive, StatusArchived, ""},
		}},
		{"unreleased can be archived directly", []step{
			{"archive", archive, StatusArchived, ""},
		}},
		{"release twice is refused", []step{
			{"release", release, StatusReleased, ""},
			{"release again", release, StatusReleased, "version.invalid_transition"},
		}},
		{"released can be reopened", []step{
			{"release", release, StatusReleased, ""},
			{"unrelease", unrelease, StatusUnreleased, ""},
		}},
		{"unreleasing an unreleased version is refused", []step{
			{"unrelease", unrelease, StatusUnreleased, "version.invalid_transition"},
		}},
		{"archived is terminal", []step{
			{"archive", archive, StatusArchived, ""},
			{"release", release, StatusArchived, "version.archived"},
			{"unrelease", unrelease, StatusArchived, "version.archived"},
			{"archive again", archive, StatusArchived, "version.archived"},
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v, err := NewVersion(testProject, "1.0", "", nil, nil, testNow)
			if err != nil {
				t.Fatal(err)
			}
			for _, s := range tc.steps {
				err := s.apply(v)
				if codeOf(err) != s.code {
					t.Fatalf("%s: code = %q, want %q", s.name, codeOf(err), s.code)
				}
				if v.Status != s.want {
					t.Fatalf("%s: status = %s, want %s", s.name, v.Status, s.want)
				}
			}
		})
	}
}

func TestReleaseSetsDatesAndDefaultsToToday(t *testing.T) {
	v, _ := NewVersion(testProject, "1.0", "", day(2026, 11, 1), nil, testNow)
	if err := v.Release(nil, testNow); err != nil {
		t.Fatal(err)
	}
	if FormatDate(v.ReleaseDate) != "2026-11-27" || v.ReleasedAt == nil || !v.ReleasedAt.Equal(testNow) {
		t.Fatalf("release date %v released at %v", v.ReleaseDate, v.ReleasedAt)
	}
	v2, _ := NewVersion(testProject, "2.0", "", day(2026, 11, 1), nil, testNow)
	if err := v2.Release(day(2026, 10, 31), testNow); codeOf(err) != "version.invalid_dates" {
		t.Fatalf("release before start accepted: %v", err)
	}
}

func TestUpdateRules(t *testing.T) {
	v, _ := NewVersion(testProject, "1.0", "old", nil, nil, testNow)
	newName := "1.1"
	if err := v.Update(VersionPatch{Name: &newName}, testNow); err != nil || v.Name != "1.1" {
		t.Fatalf("rename: %v %q", err, v.Name)
	}
	released := StatusReleased
	if err := v.Update(VersionPatch{Status: &released}, testNow); codeOf(err) != "version.invalid_transition" {
		t.Fatalf("status released via patch: %v", err)
	}
	bogus := Status("bogus")
	if err := v.Update(VersionPatch{Status: &bogus}, testNow); codeOf(err) != "version.status_invalid" {
		t.Fatalf("bogus status: %v", err)
	}
	if err := v.Release(nil, testNow); err != nil {
		t.Fatal(err)
	}
	unreleased := StatusUnreleased
	if err := v.Update(VersionPatch{Status: &unreleased}, testNow); err != nil || v.Status != StatusUnreleased || v.ReleasedAt != nil {
		t.Fatalf("reopen via patch: %v %s", err, v.Status)
	}
	if err := v.Archive(testNow); err != nil {
		t.Fatal(err)
	}
	if err := v.Update(VersionPatch{Name: &newName}, testNow); codeOf(err) != "version.archived" {
		t.Fatalf("edit archived: %v", err)
	}
}

func TestUpdateClearsAndSetsDates(t *testing.T) {
	v, _ := NewVersion(testProject, "1.0", "", day(2026, 1, 1), day(2026, 2, 1), testNow)
	if err := v.Update(VersionPatch{StartDateSet: true, StartDate: nil}, testNow); err != nil {
		t.Fatal(err)
	}
	if v.StartDate != nil {
		t.Fatal("start date not cleared")
	}
	if err := v.Update(VersionPatch{StartDateSet: true, StartDate: day(2026, 3, 1)}, testNow); codeOf(err) != "version.invalid_dates" {
		t.Fatalf("release before start: %v", err)
	}
}

func TestAcceptanceRules(t *testing.T) {
	v, _ := NewVersion(testProject, "1.0", "", nil, nil, testNow)
	if !v.AcceptsFixIssues() || !v.AcceptsAffectsIssues() || !v.AcceptsMoves() {
		t.Fatal("unreleased should accept everything")
	}
	_ = v.Release(nil, testNow)
	if v.AcceptsFixIssues() || !v.AcceptsAffectsIssues() || v.AcceptsMoves() {
		t.Fatal("released: no fix, affects ok, no moves")
	}
	_ = v.Archive(testNow)
	if v.AcceptsAffectsIssues() {
		t.Fatal("archived should refuse affects")
	}
}

func TestProgressMath(t *testing.T) {
	p := NewProgress([]CategoryCount{
		{Category: CategoryTodo, Issues: 2, Points: 5},
		{Category: CategoryInProgress, Issues: 1, Points: 3},
		{Category: CategoryDone, Issues: 4, Points: 8.5},
		{Category: "unknown", Issues: 9, Points: 100},
	})
	if p.IssuesTotal != 16 || p.IssuesDone != 4 {
		t.Fatalf("totals %d done %d", p.IssuesTotal, p.IssuesDone)
	}
	if p.PointsTotal != 116.5 || p.PointsDone != 8.5 {
		t.Fatalf("points %v done %v", p.PointsTotal, p.PointsDone)
	}
	if p.ByCategory[CategoryTodo] != 2 || p.ByCategory[CategoryInProgress] != 1 || p.ByCategory[CategoryDone] != 4 {
		t.Fatalf("by category %v", p.ByCategory)
	}
	empty := NewProgress(nil)
	if len(empty.ByCategory) != 3 || empty.IssuesTotal != 0 {
		t.Fatalf("empty progress %+v", empty)
	}
}
