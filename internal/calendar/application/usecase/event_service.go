package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/calendar/application/port"
	"github.com/jhonsferg/corelog/internal/calendar/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
)

// Feed window: the past 30 days and the next 90.
const (
	feedPast   = 30 * 24 * time.Hour
	feedFuture = 90 * 24 * time.Hour
)

// CodeForbidden is the stable code of a denied calendar action.
const CodeForbidden = "calendar.forbidden"

var errForbidden = apperror.WithCode(apperror.KindForbidden, CodeForbidden, "no tienes permiso para esta acción en el calendario")

// EventService implements port.EventService.
type EventService struct {
	repo       port.EventRepository
	authorizer port.Authorizer
	clock      clock.Clock
	newToken   func() (string, error)
}

var _ port.EventService = (*EventService)(nil)

// NewEventService builds the service.
func NewEventService(repo port.EventRepository, authorizer port.Authorizer, clk clock.Clock) *EventService {
	return &EventService{repo: repo, authorizer: authorizer, clock: clk, newToken: randomToken}
}

// WithTokenGenerator replaces the feed token generator (tests).
func (s *EventService) WithTokenGenerator(f func() (string, error)) *EventService {
	s.newToken = f
	return s
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func isOrgAdmin(role string) bool { return role == "owner" || role == "admin" }

// Create stores a new event owned by the viewer.
func (s *EventService) Create(ctx context.Context, viewer port.Viewer, in port.EventInput) (*port.EventView, error) {
	e := &domain.Event{
		OwnerID: viewer.UserID, ProjectID: in.ProjectID, Title: in.Title, Description: in.Description, Location: in.Location,
		StartsAt: in.StartsAt, EndsAt: in.EndsAt, AllDay: in.AllDay, Timezone: in.Timezone, RecurrenceRule: in.RecurrenceRule,
		Visibility: in.Visibility, Color: in.Color,
	}
	if err := e.Normalize(); err != nil {
		return nil, err
	}
	orgID, err := s.resolveOrganization(ctx, viewer, in.OrganizationID, e)
	if err != nil {
		return nil, err
	}
	e.OrganizationID = orgID
	if e.Visibility == domain.VisibilityOrganization {
		if err := s.requireOrgAdmin(ctx, viewer, orgID); err != nil {
			return nil, err
		}
	}
	if e.Attendees, err = s.attendees(ctx, orgID, in.Attendees, nil); err != nil {
		return nil, err
	}
	now := s.clock.Now()
	e.CreatedAt, e.UpdatedAt = now, now
	if err := s.repo.Create(ctx, e); err != nil {
		return nil, err
	}
	return s.view(ctx, viewer, e, nil), nil
}

// resolveOrganization checks the project (visible, and editable for project events) and returns the organization.
func (s *EventService) resolveOrganization(ctx context.Context, viewer port.Viewer, requested uuid.UUID, e *domain.Event) (uuid.UUID, error) {
	if e.ProjectID == nil {
		return s.repo.OrganizationFor(ctx, viewer.UserID, requested)
	}
	action := port.ActionViewProject
	if e.Visibility == domain.VisibilityProject {
		action = port.ActionEditIssue
	}
	if err := s.authorizer.Authorize(ctx, action, viewer.UserID, viewer.GlobalRole, *e.ProjectID); err != nil {
		return uuid.Nil, err
	}
	return s.repo.ProjectOrganization(ctx, *e.ProjectID)
}

func (s *EventService) requireOrgAdmin(ctx context.Context, viewer port.Viewer, orgID uuid.UUID) error {
	if viewer.PlatformAdmin() {
		return nil
	}
	role, err := s.repo.OrgRole(ctx, viewer.UserID, orgID)
	if err != nil {
		return err
	}
	if !isOrgAdmin(role) {
		return errForbidden
	}
	return nil
}

// attendees validates the guest list: unique, members of the organization. previous keeps the responses of the
// guests that stay.
func (s *EventService) attendees(ctx context.Context, orgID uuid.UUID, ids []uuid.UUID, previous []domain.Attendee) ([]domain.Attendee, error) {
	seen := map[uuid.UUID]bool{}
	unique := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	if len(unique) > domain.MaxAttendees {
		return nil, apperror.WithCode(apperror.KindValidation, domain.CodeInvalidEvent, "evento no válido: un evento admite como máximo 100 invitados").
			WithDetails(apperror.FieldError{Field: "attendees", Message: "demasiados invitados"})
	}
	if len(unique) == 0 {
		return nil, nil
	}
	members, err := s.repo.MembersAmong(ctx, orgID, unique)
	if err != nil {
		return nil, err
	}
	if len(members) != len(unique) {
		return nil, apperror.WithCode(apperror.KindValidation, domain.CodeInvalidEvent, "evento no válido: hay invitados que no pertenecen a la organización").
			WithDetails(apperror.FieldError{Field: "attendees", Message: "todos los invitados deben ser miembros de la organización"})
	}
	kept := map[uuid.UUID]domain.Response{}
	for _, a := range previous {
		kept[a.UserID] = a.Response
	}
	out := make([]domain.Attendee, 0, len(unique))
	for _, id := range unique {
		resp := domain.ResponseNeedsAction
		if r, ok := kept[id]; ok {
			resp = r
		}
		out = append(out, domain.Attendee{UserID: id, Response: resp})
	}
	return out, nil
}

// canEdit decides whether the viewer can change the event. cache avoids repeating the project and organization
// checks while building a list.
func (s *EventService) canEdit(ctx context.Context, viewer port.Viewer, e *domain.Event, cache map[string]bool) bool {
	if e.OwnerID == viewer.UserID {
		return true
	}
	var key string
	var check func() bool
	switch {
	case e.Visibility == domain.VisibilityProject && e.ProjectID != nil:
		key = "p:" + e.ProjectID.String()
		check = func() bool {
			return s.authorizer.Authorize(ctx, port.ActionEditIssue, viewer.UserID, viewer.GlobalRole, *e.ProjectID) == nil
		}
	case e.Visibility == domain.VisibilityOrganization:
		key = "o:" + e.OrganizationID.String()
		check = func() bool { return s.requireOrgAdmin(ctx, viewer, e.OrganizationID) == nil }
	default:
		return false
	}
	if cache != nil {
		if v, ok := cache[key]; ok {
			return v
		}
	}
	v := check()
	if cache != nil {
		cache[key] = v
	}
	return v
}

func myResponse(e *domain.Event, userID uuid.UUID) domain.Response {
	for _, a := range e.Attendees {
		if a.UserID == userID {
			return a.Response
		}
	}
	return ""
}

func (s *EventService) view(ctx context.Context, viewer port.Viewer, e *domain.Event, cache map[string]bool) *port.EventView {
	return &port.EventView{Event: *e, CanEdit: s.canEdit(ctx, viewer, e, cache), MyResponse: myResponse(e, viewer.UserID)}
}

// Get returns one visible event.
func (s *EventService) Get(ctx context.Context, viewer port.Viewer, id uuid.UUID) (*port.EventView, error) {
	e, err := s.repo.FindVisible(ctx, viewer, id)
	if err != nil {
		return nil, err
	}
	return s.view(ctx, viewer, e, nil), nil
}

func (s *EventService) loadEditable(ctx context.Context, viewer port.Viewer, id uuid.UUID) (*domain.Event, error) {
	e, err := s.repo.FindVisible(ctx, viewer, id)
	if err != nil {
		return nil, err
	}
	if !s.canEdit(ctx, viewer, e, nil) {
		return nil, errForbidden
	}
	return e, nil
}

// Update changes the event, or one occurrence of it when occurrence is set.
func (s *EventService) Update(ctx context.Context, viewer port.Viewer, id uuid.UUID, occurrence *time.Time, patch port.EventPatch) (*port.EventView, error) {
	e, err := s.loadEditable(ctx, viewer, id)
	if err != nil {
		return nil, err
	}
	if occurrence != nil {
		if err := s.patchOccurrence(e, *occurrence, patch); err != nil {
			return nil, err
		}
		e.UpdatedAt = s.clock.Now()
		if err := s.repo.Update(ctx, e, false); err != nil {
			return nil, err
		}
		return s.view(ctx, viewer, e, nil), nil
	}
	replaceAttendees, err := s.applyPatch(ctx, viewer, e, patch)
	if err != nil {
		return nil, err
	}
	e.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, e, replaceAttendees); err != nil {
		return nil, err
	}
	return s.view(ctx, viewer, e, nil), nil
}

