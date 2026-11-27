package domain

import (
	"html"
	"sort"
	"strings"
	"unicode"
)

// Result kinds added by the global search. KindProject and KindIssue are the original ones.
const (
	KindPerson Kind = "person"
	KindTeam   Kind = "team"
	KindFilter Kind = "filter"
	KindNote   Kind = "note"
)

// Highlight markers produced by the database (ts_headline). They are control characters that cannot appear
// in user text after cleaning, so they are safe to replace with markup once everything else is escaped.
const (
	MarkStart = "\x02"
	MarkEnd   = "\x03"
)

// AllKinds lists every kind in display order.
var AllKinds = []Kind{KindProject, KindIssue, KindPerson, KindTeam, KindFilter, KindNote}

// DefaultKinds are the kinds returned when the request names none: the ones the endpoint always returned.
var DefaultKinds = []Kind{KindProject, KindIssue}

// ParseKinds reads a comma-separated list of kinds. "all" selects every kind; unknown names are ignored; an
// empty list selects the default kinds. The result keeps display order and has no duplicates.
func ParseKinds(raw string) []Kind {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return DefaultKinds
	}
	want := map[Kind]bool{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.ToLower(strings.TrimSpace(part))
		switch part {
		case "all":
			return AllKinds
		case "person", "people", "user", "users":
			want[KindPerson] = true
		case "team", "teams":
			want[KindTeam] = true
		case "project", "projects":
			want[KindProject] = true
		case "issue", "issues":
			want[KindIssue] = true
		case "filter", "filters":
			want[KindFilter] = true
		case "note", "notes":
			want[KindNote] = true
		}
	}
	var out []Kind
	for _, k := range AllKinds {
		if want[k] {
			out = append(out, k)
		}
	}
	if len(out) == 0 {
		return DefaultKinds
	}
	return out
}

// GlobalHit is a global search result. Highlight is the title as HTML in which every character is escaped and
// the matched terms are wrapped in <mark>; Snippet is an escaped excerpt with the same markup.
type GlobalHit struct {
	Hit
	Key       string
	Highlight string
	Snippet   string
}

// SearchTerms splits a query into the terms to highlight: words of at least two characters, without
// websearch operators.
func SearchTerms(query string) []string {
	var terms []string
	seen := map[string]bool{}
	for _, f := range strings.FieldsFunc(query, func(r rune) bool { return unicode.IsSpace(r) || r == '"' }) {
		f = strings.TrimLeft(f, "-+")
		if len([]rune(f)) < 2 || strings.EqualFold(f, "or") {
			continue
		}
		if !seen[strings.ToLower(f)] {
			seen[strings.ToLower(f)] = true
			terms = append(terms, f)
		}
	}
	return terms
}

// HighlightHTML escapes text and wraps the case-insensitive occurrences of any term in <mark>.
func HighlightHTML(text string, terms []string) string {
	runes := []rune(text)
	type span struct{ from, to int }
	var spans []span
	for _, term := range terms {
		t := []rune(term)
		if len(t) == 0 {
			continue
		}
		for i := 0; i+len(t) <= len(runes); i++ {
			match := true
			for j := range t {
				if unicode.ToLower(runes[i+j]) != unicode.ToLower(t[j]) {
					match = false
					break
				}
			}
			if match {
				spans = append(spans, span{i, i + len(t)})
			}
		}
	}
	sort.Slice(spans, func(a, b int) bool { return spans[a].from < spans[b].from })
	var merged []span
	for _, s := range spans {
		if n := len(merged); n > 0 && s.from <= merged[n-1].to {
			if s.to > merged[n-1].to {
				merged[n-1].to = s.to
			}
			continue
		}
		merged = append(merged, s)
	}
	var sb strings.Builder
	pos := 0
	for _, s := range merged {
		sb.WriteString(html.EscapeString(string(runes[pos:s.from])))
		sb.WriteString("<mark>" + html.EscapeString(string(runes[s.from:s.to])) + "</mark>")
		pos = s.to
	}
	sb.WriteString(html.EscapeString(string(runes[pos:])))
	return sb.String()
}

// SnippetHTML converts a database headline, in which matches are wrapped in MarkStart and MarkEnd, to safe
// HTML: stray control characters are removed, everything is escaped and the markers become <mark>.
func SnippetHTML(headline string) string {
	cleaned := strings.Map(func(r rune) rune {
		if r < 0x20 && r != '\x02' && r != '\x03' && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, headline)
	escaped := html.EscapeString(cleaned)
	escaped = strings.ReplaceAll(escaped, MarkStart, "<mark>")
	escaped = strings.ReplaceAll(escaped, MarkEnd, "</mark>")
	return strings.Join(strings.Fields(escaped), " ")
}
