package domain

import "testing"

func TestParseKey(t *testing.T) {
	tests := []struct {
		ref     string
		project string
		number  int
		ok      bool
	}{
		{"CORE-12", "CORE", 12, true},
		{"A1-1", "A1", 1, true},
		{"ABCDEFGHIJ-999", "ABCDEFGHIJ", 999, true},
		{"core-12", "", 0, false},
		{"C-12", "", 0, false},
		{"ABCDEFGHIJK-1", "", 0, false},
		{"CORE-", "", 0, false},
		{"CORE-0", "", 0, false},
		{"CORE-99999999999", "", 0, false},
		{"CORE-1-2", "", 0, false},
		{"", "", 0, false},
	}
	for _, tt := range tests {
		project, number, ok := ParseKey(tt.ref)
		if project != tt.project || number != tt.number || ok != tt.ok {
			t.Errorf("ParseKey(%q) = %q, %d, %v; want %q, %d, %v", tt.ref, project, number, ok, tt.project, tt.number, tt.ok)
		}
	}
}

func TestIssueKey(t *testing.T) {
	if got := (&Issue{ProjectKey: "CORE", KeyNumber: 12}).Key(); got != "CORE-12" {
		t.Errorf("Key() = %q", got)
	}
	if got := (&Issue{KeyNumber: 12}).Key(); got != "" {
		t.Errorf("Key() without project key = %q", got)
	}
}
