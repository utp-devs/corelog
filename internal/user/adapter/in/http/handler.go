package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/authtoken"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
	"github.com/jhonsferg/corelog/internal/platform/ratelimit"
	"github.com/jhonsferg/corelog/internal/platform/validation"
	"github.com/jhonsferg/corelog/internal/user/application/port"
	"github.com/jhonsferg/corelog/internal/user/domain"
)

type Handler struct {
	service       port.Service
	jwtSecret     string
	jwtExpiration time.Duration
	// twoFactor is nil when the second factor is not wired; logins then never ask for it.
	twoFactor port.TwoFactorService
	// consents is nil when the legal module is not wired; registration then asks for no consents.
	consents port.ConsentGate
	// sec holds the optional recovery, rate limit and lockout collaborators.
	sec Security
}

func NewHandler(service port.Service, jwtSecret string, jwtExpiration time.Duration) *Handler {
	return &Handler{service: service, jwtSecret: jwtSecret, jwtExpiration: jwtExpiration}
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	if !h.allowIP(w, r, h.limits().Register, "register") {
		return
	}
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	// The confirmation is checked before every other validation, so a mismatch never reaches the policy.
	if err := domain.CheckPasswordConfirmation(req.Password, req.PasswordConfirmation, "password_confirmation"); err != nil {
		httpresponse.Error(w, err)
		return
	}
	if err := validation.Struct(req); err != nil {
		httpresponse.Error(w, validation.Failed(req, err))
		return
	}

	user, err := h.service.Register(r.Context(), port.RegisterInput{
		FirstName:            req.FirstName,
		LastName:             req.LastName,
		Name:                 req.Name,
		Email:                req.Email,
		Password:             req.Password,
		PasswordConfirmation: req.PasswordConfirmation,
		Phone:                req.Phone.toChange(),
		Locale:               req.Locale,
		Consents:             toConsentRefs(req.Consents),
		ConsentMeta:          consentMeta(r, "register"),
	})
	if err != nil {
		httpresponse.Error(w, err)
		return
	}

	httpresponse.JSON(w, http.StatusCreated, toPrivateUserResponse(user))
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if !h.allowIP(w, r, h.limits().Login, "login") {
		return
	}
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	if err := validation.Struct(req); err != nil {
		httpresponse.Error(w, validation.Failed(req, err))
		return
	}

	if h.sec.Guard != nil {
		locked, wait, err := h.sec.Guard.Check(r.Context(), req.Email)
		if err != nil {
			httpresponse.Error(w, err)
			return
		}
		if locked {
			lockedResponse(w, wait)
			return
		}
	}

	user, err := h.service.Authenticate(r.Context(), port.AuthenticateInput{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		if h.sec.Guard != nil && apperror.CodeOf(err) == domain.ErrInvalidCredentials.Code {
			if locked, wait, gerr := h.sec.Guard.RecordFailure(r.Context(), req.Email); gerr == nil && locked {
				lockedResponse(w, wait)
				return
			}
		}
		httpresponse.Error(w, err)
		return
	}

	// With the second factor enabled the password only starts a challenge; the session opens after the code.
	if h.twoFactor != nil {
		enabled, err := h.twoFactor.IsEnabled(r.Context(), user.ID)
		if err != nil {
			httpresponse.Error(w, err)
			return
		}
		if enabled {
			challengeID, expiresAt, err := h.twoFactor.StartChallenge(r.Context(), user.ID)
			if err != nil {
				httpresponse.Error(w, err)
				return
			}
			token, err := signChallengeToken(h.jwtSecret, user.ID, challengeID, expiresAt)
			if err != nil {
				httpresponse.Error(w, apperror.Wrap(apperror.KindInternal, "failed to issue challenge", err))
				return
			}
			httpresponse.JSON(w, http.StatusOK, twoFactorChallengeResponse{TwoFactorRequired: true, ChallengeToken: token})
			return
		}
	}

	sessionID, err := h.service.OpenSession(r.Context(), user.ID, r.UserAgent(), time.Now().Add(h.jwtExpiration))
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	h.recordLogin(r, user)

	token, err := authtoken.GenerateWithSession(h.jwtSecret, h.jwtExpiration, user.ID, string(user.Role), sessionID)
	if err != nil {
		httpresponse.Error(w, apperror.Wrap(apperror.KindInternal, "failed to issue token", err))
		return
	}

	h.writeSession(w, r, token, user)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}

	user, err := h.service.GetByID(r.Context(), userID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}

	h.writeMe(w, r, user)
}

