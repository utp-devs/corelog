package domain

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestBuildReleaseNotesGroupsByType(t *testing.T) {
	v, _ := NewVersion(testProject, "1.2.0", "", nil, nil, testNow)
	_ = v.Release(day(2026, 11, 27), testNow)
	issues := []NoteIssue{
		{ID: uuid.New(), Key: "CORE-3", Summary: "Fix login", TypeName: "Error", Done: true},
		{ID: uuid.New(), Key: "CORE-1", Summary: "Add export", TypeName: "Historia"},
		{ID: uuid.New(), Key: "CORE-9", Summary: "Untyped *item*", TypeName: ""},
		{ID: uuid.New(), Key: "CORE-4", Summary: "Crash on `save`", TypeName: "Error"},
	}
	notes := BuildReleaseNotes(v, issues)
	titles := []string{}
	for _, g := range notes.Groups {
		titles = append(titles, g.Title)
	}
	if strings.Join(titles, "|") != "Error|Historia|Sin tipo" {
		t.Fatalf("group order = %v", titles)
	}
	if len(notes.Groups[0].Issues) != 2 || notes.Groups[0].Issues[0].Key != "CORE-3" {
		t.Fatalf("issues keep input order inside a group: %+v", notes.Groups[0].Issues)
	}
	for _, want := range []string{
		"# Notas de la versión 1.2.0\n", "Fecha de publicación: 2026-11-27", "## Error\n\n- CORE-3 Fix login\n",
		"- CORE-9 Untyped \\*item\\*", "Crash on \\`save\\`",
	} {
		if !strings.Contains(notes.Markdown, want) {
			t.Errorf("markdown lacks %q:\n%s", want, notes.Markdown)
		}
	}
}

func TestBuildReleaseNotesEmpty(t *testing.T) {
	v, _ := NewVersion(testProject, "0.1", "", nil, nil, testNow)
	notes := BuildReleaseNotes(v, nil)
	if notes.Groups == nil || len(notes.Groups) != 0 {
		t.Fatalf("groups = %#v", notes.Groups)
	}
	if !strings.Contains(notes.Markdown, "No hay incidencias asociadas a esta versión.") {
		t.Fatalf("markdown = %q", notes.Markdown)
	}
}