func (s *EventService) applyPatch(ctx context.Context, viewer port.Viewer, e *domain.Event, p port.EventPatch) (bool, error) {
	before := struct {
		start, end time.Time
		allDay     bool
		tz, rule   string
	}{e.StartsAt, e.EndsAt, e.AllDay, e.Timezone, ptrString(e.RecurrenceRule)}
	if p.Title != nil {
		e.Title = *p.Title
	}
	if p.Description != nil {
		e.Description = *p.Description
	}
	if p.Location != nil {
		e.Location = *p.Location
	}
	if p.StartsAt != nil {
		e.StartsAt = *p.StartsAt
	}
	if p.EndsAt != nil {
		e.EndsAt = *p.EndsAt
	}
	if p.AllDay != nil {
		e.AllDay = *p.AllDay
	}
	if p.Timezone != nil {
		e.Timezone = *p.Timezone
	}
	if p.RecurrenceRule.Set {
		e.RecurrenceRule = p.RecurrenceRule.Value
	}
	if p.Visibility != nil {
		e.Visibility = *p.Visibility
	}
	if p.Color != nil {
		e.Color = *p.Color
	}
	if p.ProjectID.Set {
		e.ProjectID = p.ProjectID.Value
	}
	if err := e.Normalize(); err != nil {
		return false, err
	}
	// Moving the event to another project, or making it visible to a project or the organization, needs the
	// rights of the new audience.
	if p.ProjectID.Set || p.Visibility != nil {
		if e.ProjectID != nil {
			action := port.ActionViewProject
			if e.Visibility == domain.VisibilityProject {
				action = port.ActionEditIssue
			}
			if err := s.authorizer.Authorize(ctx, action, viewer.UserID, viewer.GlobalRole, *e.ProjectID); err != nil {
				return false, err
			}
			orgID, err := s.repo.ProjectOrganization(ctx, *e.ProjectID)
			if err != nil {
				return false, err
			}
			if orgID != e.OrganizationID {
				return false, apperror.WithCode(apperror.KindValidation, domain.CodeInvalidEvent, "evento no válido: el proyecto pertenece a otra organización").
					WithDetails(apperror.FieldError{Field: "project_id", Message: "el proyecto pertenece a otra organización"})
			}
		}
		if e.Visibility == domain.VisibilityOrganization {
			if err := s.requireOrgAdmin(ctx, viewer, e.OrganizationID); err != nil {
				return false, err
			}
		}
	}
	if !before.start.Equal(e.StartsAt) || !before.end.Equal(e.EndsAt) || before.allDay != e.AllDay || before.tz != e.Timezone || before.rule != ptrString(e.RecurrenceRule) {
		// The exceptions point at instants of the old series.
		e.Exceptions = nil
	}
	if p.Attendees == nil {
		return false, nil
	}
	list, err := s.attendees(ctx, e.OrganizationID, *p.Attendees, e.Attendees)
	if err != nil {
		return false, err
	}
	e.Attendees = list
	return true, nil
}