func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}

	var req updateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	if err := validation.Struct(req); err != nil {
		httpresponse.Error(w, validation.Failed(req, err))
		return
	}

	user, err := h.service.UpdateProfile(r.Context(), userID, port.UpdateProfileInput{
		Name:        req.Name,
		FirstName:   req.FirstName,
		LastName:    req.LastName,
		Phone:       req.Phone.toChange(),
		Locale:      req.Locale,
		Email:       req.Email,
		DisplayName: req.DisplayName,
		JobTitle:    req.JobTitle,
		Department:  req.Department,
		Timezone:    req.Timezone,
	})
	if err != nil {
		httpresponse.Error(w, err)
		return
	}

	httpresponse.JSON(w, http.StatusOK, toPrivateUserResponse(user))
}

func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}

	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	if err := domain.CheckPasswordConfirmation(req.NewPassword, req.NewPasswordConfirmation, "new_password_confirmation"); err != nil {
		httpresponse.Error(w, err)
		return
	}
	if err := validation.Struct(req); err != nil {
		httpresponse.Error(w, validation.Failed(req, err))
		return
	}

	if err := h.service.ChangePassword(r.Context(), userID, port.ChangePasswordInput{
		CurrentPassword:         req.CurrentPassword,
		NewPassword:             req.NewPassword,
		NewPasswordConfirmation: req.NewPasswordConfirmation,
	}); err != nil {
		httpresponse.Error(w, err)
		return
	}

	httpresponse.JSON(w, http.StatusNoContent, nil)
}

func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	userID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}

	q := r.URL.Query()
	query := q.Get("q")

	// Without organization_id only platform admins keep the team-based search; everyone else must name the organization.
	orgParam := q.Get("organization_id")
	if orgParam == "" {
		if role, _ := appmw.RoleFromContext(r.Context()); role != string(domain.RoleAdmin) {
			httpresponse.Error(w, errOrganizationIDRequired)
			return
		}
		users, err := h.service.SearchTeammates(r.Context(), userID, query)
		if err != nil {
			httpresponse.Error(w, err)
			return
		}
		httpresponse.JSON(w, http.StatusOK, toUserResponses(users, true, userID))
		return
	}

	orgID, err := uuid.Parse(orgParam)
	if err != nil {
		httpresponse.Error(w, errOrganizationIDInvalid)
		return
	}

	users, err := h.service.SearchInOrganization(r.Context(), userID, orgID, query)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}

	showPhones, err := h.service.CanViewMemberPhones(r.Context(), userID, orgID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}

	httpresponse.JSON(w, http.StatusOK, toUserResponses(users, showPhones, userID))
}

var (
	errOrganizationIDRequired = apperror.WithCode(apperror.KindValidation, "validation.failed", "organization_id es obligatorio")
	errOrganizationIDInvalid  = apperror.WithCode(apperror.KindValidation, "validation.failed", "organization_id no es un UUID valido")
)

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filter := port.SearchFilter{Query: q.Get("q")}
	if teamIDStr := q.Get("team_id"); teamIDStr != "" {
		teamID, err := uuid.Parse(teamIDStr)
		if err != nil {
			httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid team id"))
			return
		}
		filter.TeamID = &teamID
	}

	users, err := h.service.Search(r.Context(), filter)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}

	httpresponse.JSON(w, http.StatusOK, toUserResponses(users, true, uuid.Nil))
}

