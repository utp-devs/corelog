package http

import (
	"net/http"
	"strconv"
	"time"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
)

type activityFeedResponse struct {
	Data []activityEntryResponse `json:"data"`
	Meta activityFeedMeta        `json:"meta"`
}

type activityFeedMeta struct {
	Limit      int     `json:"limit"`
	NextCursor *string `json:"next_cursor"`
}

type activityEntryResponse struct {
	ID        string                   `json:"id"`
	Type      string                   `json:"type"`
	CreatedAt time.Time                `json:"created_at"`
	Actor     activityActorResponse    `json:"actor"`
	Comment   *activityCommentResponse `json:"comment,omitempty"`
	Changes   []activityChangeResponse `json:"changes,omitempty"`
}

type activityActorResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type activityCommentResponse struct {
	ID         string    `json:"id"`
	Body       string    `json:"body"`
	UpdatedAt  time.Time `json:"updated_at"`
	Edited     bool      `json:"edited"`
	Deleted    bool      `json:"deleted"`
	Visibility string    `json:"visibility"`
}

type activityChangeResponse struct {
	Action string  `json:"action"`
	Field  string  `json:"field"`
	Label  string  `json:"label"`
	From   *string `json:"from"`
	To     *string `json:"to"`
}

// ListActivityFeed serves GET /issues/{id}/activity: comments and history merged, newest first, paged by an
// opaque cursor. Query: filter=all|comments|history, cursor, limit (1..100, default 25).
func (h *Handler) ListActivityFeed(w http.ResponseWriter, r *http.Request) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	issueID, err := h.issueRef(r, "id")
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	feed, ok := h.service.(port.ActivityFeed)
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindInternal, "activity feed is not available"))
		return
	}
	query := r.URL.Query()
	limit, given, err := parseActivityLimit(query.Get("limit"))
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	page, err := feed.ListActivityFeed(r.Context(), issueID, actorID, globalRole, port.ActivityQuery{
		Filter:     query.Get("filter"),
		Cursor:     query.Get("cursor"),
		Limit:      limit,
		LimitGiven: given,
	})
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	effective, err := domain.NormalizeActivityLimit(limit, given)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	body := activityFeedResponse{
		Data: toActivityEntryResponses(page.Entries),
		Meta: activityFeedMeta{Limit: effective},
	}
	if page.NextCursor != "" {
		cursor := page.NextCursor
		body.Meta.NextCursor = &cursor
	}
	httpresponse.JSON(w, http.StatusOK, body)
}

// parseActivityLimit reads the limit query value. given is false when the parameter is absent.
func parseActivityLimit(raw string) (int, bool, error) {
	if raw == "" {
		return 0, false, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false, domain.ErrInvalidActivityLimit
	}
	return limit, true, nil
}

func toActivityEntryResponses(entries []port.ActivityEntry) []activityEntryResponse {
	out := make([]activityEntryResponse, 0, len(entries))
	for _, entry := range entries {
		item := activityEntryResponse{
			ID: entry.ID.String(), Type: entry.Kind, CreatedAt: entry.At,
			Actor: activityActorResponse{ID: entry.ActorID.String(), Name: entry.ActorName},
		}
		if entry.Comment != nil {
			item.Comment = &activityCommentResponse{
				ID: entry.Comment.ID.String(), Body: entry.Comment.Body,
				UpdatedAt: entry.Comment.UpdatedAt, Edited: entry.Comment.Edited,
				Deleted: entry.Comment.Deleted, Visibility: entry.Comment.Visibility,
			}
		}
		for _, change := range entry.Changes {
			item.Changes = append(item.Changes, activityChangeResponse{
				Action: change.Action, Field: change.Field, Label: change.Label, From: change.From, To: change.To,
			})
		}
		out = append(out, item)
	}
	return out
}