func ptrString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// patchOccurrence overrides one occurrence of a recurring event.
func (s *EventService) patchOccurrence(e *domain.Event, occurrence time.Time, p port.EventPatch) error {
	if !e.Recurring() {
		return domain.ErrInvalidOccurrence
	}
	occurrence = occurrence.UTC()
	if !e.IsOccurrence(occurrence) {
		return domain.ErrInvalidOccurrence
	}
	if p.AllDay != nil || p.Timezone != nil || p.RecurrenceRule.Set || p.Visibility != nil || p.ProjectID.Set || p.Attendees != nil {
		return apperror.WithCode(apperror.KindValidation, domain.CodeInvalidOccurrence, "una ocurrencia solo admite cambiar título, descripción, lugar, color y horario")
	}
	override := &domain.Override{}
	if i, ok := e.FindException(occurrence); ok && e.Exceptions[i].Override != nil {
		copied := *e.Exceptions[i].Override
		override = &copied
	}
	if p.Title != nil {
		t := strings.TrimSpace(*p.Title)
		if t == "" || len([]rune(t)) > domain.MaxTitle {
			return invalidField("title", "el título es obligatorio y admite como máximo 200 caracteres")
		}
		override.Title = &t
	}
	if p.Description != nil {
		if len([]rune(*p.Description)) > domain.MaxDescription {
			return invalidField("description", "la descripción admite como máximo 8000 caracteres")
		}
		override.Description = p.Description
	}
	if p.Location != nil {
		if len([]rune(*p.Location)) > domain.MaxLocation {
			return invalidField("location", "el lugar admite como máximo 300 caracteres")
		}
		override.Location = p.Location
	}
	if p.Color != nil {
		override.Color = p.Color
	}
	if p.StartsAt != nil {
		t := p.StartsAt.UTC()
		override.StartsAt = &t
	}
	if p.EndsAt != nil {
		t := p.EndsAt.UTC()
		override.EndsAt = &t
	}
	dur := e.EndsAt.Sub(e.StartsAt)
	start, end := occurrence, occurrence.Add(dur)
	if override.StartsAt != nil {
		start = *override.StartsAt
		end = start.Add(dur)
	}
	if override.EndsAt != nil {
		end = *override.EndsAt
	}
	if end.Before(start) {
		return invalidField("ends_at", "el fin no puede ser anterior al inicio")
	}
	if e.AllDay && (start.Hour()+start.Minute()+end.Hour()+end.Minute() != 0 || !end.After(start)) {
		return invalidField("starts_at", "los eventos de día completo usan fechas (medianoche UTC)")
	}
	return e.SetException(domain.Exception{OriginalStart: occurrence, Override: override})
}

