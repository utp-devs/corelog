package domain

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

var fixedNow = time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)

func TestNormalizeTags(t *testing.T) {
	long := strings.Repeat("a", MaxTagLength)
	tooLong := strings.Repeat("a", MaxTagLength+1)
	tests := []struct {
		name    string
		in      []string
		want    []string
		wantErr bool
	}{
		{name: "nil gives empty slice", in: nil, want: []string{}},
		{name: "trims and lowercases", in: []string{"  Urgente ", "BUG"}, want: []string{"urgente", "bug"}},
		{name: "dedupes keeping first", in: []string{"a", "A", " a ", "b"}, want: []string{"a", "b"}},
		{name: "exactly ten allowed", in: tagsN(MaxTags), want: tagsN(MaxTags)},
		{name: "eleven distinct rejected", in: tagsN(MaxTags + 1), wantErr: true},
		{name: "blank tag rejected", in: []string{"  "}, wantErr: true},
		{name: "30 characters allowed", in: []string{long}, want: []string{long}},
		{name: "31 characters rejected", in: []string{tooLong}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeTags(tt.in)
			if tt.wantErr {
				assertCode(t, err, "note.invalid", apperror.KindValidation)
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewNoteValidation(t *testing.T) {
	owner := uuid.New()
	tests := []struct {
		name    string
		owner   uuid.UUID
		fields  Fields
		wantErr bool
	}{
		{name: "empty note is valid", owner: owner, fields: Fields{}},
		{name: "title of 120 runes allowed", owner: owner, fields: Fields{Title: strings.Repeat("ñ", 120)}},
		{name: "title of 121 runes rejected", owner: owner, fields: Fields{Title: strings.Repeat("ñ", 121)}, wantErr: true},
		{name: "body of 20000 allowed", owner: owner, fields: Fields{Body: strings.Repeat("x", MaxBodyLength)}},
		{name: "body of 20001 rejected", owner: owner, fields: Fields{Body: strings.Repeat("x", MaxBodyLength+1)}, wantErr: true},
		{name: "color outside the palette rejected", owner: owner, fields: Fields{Color: "teal"}, wantErr: true},
		{name: "known color allowed", owner: owner, fields: Fields{Color: "gray"}},
		{name: "nil owner rejected", owner: uuid.Nil, fields: Fields{}, wantErr: true},
		{name: "nil issue id rejected", owner: owner, fields: Fields{IssueID: &uuid.Nil}, wantErr: true},
		{name: "nil project id rejected", owner: owner, fields: Fields{ProjectID: &uuid.Nil}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := NewNote(tt.owner, tt.fields, fixedNow)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				if apperror.KindOf(err) != apperror.KindValidation {
					t.Fatalf("kind = %s, want validation", apperror.KindOf(err))
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if n.Color == "" {
				t.Fatal("color must default to the palette default")
			}
		})
	}
}

func TestNewNoteDefaultsAndTrims(t *testing.T) {
	n, err := NewNote(uuid.New(), Fields{Title: "  Hola  ", Tags: []string{" Compras "}}, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	if n.Title != "Hola" || n.Color != DefaultColor || !reflect.DeepEqual(n.Tags, []string{"compras"}) {
		t.Fatalf("unexpected note %+v", n)
	}
	if !n.CreatedAt.Equal(fixedNow) || n.ID != uuid.Nil {
		t.Fatalf("id must stay nil and timestamps come from the clock: %+v", n)
	}
}

func TestApplyPatchIsAtomic(t *testing.T) {
	n, err := NewNote(uuid.New(), Fields{Title: "keep", Tags: []string{"a"}}, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	newTitle := "changed"
	badTags := []string{"ok", ""}
	err = n.ApplyPatch(Patch{Title: &newTitle, Tags: &badTags}, fixedNow.Add(time.Hour))
	if err == nil {
		t.Fatal("expected a validation error")
	}
	if n.Title != "keep" || !reflect.DeepEqual(n.Tags, []string{"a"}) {
		t.Fatalf("failed patch must not change the note: %+v", n)
	}
}

func TestApplyPatchLinks(t *testing.T) {
	issue := uuid.New()
	n, err := NewNote(uuid.New(), Fields{IssueID: &issue}, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	if n.IssueLinkChanged(Patch{}) {
		t.Fatal("absent link must not count as a change")
	}
	if !n.IssueLinkChanged(Patch{ClearIssue: true}) {
		t.Fatal("clearing an existing link is a change")
	}
	if n.IssueLinkChanged(Patch{IssueID: &issue}) {
		t.Fatal("setting the same issue again is not a change")
	}
	if err := n.ApplyPatch(Patch{ClearIssue: true}, fixedNow); err != nil {
		t.Fatal(err)
	}
	if n.IssueID != nil {
		t.Fatal("issue link must be cleared")
	}
}

func TestIsValidColorPalette(t *testing.T) {
	if len(Colors) != 8 {
		t.Fatalf("palette has %d keys, want 8", len(Colors))
	}
	for _, c := range Colors {
		if !IsValidColor(c) {
			t.Fatalf("%s must be valid", c)
		}
	}
	if IsValidColor("Yellow") {
		t.Fatal("keys are case sensitive")
	}
}

func tagsN(n int) []string {
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, "t"+strings.Repeat("x", i))
	}
	return out
}

func assertCode(t *testing.T, err error, code string, kind apperror.Kind) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %s", code)
	}
	if apperror.CodeOf(err) != code || apperror.KindOf(err) != kind {
		t.Fatalf("got code %q kind %s, want %q %s", apperror.CodeOf(err), apperror.KindOf(err), code, kind)
	}
}
