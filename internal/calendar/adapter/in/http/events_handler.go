package http

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/calendar/application/port"
	"github.com/jhonsferg/corelog/internal/calendar/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
)

const maxBodyBytes = 256 << 10

// WithEvents enables the event, merged calendar and feed endpoints. publicURL is the base the feed URL is built on.
func (h *Handler) WithEvents(events port.EventService, publicURL string) *Handler {
	h.events = events
	h.publicURL = strings.TrimRight(publicURL, "/")
	return h
}

func (h *Handler) registerEventRoutes(r chi.Router) {
	// The feed is the only route that authenticates by token instead of the session.
	r.Get("/export.ics", h.ExportICS)
	r.Group(func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))
		r.Get("/", h.Items)
		r.Post("/events", h.CreateEvent)
		r.Get("/events/{id}", h.GetEvent)
		r.Patch("/events/{id}", h.UpdateEvent)
		r.Delete("/events/{id}", h.DeleteEvent)
		r.Put("/events/{id}/attendees/me", h.Respond)
	})
}

// RegisterMeRoutes registers the feed management routes under /me/calendar/feed.
func (h *Handler) RegisterMeRoutes(r chi.Router) {
	if h.events == nil {
		return
	}
	r.Route("/me/calendar/feed", func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))
		r.Get("/", h.FeedInfo)
		r.Post("/", h.CreateFeed)
		r.Delete("/", h.RevokeFeed)
	})
}

func validation(code, message string) error {
	return apperror.WithCode(apperror.KindValidation, code, message)
}

func invalidBody() error {
	return validation("validation.failed", "el cuerpo de la petición no es válido")
}

func decodeStrict(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		httpresponse.Error(w, invalidBody())
		return false
	}
	if _, err := dec.Token(); err != io.EOF {
		httpresponse.Error(w, invalidBody())
		return false
	}
	return true
}

func viewerOf(w http.ResponseWriter, r *http.Request) (port.Viewer, bool) {
	id, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return port.Viewer{}, false
	}
	role, _ := appmw.RoleFromContext(r.Context())
	return port.Viewer{UserID: id, GlobalRole: role}, true
}

func eventID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, domain.ErrEventNotFound)
		return uuid.Nil, false
	}
	return id, true
}

// parseInstant accepts RFC 3339 or a plain date (midnight UTC).
func parseInstant(value string) (time.Time, bool, error) {
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t.UTC(), false, nil
	}
	if t, err := time.Parse(dateLayout, value); err == nil {
		return t, true, nil
	}
	return time.Time{}, false, validation("validation.failed", "la fecha no es válida (RFC 3339 o AAAA-MM-DD)")
}

func occurrenceParam(r *http.Request) (*time.Time, error) {
	raw := r.URL.Query().Get("occurrence")
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, domain.ErrInvalidOccurrence
	}
	t = t.UTC()
	return &t, nil
}

type attendeeResponse struct {
	UserID   string `json:"user_id"`
	Name     string `json:"name"`
	Response string `json:"response"`
}

type overrideBody struct {
	Title       *string    `json:"title,omitempty"`
	Description *string    `json:"description,omitempty"`
	Location    *string    `json:"location,omitempty"`
	Color       *string    `json:"color,omitempty"`
	StartsAt    *time.Time `json:"starts_at,omitempty"`
	EndsAt      *time.Time `json:"ends_at,omitempty"`
}

type exceptionResponse struct {
	OriginalStart time.Time     `json:"original_start"`
	Cancelled     bool          `json:"cancelled"`
	Override      *overrideBody `json:"override,omitempty"`
}

type fullEventResponse struct {
	ID                   string              `json:"id"`
	OwnerID              string              `json:"owner_id"`
	OrganizationID       string              `json:"organization_id"`
	ProjectID            *string             `json:"project_id"`
	Title                string              `json:"title"`
	Description          string              `json:"description"`
	Location             string              `json:"location"`
	StartsAt             time.Time           `json:"starts_at"`
	EndsAt               time.Time           `json:"ends_at"`
	AllDay               bool                `json:"all_day"`
	Timezone             string              `json:"timezone"`
	RecurrenceRule       *string             `json:"recurrence_rule"`
	RecurrenceExceptions []exceptionResponse `json:"recurrence_exceptions"`
	Visibility           string              `json:"visibility"`
	Color                string              `json:"color"`
	Attendees            []attendeeResponse  `json:"attendees"`
	MyResponse           *string             `json:"my_response"`
	CanEdit              bool                `json:"can_edit"`
	CreatedAt            time.Time           `json:"created_at"`
	UpdatedAt            time.Time           `json:"updated_at"`
}