func invalidField(field, message string) error {
	return apperror.WithCode(apperror.KindValidation, domain.CodeInvalidEvent, "evento no válido: "+message).
		WithDetails(apperror.FieldError{Field: field, Message: message})
}

// Delete removes an event, or cancels one occurrence of a recurring event.
func (s *EventService) Delete(ctx context.Context, viewer port.Viewer, id uuid.UUID, occurrence *time.Time) error {
	e, err := s.loadEditable(ctx, viewer, id)
	if err != nil {
		return err
	}
	now := s.clock.Now()
	if occurrence == nil {
		return s.repo.Delete(ctx, id, now)
	}
	if !e.Recurring() || !e.IsOccurrence(occurrence.UTC()) {
		return domain.ErrInvalidOccurrence
	}
	if err := e.SetException(domain.Exception{OriginalStart: occurrence.UTC(), Cancelled: true}); err != nil {
		return err
	}
	e.UpdatedAt = now
	return s.repo.Update(ctx, e, false)
}

// Respond stores the answer of a guest.
func (s *EventService) Respond(ctx context.Context, viewer port.Viewer, id uuid.UUID, response domain.Response) error {
	if !domain.ValidResponse(response) {
		return invalidField("response", "la respuesta debe ser needs_action, accepted, declined o tentative")
	}
	if _, err := s.repo.FindVisible(ctx, viewer, id); err != nil {
		return err
	}
	ok, err := s.repo.SetResponse(ctx, id, viewer.UserID, response)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrNotAttendee
	}
	return nil
}

var kindGroups = map[string][]string{
	"events":   {port.KindEvent},
	"issues":   {port.KindIssueDue, port.KindIssueStart},
	"sprints":  {port.KindSprintStart, port.KindSprintEnd},
	"versions": {port.KindVersionRelease},
}

var derivedKinds = []string{port.KindIssueDue, port.KindIssueStart, port.KindSprintStart, port.KindSprintEnd, port.KindVersionRelease}

// ParseKinds expands groups (events, issues, sprints, versions) and exact kinds into a set. Empty means all.
func ParseKinds(raw []string) (map[string]bool, error) {
	set := map[string]bool{}
	if len(raw) == 0 {
		set[port.KindEvent] = true
		for _, k := range derivedKinds {
			set[k] = true
		}
		return set, nil
	}
	for _, r := range raw {
		r = strings.TrimSpace(r)
		if group, ok := kindGroups[r]; ok {
			for _, k := range group {
				set[k] = true
			}
			continue
		}
		known := r == port.KindEvent
		for _, k := range derivedKinds {
			known = known || k == r
		}
		if !known {
			return nil, apperror.WithCode(apperror.KindValidation, "calendar.invalid_types", "tipo de calendario no válido: "+r)
		}
		set[r] = true
	}
	return set, nil
}

