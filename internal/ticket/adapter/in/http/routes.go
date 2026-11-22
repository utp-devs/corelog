package http

import (
	"github.com/go-chi/chi/v5"

	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
)

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/issues", func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))

		r.Post("/", h.Create)
		r.Get("/", h.List)
		r.Get("/{id}", h.Get)
		r.Patch("/{id}/status", h.UpdateStatus)
		r.Patch("/{id}/assign", h.Assign)
		r.Patch("/{id}/metadata", h.UpdateMetadata)
		r.Patch("/{id}/rank", h.UpdateRank)
		r.Patch("/{id}/details", h.UpdateDetails)
		r.Get("/{id}/comments", h.ListComments)
		r.Post("/{id}/comments", h.CreateComment)
		r.Patch("/{id}/comments/{comment_id}", h.UpdateComment)
		r.Delete("/{id}/comments/{comment_id}", h.DeleteComment)
		r.Get("/{id}/activities", h.ListActivities)
		r.Get("/{id}/links", h.ListIssueLinks)
		r.Post("/{id}/links", h.CreateIssueLink)
		r.Delete("/{id}/links/{link_id}", h.DeleteIssueLink)
		r.Get("/{id}/attachments", h.ListAttachments)
		r.Post("/{id}/attachments", h.UploadAttachment)
		r.Get("/{id}/attachments/{attachment_id}", h.DownloadAttachment)
		r.Get("/{id}/watchers", h.GetWatcherState)
		r.Post("/{id}/watchers", h.WatchIssue)
		r.Delete("/{id}/watchers", h.UnwatchIssue)
	})
	r.Route("/workflows", func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))
		r.Get("/", h.GetDefaultWorkflow)
	})
	r.Route("/saved-filters", func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))
		r.Get("/", h.ListSavedFilters)
		r.Post("/", h.CreateSavedFilter)
		r.Delete("/{filter_id}", h.DeleteSavedFilter)
	})
}