func (h *Handler) SetTeam(w http.ResponseWriter, r *http.Request) {
	targetID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid user id"))
		return
	}

	var req setTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	if err := validation.Struct(req); err != nil {
		httpresponse.Error(w, validation.Failed(req, err))
		return
	}

	var teamID *uuid.UUID
	if req.TeamID != nil {
		parsed, err := uuid.Parse(*req.TeamID)
		if err != nil {
			httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid team id"))
			return
		}
		teamID = &parsed
	}

	user, err := h.service.SetTeam(r.Context(), targetID, teamID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}

	httpresponse.JSON(w, http.StatusOK, toPrivateUserResponse(user))
}

// currentSession devuelve la sesion del token de la peticion. Sin validacion de sesiones devuelve uuid.Nil.
func currentSession(r *http.Request) uuid.UUID {
	id, _ := appmw.SessionIDFromContext(r.Context())
	return id
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	userID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	if err := h.service.RevokeSession(r.Context(), userID, currentSession(r)); err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusNoContent, nil)
}

func (h *Handler) ListSessions(w http.ResponseWriter, r *http.Request) {
	userID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	views, err := h.service.ListSessions(r.Context(), userID, currentSession(r))
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	out := make([]sessionResponse, 0, len(views))
	for _, v := range views {
		out = append(out, toSessionResponse(v))
	}
	httpresponse.JSON(w, http.StatusOK, out)
}

func (h *Handler) RevokeSession(w http.ResponseWriter, r *http.Request) {
	userID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	sessionID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid session id"))
		return
	}
	if err := h.service.RevokeSession(r.Context(), userID, sessionID); err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusNoContent, nil)
}

func (h *Handler) RevokeOtherSessions(w http.ResponseWriter, r *http.Request) {
	userID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	if err := h.service.RevokeOtherSessions(r.Context(), userID, currentSession(r)); err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusNoContent, nil)
}

func (h *Handler) DeactivateAccount(w http.ResponseWriter, r *http.Request) {
	userID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	var req deactivateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	if err := validation.Struct(req); err != nil {
		httpresponse.Error(w, validation.Failed(req, err))
		return
	}
	if err := h.service.DeactivateAccount(r.Context(), userID, req.Password); err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusNoContent, nil)
}

// toChange converts the optional phone of a partial update into the use case input; nil when omitted.
func (o optionalPhone) toChange() *port.PhoneChange {
	switch {
	case !o.Present:
		return nil
	case o.Value == nil:
		return &port.PhoneChange{Clear: true}
	default:
		return &port.PhoneChange{Country: o.Value.Country, Number: o.Value.Number}
	}
}

// RequestPhoneVerification sends a 6-digit SMS code to the phone of the caller.
func (h *Handler) RequestPhoneVerification(w http.ResponseWriter, r *http.Request) {
	userID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	if !ratelimit.Check(h.limits().Phone, w, "phone:"+userID.String()) {
		return
	}
	receipt, err := h.service.RequestPhoneVerification(r.Context(), userID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusAccepted, phoneVerificationResponse{Destination: receipt.Destination, ExpiresAt: receipt.ExpiresAt})
}

// VerifyPhone checks the SMS code and marks the phone of the caller as verified.
func (h *Handler) VerifyPhone(w http.ResponseWriter, r *http.Request) {
	userID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	var req phoneVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	if err := validation.Struct(req); err != nil {
		httpresponse.Error(w, validation.Failed(req, err))
		return
	}
	user, err := h.service.VerifyPhone(r.Context(), userID, req.Code)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toPrivateUserResponse(user))
}

func (p *phoneRequest) toChange() *port.PhoneChange {
	if p == nil {
		return nil
	}
	return &port.PhoneChange{Country: p.Country, Number: p.Number}
}
