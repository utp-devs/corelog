package usecase

import (
	"context"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
)

// Text shown for references that no longer exist.
const (
	deletedUserName    = "Usuario eliminado"
	deletedLabelName   = "Etiqueta eliminada"
	deletedCatalogName = "Elemento eliminado"
	unnamedCustomField = "Campo personalizado"
)

// WithActivityFeed fixes the reader of the activity feed. Without it the feed is always empty.
func (s *IssueService) WithActivityFeed(r port.ActivityFeedReader) *IssueService {
	s.feed = r
	return s
}

// WithIssueLabels fixes the repository of issue labels. Without it a patch that sets labels fails.
func (s *IssueService) WithIssueLabels(r port.IssueLabelRepository) *IssueService {
	s.labels = r
	return s
}

// ListActivityFeed returns one page of the comments and history of an issue, newest first. A caller who cannot
// see the issue gets not found.
func (s *IssueService) ListActivityFeed(ctx context.Context, issueID, actorID uuid.UUID, globalRole string, query port.ActivityQuery) (*port.ActivityFeedPage, error) {
	filter, err := domain.ParseActivityFilter(query.Filter)
	if err != nil {
		return nil, err
	}
	limit, err := domain.NormalizeActivityLimit(query.Limit, query.LimitGiven)
	if err != nil {
		return nil, err
	}
	after, err := domain.DecodeActivityCursor(query.Cursor)
	if err != nil {
		return nil, err
	}
	issue, err := s.repo.FindByID(ctx, issueID)
	if err != nil {
		return nil, err
	}
	if err := s.requireVisible(ctx, issue.ProjectID, actorID, globalRole); err != nil {
		return nil, err
	}
	page := &port.ActivityFeedPage{Entries: []port.ActivityEntry{}}
	if s.feed == nil {
		return page, nil
	}
	// One extra row tells whether an older page exists.
	refs, err := s.feed.FeedRefs(ctx, issueID, filter, after, limit+1)
	if err != nil {
		return nil, err
	}
	hasMore := len(refs) > limit
	if hasMore {
		refs = refs[:limit]
	}
	content, err := s.feed.FeedContent(ctx, issueID, refs)
	if err != nil {
		return nil, err
	}
	book, err := s.loadNameBook(ctx, refs, content)
	if err != nil {
		return nil, err
	}
	for _, ref := range refs {
		if entry, ok := book.entry(ref, content); ok {
			page.Entries = append(page.Entries, entry)
		}
	}
	if hasMore && len(refs) > 0 {
		last := refs[len(refs)-1]
		page.NextCursor = domain.EncodeActivityCursor(domain.ActivityCursor{At: last.At, Kind: last.Kind, ID: last.ID})
	}
	return page, nil
}

// nameBook holds the current names of every reference the page shows, loaded in one batch per kind.
type nameBook struct {
	s         *IssueService
	users     map[uuid.UUID]string
	priority  map[uuid.UUID]string
	issueType map[uuid.UUID]string
	labels    map[uuid.UUID]string
	custom    map[uuid.UUID]string
}

// loadNameBook collects the ids of the page and resolves them: one user lookup plus one query per catalog kind.
func (s *IssueService) loadNameBook(ctx context.Context, refs []port.FeedRef, content port.FeedContent) (*nameBook, error) {
	ids := newIDCollector()
	for _, ref := range refs {
		if ref.Kind == domain.FeedKindComment {
			if comment, ok := content.Comments[ref.ID]; ok {
				ids.add(kindUser, comment.AuthorID)
			}
			continue
		}
		for _, activity := range content.Activities[ref.ID] {
			ids.add(kindUser, activity.ActorID)
			field := ""
			if activity.FieldName != nil {
				field = *activity.FieldName
			}
			for _, raw := range []*string{activity.OldValue, activity.NewValue} {
				collectReference(ids, field, raw)
			}
		}
	}
	book := &nameBook{s: s}
	var err error
	if book.users, err = s.namesOfUsers(ctx, ids.list(kindUser)); err != nil {
		return nil, err
	}
	if book.priority, err = s.referenceNames(ctx, port.ReferencePriority, ids.list(kindPriority)); err != nil {
		return nil, err
	}
	if book.issueType, err = s.referenceNames(ctx, port.ReferenceIssueType, ids.list(kindIssueType)); err != nil {
		return nil, err
	}
	if book.labels, err = s.referenceNames(ctx, port.ReferenceLabel, ids.list(kindLabel)); err != nil {
		return nil, err
	}
	if book.custom, err = s.referenceNames(ctx, port.ReferenceCustomField, ids.list(kindCustom)); err != nil {
		return nil, err
	}
	return book, nil
}

func (s *IssueService) namesOfUsers(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	if len(ids) == 0 || s.directory == nil {
		return map[uuid.UUID]string{}, nil
	}
	names, err := s.directory.NamesOf(ctx, ids)
	if err != nil {
		return nil, err
	}
	return names, nil
}

func (s *IssueService) referenceNames(ctx context.Context, kind string, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	if len(ids) == 0 || s.feed == nil {
		return map[uuid.UUID]string{}, nil
	}
	return s.feed.ReferenceNames(ctx, kind, ids)
}