func toEventResponse(v *port.EventView) fullEventResponse {
	e := v.Event
	out := fullEventResponse{
		ID: e.ID.String(), OwnerID: e.OwnerID.String(), OrganizationID: e.OrganizationID.String(), Title: e.Title,
		Description: e.Description, Location: e.Location, StartsAt: e.StartsAt, EndsAt: e.EndsAt, AllDay: e.AllDay,
		Timezone: e.Timezone, RecurrenceRule: e.RecurrenceRule, Visibility: string(e.Visibility), Color: e.Color,
		CanEdit: v.CanEdit, CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
		RecurrenceExceptions: make([]exceptionResponse, 0, len(e.Exceptions)), Attendees: make([]attendeeResponse, 0, len(e.Attendees)),
	}
	if e.ProjectID != nil {
		s := e.ProjectID.String()
		out.ProjectID = &s
	}
	if v.MyResponse != "" {
		s := string(v.MyResponse)
		out.MyResponse = &s
	}
	for _, x := range e.Exceptions {
		item := exceptionResponse{OriginalStart: x.OriginalStart, Cancelled: x.Cancelled}
		if o := x.Override; o != nil {
			item.Override = &overrideBody{Title: o.Title, Description: o.Description, Location: o.Location, Color: o.Color, StartsAt: o.StartsAt, EndsAt: o.EndsAt}
		}
		out.RecurrenceExceptions = append(out.RecurrenceExceptions, item)
	}
	for _, a := range e.Attendees {
		out.Attendees = append(out.Attendees, attendeeResponse{UserID: a.UserID.String(), Name: a.Name, Response: string(a.Response)})
	}
	return out
}

type createEventRequest struct {
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	Location       string   `json:"location"`
	StartsAt       string   `json:"starts_at"`
	EndsAt         string   `json:"ends_at"`
	AllDay         bool     `json:"all_day"`
	Timezone       string   `json:"timezone"`
	RecurrenceRule *string  `json:"recurrence_rule"`
	Visibility     string   `json:"visibility"`
	Color          string   `json:"color"`
	ProjectID      *string  `json:"project_id"`
	OrganizationID *string  `json:"organization_id"`
	Attendees      []string `json:"attendees"`
}

func parseUUIDs(values []string, field string) ([]uuid.UUID, error) {
	out := make([]uuid.UUID, 0, len(values))
	for _, v := range values {
		id, err := uuid.Parse(strings.TrimSpace(v))
		if err != nil {
			return nil, validation(domain.CodeInvalidEvent, "evento no válido: "+field+" contiene un id no válido")
		}
		out = append(out, id)
	}
	return out, nil
}

func parseOptUUID(value *string, field string) (*uuid.UUID, error) {
	if value == nil || *value == "" {
		return nil, nil
	}
	id, err := uuid.Parse(*value)
	if err != nil {
		return nil, validation(domain.CodeInvalidEvent, "evento no válido: "+field+" no es un id válido")
	}
	return &id, nil
}

// CreateEvent creates an event owned by the caller.
func (h *Handler) CreateEvent(w http.ResponseWriter, r *http.Request) {
	viewer, ok := viewerOf(w, r)
	if !ok {
		return
	}
	var req createEventRequest
	if !decodeStrict(w, r, &req) {
		return
	}
	in, err := req.input()
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	view, err := h.events.Create(r.Context(), viewer, in)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusCreated, toEventResponse(view))
}

func (req createEventRequest) input() (port.EventInput, error) {
	start, _, err := parseInstant(req.StartsAt)
	if err != nil {
		return port.EventInput{}, err
	}
	end, _, err := parseInstant(req.EndsAt)
	if err != nil {
		return port.EventInput{}, err
	}
	project, err := parseOptUUID(req.ProjectID, "project_id")
	if err != nil {
		return port.EventInput{}, err
	}
	org, err := parseOptUUID(req.OrganizationID, "organization_id")
	if err != nil {
		return port.EventInput{}, err
	}
	attendees, err := parseUUIDs(req.Attendees, "attendees")
	if err != nil {
		return port.EventInput{}, err
	}
	in := port.EventInput{
		ProjectID: project, Title: req.Title, Description: req.Description, Location: req.Location, StartsAt: start, EndsAt: end,
		AllDay: req.AllDay, Timezone: req.Timezone, RecurrenceRule: req.RecurrenceRule, Visibility: domain.Visibility(req.Visibility),
		Color: req.Color, Attendees: attendees,
	}
	if org != nil {
		in.OrganizationID = *org
	}
	return in, nil
}

// GetEvent returns one event with its attendees.
func (h *Handler) GetEvent(w http.ResponseWriter, r *http.Request) {
	viewer, ok := viewerOf(w, r)
	if !ok {
		return
	}
	id, ok := eventID(w, r)
	if !ok {
		return
	}
	view, err := h.events.Get(r.Context(), viewer, id)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toEventResponse(view))
}

