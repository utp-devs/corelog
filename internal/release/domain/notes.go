package domain

import (
	"sort"
	"strings"

	"github.com/google/uuid"
)

// NoteIssue is one issue that fixes a version, as the notes need it.
type NoteIssue struct {
	ID       uuid.UUID
	Key      string
	Summary  string
	TypeName string
	Done     bool
}

// NoteItem is an issue listed in the notes.
type NoteItem struct {
	ID      uuid.UUID `json:"id"`
	Key     string    `json:"key"`
	Summary string    `json:"summary"`
	Done    bool      `json:"done"`
}

// NoteGroup is one section of the notes: the issues of one issue type.
type NoteGroup struct {
	Title  string     `json:"title"`
	Issues []NoteItem `json:"issues"`
}

// ReleaseNotes is the grouped list of issues that fix a version, with its markdown rendering.
type ReleaseNotes struct {
	VersionID   uuid.UUID
	VersionName string
	Status      Status
	ReleaseDate string
	Groups      []NoteGroup
	Markdown    string
}

// NoIssueTypeTitle is the heading of issues without an issue type.
const NoIssueTypeTitle = "Sin tipo"

// BuildReleaseNotes groups the issues by issue type (sorted by type name, the untyped group last) and renders
// the markdown. Issues keep the order of the input inside each group.
func BuildReleaseNotes(v *Version, issues []NoteIssue) ReleaseNotes {
	byTitle := map[string][]NoteItem{}
	for _, issue := range issues {
		title := strings.TrimSpace(issue.TypeName)
		if title == "" {
			title = NoIssueTypeTitle
		}
		byTitle[title] = append(byTitle[title], NoteItem{ID: issue.ID, Key: issue.Key, Summary: issue.Summary, Done: issue.Done})
	}
	titles := make([]string, 0, len(byTitle))
	for title := range byTitle {
		titles = append(titles, title)
	}
	sort.Slice(titles, func(i, j int) bool {
		if titles[i] == NoIssueTypeTitle {
			return false
		}
		if titles[j] == NoIssueTypeTitle {
			return true
		}
		return titles[i] < titles[j]
	})

	notes := ReleaseNotes{
		VersionID: v.ID, VersionName: v.Name, Status: v.Status, ReleaseDate: FormatDate(v.ReleaseDate),
		Groups: make([]NoteGroup, 0, len(titles)),
	}
	for _, title := range titles {
		notes.Groups = append(notes.Groups, NoteGroup{Title: title, Issues: byTitle[title]})
	}
	notes.Markdown = renderMarkdown(notes)
	return notes
}

func renderMarkdown(n ReleaseNotes) string {
	var b strings.Builder
	b.WriteString("# Notas de la versión ")
	b.WriteString(escapeMarkdown(n.VersionName))
	b.WriteString("\n\n")
	if n.ReleaseDate != "" {
		b.WriteString("Fecha de publicación: ")
		b.WriteString(n.ReleaseDate)
		b.WriteString("\n\n")
	}
	if len(n.Groups) == 0 {
		b.WriteString("No hay incidencias asociadas a esta versión.\n")
		return b.String()
	}
	for _, group := range n.Groups {
		b.WriteString("## ")
		b.WriteString(escapeMarkdown(group.Title))
		b.WriteString("\n\n")
		for _, item := range group.Issues {
			b.WriteString("- ")
			if item.Key != "" {
				b.WriteString(item.Key)
				b.WriteString(" ")
			}
			b.WriteString(escapeMarkdown(item.Summary))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}

var markdownSpecial = strings.NewReplacer(
	`\`, `\\`, "`", "\\`", "*", `\*`, "_", `\_`, "[", `\[`, "]", `\]`, "#", `\#`,
)

// escapeMarkdown keeps user text from creating markup in the rendered notes.
func escapeMarkdown(text string) string {
	return markdownSpecial.Replace(strings.ReplaceAll(text, "\n", " "))
}