// entry builds the view of one feed reference. It reports false when the rows of the reference are gone.
func (b *nameBook) entry(ref port.FeedRef, content port.FeedContent) (port.ActivityEntry, bool) {
	if ref.Kind == domain.FeedKindComment {
		comment, ok := content.Comments[ref.ID]
		if !ok {
			return port.ActivityEntry{}, false
		}
		return port.ActivityEntry{
			ID: ref.ID, Kind: domain.FeedKindComment, At: ref.At,
			ActorID: comment.AuthorID, ActorName: b.userName(comment.AuthorID),
			Comment: &port.ActivityComment{
				ID: comment.ID, Body: comment.Body, UpdatedAt: comment.UpdatedAt,
				Edited: comment.UpdatedAt.After(comment.CreatedAt),
			},
		}, true
	}
	rows := content.Activities[ref.ID]
	if len(rows) == 0 {
		return port.ActivityEntry{}, false
	}
	actor := rows[0].ActorID
	changes := make([]port.ActivityChange, 0, len(rows))
	for _, row := range rows {
		field := ""
		if row.FieldName != nil {
			field = *row.FieldName
		}
		changes = append(changes, port.ActivityChange{
			Action: row.Action,
			Field:  field,
			Label:  b.fieldLabel(field),
			From:   b.render(field, row.OldValue),
			To:     b.render(field, row.NewValue),
		})
	}
	return port.ActivityEntry{
		ID: ref.ID, Kind: domain.FeedKindHistory, At: ref.At,
		ActorID: actor, ActorName: b.userName(actor), Changes: changes,
	}, true
}

func (b *nameBook) userName(id uuid.UUID) string {
	if name, ok := b.users[id]; ok {
		return name
	}
	return deletedUserName
}

func (b *nameBook) fieldLabel(field string) string {
	if id, ok := parseReferenceID(field); ok {
		// Custom fields are stored under their id.
		if name, found := b.custom[id]; found {
			return name
		}
		return unnamedCustomField
	}
	return domain.FieldLabel(field)
}

// render turns a stored value into text for people: ids become current names, and a missing reference says so.
func (b *nameBook) render(field string, raw *string) *string {
	if raw == nil || *raw == "" {
		return nil
	}
	value := *raw
	var text string
	switch field {
	case "assignee", "assignee_id", "reporter":
		text = b.userText(value)
	case "priority":
		text = b.catalogText(b.priority, value)
	case "issue_type":
		text = b.catalogText(b.issueType, value)
	case "labels":
		text = b.labelsText(value)
	default:
		text = value
	}
	return &text
}

func (b *nameBook) userText(value string) string {
	id, ok := parseReferenceID(value)
	if !ok {
		return value
	}
	return b.userName(id)
}

func (b *nameBook) catalogText(names map[uuid.UUID]string, value string) string {
	id, ok := parseReferenceID(value)
	if !ok {
		return value
	}
	if name, found := names[id]; found {
		return name
	}
	return deletedCatalogName
}

func (b *nameBook) labelsText(value string) string {
	var parts []string
	for _, raw := range strings.Split(value, ",") {
		id, ok := parseReferenceID(strings.TrimSpace(raw))
		if !ok {
			continue
		}
		if name, found := b.labels[id]; found {
			parts = append(parts, name)
		} else {
			parts = append(parts, deletedLabelName)
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

// Kinds of references collected for the name lookups.
const (
	kindUser = iota
	kindPriority
	kindIssueType
	kindLabel
	kindCustom
)

// idCollector groups the ids of a page by kind and removes repeats.
type idCollector struct {
	seen map[int]map[uuid.UUID]bool
}

func newIDCollector() *idCollector {
	return &idCollector{seen: map[int]map[uuid.UUID]bool{}}
}

func (c *idCollector) add(kind int, id uuid.UUID) {
	if id == uuid.Nil {
		return
	}
	if c.seen[kind] == nil {
		c.seen[kind] = map[uuid.UUID]bool{}
	}
	c.seen[kind][id] = true
}

func (c *idCollector) list(kind int) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(c.seen[kind]))
	for id := range c.seen[kind] {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

// collectReference adds the ids a stored value refers to, according to the field it belongs to.
func collectReference(ids *idCollector, field string, raw *string) {
	if raw == nil {
		return
	}
	value := *raw
	switch field {
	case "assignee", "assignee_id", "reporter":
		if id, ok := parseReferenceID(value); ok {
			ids.add(kindUser, id)
		}
	case "priority":
		if id, ok := parseReferenceID(value); ok {
			ids.add(kindPriority, id)
		}
	case "issue_type":
		if id, ok := parseReferenceID(value); ok {
			ids.add(kindIssueType, id)
		}
	case "labels":
		for _, part := range strings.Split(value, ",") {
			if id, ok := parseReferenceID(strings.TrimSpace(part)); ok {
				ids.add(kindLabel, id)
			}
		}
	default:
		if id, ok := parseReferenceID(field); ok {
			ids.add(kindCustom, id)
		}
	}
}

func parseReferenceID(value string) (uuid.UUID, bool) {
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, false
	}
	return id, true
}