type patchEventRequest struct {
	Title          *string         `json:"title"`
	Description    *string         `json:"description"`
	Location       *string         `json:"location"`
	StartsAt       *string         `json:"starts_at"`
	EndsAt         *string         `json:"ends_at"`
	AllDay         *bool           `json:"all_day"`
	Timezone       *string         `json:"timezone"`
	RecurrenceRule json.RawMessage `json:"recurrence_rule"`
	Visibility     *string         `json:"visibility"`
	Color          *string         `json:"color"`
	ProjectID      json.RawMessage `json:"project_id"`
	Attendees      *[]string       `json:"attendees"`
}

func (req patchEventRequest) patch() (port.EventPatch, error) {
	p := port.EventPatch{Title: req.Title, Description: req.Description, Location: req.Location, AllDay: req.AllDay, Timezone: req.Timezone, Color: req.Color}
	if req.StartsAt != nil {
		t, _, err := parseInstant(*req.StartsAt)
		if err != nil {
			return p, err
		}
		p.StartsAt = &t
	}
	if req.EndsAt != nil {
		t, _, err := parseInstant(*req.EndsAt)
		if err != nil {
			return p, err
		}
		p.EndsAt = &t
	}
	if req.Visibility != nil {
		v := domain.Visibility(*req.Visibility)
		p.Visibility = &v
	}
	if req.RecurrenceRule != nil {
		p.RecurrenceRule.Set = true
		if string(req.RecurrenceRule) != "null" {
			var s string
			if err := json.Unmarshal(req.RecurrenceRule, &s); err != nil {
				return p, invalidBody()
			}
			p.RecurrenceRule.Value = &s
		}
	}
	if req.ProjectID != nil {
		p.ProjectID.Set = true
		if string(req.ProjectID) != "null" {
			var s string
			if err := json.Unmarshal(req.ProjectID, &s); err != nil {
				return p, invalidBody()
			}
			id, err := parseOptUUID(&s, "project_id")
			if err != nil {
				return p, err
			}
			p.ProjectID.Value = id
		}
	}
	if req.Attendees != nil {
		ids, err := parseUUIDs(*req.Attendees, "attendees")
		if err != nil {
			return p, err
		}
		p.Attendees = &ids
	}
	return p, nil
}

// UpdateEvent changes an event or, with ?occurrence=, one occurrence of it.
func (h *Handler) UpdateEvent(w http.ResponseWriter, r *http.Request) {
	viewer, ok := viewerOf(w, r)
	if !ok {
		return
	}
	id, ok := eventID(w, r)
	if !ok {
		return
	}
	occurrence, err := occurrenceParam(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	var req patchEventRequest
	if !decodeStrict(w, r, &req) {
		return
	}
	patch, err := req.patch()
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	view, err := h.events.Update(r.Context(), viewer, id, occurrence, patch)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toEventResponse(view))
}

