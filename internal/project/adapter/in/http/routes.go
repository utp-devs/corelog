package http

import (
	"github.com/go-chi/chi/v5"

	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
)

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/projects", func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))
		r.Get("/", h.List)
		// Static segments are registered explicitly so they are never captured by {id}.
		r.Get("/key-available", h.KeyAvailable)

		// {id} accepts a project UUID or its key. organization_id in the query narrows a key lookup.
		r.Get("/{id}", h.Get)
		r.Get("/{id}/members", h.ListMembers)
		r.Post("/{id}/members", h.AddMember)
		r.Patch("/{id}/members/{user_id}", h.UpdateMember)
		r.Delete("/{id}/members/{user_id}", h.RemoveMember)
		r.Get("/{id}/team-roles", h.ListTeamGrants)
		r.Put("/{id}/team-roles", h.SetTeamGrant)
		r.Delete("/{id}/team-roles/{team_id}", h.RemoveTeamGrant)
		r.Get("/{id}/permissions", h.GetPermissions)
		r.Put("/{id}/permissions", h.ReplacePermissions)
		r.Post("/{id}/permissions/reset", h.ResetPermissions)
		r.Get("/{id}/permissions/me", h.MyPermissions)
		r.Get("/{id}/labels", h.ListLabels)
		r.Post("/{id}/labels", h.CreateLabel)
		r.Delete("/{id}/labels/{label_id}", h.DeleteLabel)
		r.Get("/{id}/components", h.ListComponents)
		r.Post("/{id}/components", h.CreateComponent)
		r.Delete("/{id}/components/{component_id}", h.DeleteComponent)
		r.Post("/{id}/archive", h.Archive)
		r.Post("/{id}/restore", h.Restore)

		// Creation is decided by the org role and policy in the use case, not by the platform role.
		r.Post("/", h.Create)
		r.Patch("/{id}", h.Update)
		r.Delete("/{id}", h.Delete)
	})
}
