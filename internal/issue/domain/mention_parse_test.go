package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestExtractMentions(t *testing.T) {
	ana := "00000000-0000-4000-8000-0000000000a1"
	luis := "00000000-0000-4000-8000-0000000000a2"
	anaID := uuid.MustParse(ana)
	luisID := uuid.MustParse(luis)

	tests := []struct {
		name string
		body string
		want []uuid.UUID
	}{
		{"plain text has none", "hola @ana sin token", nil},
		{"valid token", "revisa @[Ana Torres](user:" + ana + ") por favor", []uuid.UUID{anaID}},
		{"repeated token is kept once", "@[Ana](user:" + ana + ") y otra vez @[Ana](user:" + ana + ")", []uuid.UUID{anaID}},
		{"order of first appearance", "@[Luis](user:" + luis + ") y @[Ana](user:" + ana + ")", []uuid.UUID{luisID, anaID}},
		{"malformed uuid is not a mention", "@[Ana](user:1234) y @[Ana](user:" + ana + "0)", nil},
		{"nil uuid is not a mention", "@[Nadie](user:00000000-0000-0000-0000-000000000000)", nil},
		{"inline code is ignored", "usa `@[Ana](user:" + ana + ")` como ejemplo", nil},
		{"double backtick span is ignored", "``@[Ana](user:" + ana + ")``", nil},
		{"fenced block is ignored", "antes\n```\n@[Ana](user:" + ana + ")\n```\n@[Luis](user:" + luis + ")", []uuid.UUID{luisID}},
		{"tilde fence is ignored", "~~~\n@[Ana](user:" + ana + ")\n~~~", nil},
		{"unmatched backtick keeps the rest", "un ` suelto @[Ana](user:" + ana + ")", []uuid.UUID{anaID}},
		{"empty name is not a mention", "@[](user:" + ana + ")", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractMentions(tt.body)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d mentions %+v, want %d", len(got), got, len(tt.want))
			}
			for i, token := range got {
				if token.UserID != tt.want[i] {
					t.Fatalf("mention %d = %s, want %s", i, token.UserID, tt.want[i])
				}
			}
		})
	}
}

func FuzzExtractMentions(f *testing.F) {
	f.Add("@[Ana](user:00000000-0000-4000-8000-0000000000a1)")
	f.Add("```\n@[x](user:")
	f.Add("`")
	f.Fuzz(func(t *testing.T, body string) {
		for _, token := range ExtractMentions(body) {
			if token.UserID == uuid.Nil {
				t.Fatal("nil user id extracted")
			}
		}
	})
}