// DeleteEvent deletes an event or, with ?occurrence=, cancels one occurrence.
func (h *Handler) DeleteEvent(w http.ResponseWriter, r *http.Request) {
	viewer, ok := viewerOf(w, r)
	if !ok {
		return
	}
	id, ok := eventID(w, r)
	if !ok {
		return
	}
	occurrence, err := occurrenceParam(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	if err := h.events.Delete(r.Context(), viewer, id, occurrence); err != nil {
		httpresponse.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type respondRequest struct {
	Response string `json:"response"`
}

// Respond stores the RSVP of the caller.
func (h *Handler) Respond(w http.ResponseWriter, r *http.Request) {
	viewer, ok := viewerOf(w, r)
	if !ok {
		return
	}
	id, ok := eventID(w, r)
	if !ok {
		return
	}
	var req respondRequest
	if !decodeStrict(w, r, &req) {
		return
	}
	if err := h.events.Respond(r.Context(), viewer, id, domain.Response(req.Response)); err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, respondRequest{Response: req.Response})
}

type itemResponse struct {
	Kind            string     `json:"kind"`
	Title           string     `json:"title"`
	StartsAt        time.Time  `json:"starts_at"`
	EndsAt          time.Time  `json:"ends_at"`
	AllDay          bool       `json:"all_day"`
	Date            string     `json:"date"`
	SourceID        string     `json:"source_id"`
	ProjectID       *string    `json:"project_id"`
	ProjectKey      string     `json:"project_key"`
	Link            string     `json:"link"`
	Color           string     `json:"color"`
	Location        string     `json:"location"`
	Timezone        string     `json:"timezone"`
	Visibility      string     `json:"visibility"`
	Recurring       bool       `json:"recurring"`
	OccurrenceStart *time.Time `json:"occurrence_start"`
	CanEdit         bool       `json:"can_edit"`
	MyResponse      *string    `json:"my_response"`
}

type itemsResponse struct {
	Items     []itemResponse `json:"items"`
	Truncated bool           `json:"truncated"`
}

// parseWindow reads from and to. A plain date `to` is inclusive; an RFC 3339 `to` is exclusive.
func parseWindow(r *http.Request) (time.Time, time.Time, error) {
	from, _, err := parseInstant(r.URL.Query().Get("from"))
	if err != nil {
		return time.Time{}, time.Time{}, validation("calendar.invalid_range", "from debe usar el formato AAAA-MM-DD o RFC 3339")
	}
	to, dateOnly, err := parseInstant(r.URL.Query().Get("to"))
	if err != nil {
		return time.Time{}, time.Time{}, validation("calendar.invalid_range", "to debe usar el formato AAAA-MM-DD o RFC 3339")
	}
	if dateOnly {
		to = to.AddDate(0, 0, 1)
	}
	return from, to, nil
}

// Items returns the merged calendar: event occurrences plus dates derived from issues, sprints and versions.
func (h *Handler) Items(w http.ResponseWriter, r *http.Request) {
	viewer, ok := viewerOf(w, r)
	if !ok {
		return
	}
	from, to, err := parseWindow(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	q := port.ItemsQuery{From: from, To: to}
	if raw := r.URL.Query().Get("projects"); raw != "" {
		if q.Projects, err = parseUUIDs(strings.Split(raw, ","), "projects"); err != nil {
			httpresponse.Error(w, validation("calendar.invalid_projects", "projects debe ser una lista de ids separados por comas"))
			return
		}
	}
	if raw := r.URL.Query().Get("types"); raw != "" {
		q.Kinds = strings.Split(raw, ",")
	}
	result, err := h.events.Items(r.Context(), viewer, q)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	out := itemsResponse{Items: make([]itemResponse, 0, len(result.Items)), Truncated: result.Truncated}
	for _, it := range result.Items {
		item := itemResponse{
			Kind: it.Kind, Title: it.Title, StartsAt: it.StartsAt, EndsAt: it.EndsAt, AllDay: it.AllDay,
			Date: it.StartsAt.Format(dateLayout), SourceID: it.SourceID.String(), ProjectKey: it.ProjectKey, Link: it.Link,
			Color: it.Color, Location: it.Location, Timezone: it.Timezone, Visibility: it.Visibility, Recurring: it.Recurring,
			OccurrenceStart: it.OccurrenceStart, CanEdit: it.CanEdit,
		}
		if it.ProjectID != nil {
			s := it.ProjectID.String()
			item.ProjectID = &s
		}
		if it.MyResponse != "" {
			s := string(it.MyResponse)
			item.MyResponse = &s
		}
		out.Items = append(out.Items, item)
	}
	httpresponse.JSON(w, http.StatusOK, out)
}

type feedResponse struct {
	Enabled   bool       `json:"enabled"`
	CreatedAt *time.Time `json:"created_at"`
	Token     string     `json:"token,omitempty"`
	URL       string     `json:"url,omitempty"`
}

// FeedInfo tells whether the caller has an active feed.
func (h *Handler) FeedInfo(w http.ResponseWriter, r *http.Request) {
	viewer, ok := viewerOf(w, r)
	if !ok {
		return
	}
	at, err := h.events.FeedInfo(r.Context(), viewer.UserID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, feedResponse{Enabled: at != nil, CreatedAt: at})
}

// CreateFeed creates or rotates the feed token; the URL is returned only here.
func (h *Handler) CreateFeed(w http.ResponseWriter, r *http.Request) {
	viewer, ok := viewerOf(w, r)
	if !ok {
		return
	}
	token, err := h.events.CreateFeed(r.Context(), viewer.UserID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	at := token.CreatedAt
	httpresponse.JSON(w, http.StatusCreated, feedResponse{Enabled: true, CreatedAt: &at, Token: token.Token,
		URL: h.publicURL + "/api/v1/calendar/export.ics?token=" + token.Token})
}

// RevokeFeed disables the feed.
func (h *Handler) RevokeFeed(w http.ResponseWriter, r *http.Request) {
	viewer, ok := viewerOf(w, r)
	if !ok {
		return
	}
	if err := h.events.RevokeFeed(r.Context(), viewer.UserID); err != nil {
		httpresponse.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ExportICS serves the private feed. The token is the credential, so an unknown or revoked one answers 404.
// The response is cacheable by the subscriber for a few minutes and supports conditional requests.
func (h *Handler) ExportICS(w http.ResponseWriter, r *http.Request) {
	body, err := h.events.RenderFeed(r.Context(), r.URL.Query().Get("token"))
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	sum := sha256.Sum256([]byte(body))
	etag := `"` + hex.EncodeToString(sum[:16]) + `"`
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", `inline; filename="corelog.ics"`)
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("ETag", etag)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	_, _ = io.WriteString(w, body)
}
