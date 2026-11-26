package authztest

import (
	"strings"
	"testing"
)

func TestValidateMatrix(t *testing.T) {
	actors := []Actor{{Name: "lead"}, {Name: "viewer"}}
	permissions := []string{"edit", "view"}

	cases := []struct {
		name    string
		allowed map[string][]string
		want    string
	}{
		{"valid table", map[string][]string{"edit": {"lead"}, "view": {}}, ""},
		{"uncovered permission", map[string][]string{"edit": {"lead"}}, `permission "view" is not covered`},
		{"unknown permission", map[string][]string{"edit": {}, "view": {}, "delete": {}}, `unknown permission "delete"`},
		{"unknown actor", map[string][]string{"edit": {"ghost"}, "view": {}}, `unknown actor "ghost"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			problems := validateMatrix(actors, permissions, tc.allowed)
			joined := strings.Join(problems, "; ")
			if tc.want == "" {
				if len(problems) != 0 {
					t.Fatalf("expected no problems, got %q", joined)
				}
				return
			}
			if !strings.Contains(joined, tc.want) {
				t.Fatalf("expected a problem containing %q, got %q", tc.want, joined)
			}
		})
	}
}

func TestValidateMatrixRejectsDuplicateActors(t *testing.T) {
	problems := validateMatrix([]Actor{{Name: "lead"}, {Name: "lead"}}, nil, map[string][]string{})
	if len(problems) != 1 || !strings.Contains(problems[0], "duplicate actor") {
		t.Fatalf("expected one duplicate actor problem, got %v", problems)
	}
}

func TestStandardActorsAreUnique(t *testing.T) {
	if problems := validateMatrix(StandardActors, nil, map[string][]string{}); len(problems) != 0 {
		t.Fatalf("StandardActors has problems: %v", problems)
	}
}
