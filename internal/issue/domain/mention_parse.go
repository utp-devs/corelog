package domain

import (
	"regexp"
	"strings"

	"github.com/google/uuid"
)

// MentionToken is one mention written with the editor syntax @[Name](user:UUID).
type MentionToken struct {
	UserID uuid.UUID
	Name   string
}

var mentionPattern = regexp.MustCompile(`@\[([^\[\]\r\n]+)\]\(user:([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})\)`)

// ExtractMentions returns the distinct mention tokens of a markdown body in order of first appearance. Tokens
// inside code spans and fenced code blocks are ignored. Validity (the user exists and can read the comment) is
// decided by the caller.
func ExtractMentions(markdown string) []MentionToken {
	text := stripCodeForMentions(markdown)
	out := []MentionToken{}
	seen := map[uuid.UUID]bool{}
	for _, match := range mentionPattern.FindAllStringSubmatch(text, -1) {
		id, err := uuid.Parse(match[2])
		if err != nil || id == uuid.Nil || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, MentionToken{UserID: id, Name: strings.TrimSpace(match[1])})
	}
	return out
}

// stripCodeForMentions blanks fenced code blocks and inline code spans so that mention tokens inside them are not
// matched. Line structure is kept so that fences are recognized line by line.
func stripCodeForMentions(markdown string) string {
	lines := strings.Split(markdown, "\n")
	fence := ""
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if fence != "" {
			if strings.HasPrefix(trimmed, fence) {
				fence = ""
			}
			lines[i] = ""
			continue
		}
		if strings.HasPrefix(trimmed, "```") {
			fence = "```"
			lines[i] = ""
		} else if strings.HasPrefix(trimmed, "~~~") {
			fence = "~~~"
			lines[i] = ""
		}
	}
	return stripInlineCode(strings.Join(lines, "\n"))
}

// stripInlineCode replaces each code span (a backtick run closed by a run of the same length) with a space.
// Unmatched backtick runs are kept as text.
func stripInlineCode(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] != '`' {
			b.WriteByte(s[i])
			i++
			continue
		}
		n := backtickRun(s, i)
		end := findBacktickRun(s, i+n, n)
		if end < 0 {
			b.WriteString(s[i : i+n])
			i += n
			continue
		}
		b.WriteByte(' ')
		i = end + n
	}
	return b.String()
}

func backtickRun(s string, at int) int {
	n := 0
	for at+n < len(s) && s[at+n] == '`' {
		n++
	}
	return n
}

// findBacktickRun returns the index of the next run of exactly n backticks at or after from, or -1.
func findBacktickRun(s string, from, n int) int {
	for j := from; j < len(s); {
		if s[j] != '`' {
			j++
			continue
		}
		k := backtickRun(s, j)
		if k == n {
			return j
		}
		j += k
	}
	return -1
}
