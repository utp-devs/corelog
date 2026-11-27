package http

import (
	"github.com/go-chi/chi/v5"

	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
)

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/meta/countries", h.Countries)

	r.Route("/auth", func(r chi.Router) {
		r.Post("/register", h.Register)
		r.Post("/login", h.Login)
		r.Post("/login/2fa", h.LoginSecondFactor)

		r.Group(func(r chi.Router) {
			r.Use(appmw.Authenticate(h.jwtSecret))
			r.Post("/logout", h.Logout)
			r.Get("/2fa", h.TwoFactorStatus)
			r.Post("/2fa/enroll", h.TwoFactorEnroll)
			r.Post("/2fa/confirm", h.TwoFactorConfirm)
			r.Post("/2fa/disable", h.TwoFactorDisable)
			r.Post("/2fa/recovery-codes", h.TwoFactorRegenerateRecoveryCodes)
		})
	})

	r.Route("/users", func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))

		r.Get("/me", h.Me)
		r.Patch("/me", h.UpdateProfile)
		r.Post("/me/password", h.ChangePassword)
		r.Post("/me/phone/verification", h.RequestPhoneVerification)
		r.Post("/me/phone/verify", h.VerifyPhone)
		r.Post("/me/deactivate", h.DeactivateAccount)
		r.Get("/me/sessions", h.ListSessions)
		r.Delete("/me/sessions/{id}", h.RevokeSession)
		r.Post("/me/sessions/revoke-others", h.RevokeOtherSessions)
		r.Get("/search", h.Search)

		r.Group(func(r chi.Router) {
			r.Use(appmw.RequireRole("admin"))

			r.Get("/", h.List)
			r.Patch("/{id}/team", h.SetTeam)
		})
	})
}