// Items merges event occurrences with the dates derived from issues, sprints and versions.
func (s *EventService) Items(ctx context.Context, viewer port.Viewer, q port.ItemsQuery) (port.ItemsResult, error) {
	if err := domain.ValidateRange(q.From, q.To); err != nil {
		return port.ItemsResult{}, err
	}
	kinds, err := ParseKinds(q.Kinds)
	if err != nil {
		return port.ItemsResult{}, err
	}
	for _, p := range q.Projects {
		if err := s.authorizer.Authorize(ctx, port.ActionViewProject, viewer.UserID, viewer.GlobalRole, p); err != nil {
			return port.ItemsResult{}, err
		}
	}
	var result port.ItemsResult
	if kinds[port.KindEvent] {
		events, err := s.repo.ListVisible(ctx, viewer, q.From, q.To, nilIfEmpty(q.Projects))
		if err != nil {
			return result, err
		}
		cache := map[string]bool{}
		for i := range events {
			e := &events[i]
			occ, truncated := e.Series().Expand(q.From, q.To)
			result.Truncated = result.Truncated || truncated
			canEdit := s.canEdit(ctx, viewer, e, cache)
			mine := myResponse(e, viewer.UserID)
			for _, o := range occ {
				item := port.Item{
					Kind: port.KindEvent, Title: e.Title, StartsAt: o.Start, EndsAt: o.End, AllDay: e.AllDay, SourceID: e.ID,
					ProjectID: e.ProjectID, Link: "/calendar?event=" + e.ID.String(), Color: e.Color, Location: e.Location,
					Timezone: e.Timezone, Visibility: string(e.Visibility), Recurring: e.Recurring(), CanEdit: canEdit, MyResponse: mine,
				}
				if e.Recurring() {
					original := o.OriginalStart
					item.OccurrenceStart = &original
				}
				if ov := o.Override; ov != nil {
					if ov.Title != nil {
						item.Title = *ov.Title
					}
					if ov.Location != nil {
						item.Location = *ov.Location
					}
					if ov.Color != nil {
						item.Color = *ov.Color
					}
				}
				result.Items = append(result.Items, item)
			}
		}
	}
	var wanted []string
	for _, k := range derivedKinds {
		if kinds[k] {
			wanted = append(wanted, k)
		}
	}
	if len(wanted) > 0 {
		derived, err := s.repo.Derived(ctx, port.DerivedQuery{Viewer: viewer, From: q.From, To: q.To, Projects: nilIfEmpty(q.Projects), Kinds: wanted})
		if err != nil {
			return result, err
		}
		for _, d := range derived {
			pid := d.ProjectID
			start := time.Date(d.Date.Year(), d.Date.Month(), d.Date.Day(), 0, 0, 0, 0, time.UTC)
			result.Items = append(result.Items, port.Item{
				Kind: d.Kind, Title: d.Title, StartsAt: start, EndsAt: start.AddDate(0, 0, 1), AllDay: true, SourceID: d.SourceID,
				ProjectID: &pid, ProjectKey: d.ProjectKey, Link: d.Link, Color: derivedColor(d.Kind),
			})
		}
		if len(derived) >= DerivedCap {
			result.Truncated = true
		}
	}
	sort.SliceStable(result.Items, func(i, j int) bool {
		a, b := result.Items[i], result.Items[j]
		if !a.StartsAt.Equal(b.StartsAt) {
			return a.StartsAt.Before(b.StartsAt)
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Title < b.Title
	})
	return result, nil
}

// DerivedCap is the most derived dates a query returns; the repository applies the same limit.
const DerivedCap = 2000

func nilIfEmpty(ids []uuid.UUID) []uuid.UUID {
	if len(ids) == 0 {
		return nil
	}
	return ids
}

func derivedColor(kind string) string {
	switch kind {
	case port.KindIssueDue:
		return "red"
	case port.KindIssueStart:
		return "amber"
	case port.KindSprintStart, port.KindSprintEnd:
		return "indigo"
	default:
		return "green"
	}
}

// FeedInfo tells when the feed token was created.
func (s *EventService) FeedInfo(ctx context.Context, userID uuid.UUID) (*time.Time, error) {
	return s.repo.FeedInfo(ctx, userID)
}

// CreateFeed creates or rotates the feed token. The token is only returned here.
func (s *EventService) CreateFeed(ctx context.Context, userID uuid.UUID) (port.FeedToken, error) {
	token, err := s.newToken()
	if err != nil {
		return port.FeedToken{}, apperror.Wrap(apperror.KindInternal, "failed to generate the feed token", err)
	}
	now := s.clock.Now()
	if err := s.repo.ReplaceFeedToken(ctx, userID, hashToken(token), now); err != nil {
		return port.FeedToken{}, err
	}
	return port.FeedToken{Token: token, CreatedAt: now}, nil
}

// RevokeFeed disables the feed of the user.
func (s *EventService) RevokeFeed(ctx context.Context, userID uuid.UUID) error {
	ok, err := s.repo.RevokeFeedToken(ctx, userID, s.clock.Now())
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrFeedNotFound
	}
	return nil
}

// RenderFeed returns the calendar of the owner of the token.
func (s *EventService) RenderFeed(ctx context.Context, token string) (string, error) {
	if len(token) < 20 || len(token) > 128 {
		return "", domain.ErrFeedNotFound
	}
	userID, err := s.repo.UserByFeedToken(ctx, hashToken(token))
	if err != nil {
		return "", err
	}
	now := s.clock.Now()
	from, to := now.Add(-feedPast), now.Add(feedFuture)
	events, err := s.repo.ListVisible(ctx, port.Viewer{UserID: userID, GlobalRole: "user"}, from, to, nil)
	if err != nil {
		return "", err
	}
	return domain.RenderICS("CoreLog", events, from, to), nil
}
