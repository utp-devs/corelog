package http

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
	"github.com/jhonsferg/corelog/internal/platform/ratelimit"
	"github.com/jhonsferg/corelog/internal/platform/validation"
	"github.com/jhonsferg/corelog/internal/user/application/port"
	"github.com/jhonsferg/corelog/internal/user/domain"
)

// RateLimits holds one limiter per abuse sensitive endpoint. A nil limiter (or a nil RateLimits) allows
// everything, which is the default of the HTTP tests.
type RateLimits struct {
	Login         *ratelimit.Limiter
	LoginTwoFact  *ratelimit.Limiter
	Register      *ratelimit.Limiter
	Forgot        *ratelimit.Limiter
	ForgotAccount *ratelimit.Limiter
	Reset         *ratelimit.Limiter
	Phone         *ratelimit.Limiter
}

// Security groups the optional abuse protection and recovery collaborators of the handler.
type Security struct {
	Recovery port.RecoveryService
	Guard    port.LoginGuard
	Limits   *RateLimits
	// ClientIP resolves the address used as the rate limit key (trusted proxies aware). Nil uses the TCP peer.
	ClientIP func(*http.Request) string
}

// WithSecurity enables password recovery, email verification, rate limits and account lockout.
func (h *Handler) WithSecurity(s Security) *Handler {
	h.sec = s
	return h
}

const forgotMessage = "Si el correo está registrado, recibirás un mensaje con las instrucciones para restablecer tu contraseña"

type forgotRequest struct {
	Email string `json:"email" validate:"required,email,max=254"`
}

type resetRequest struct {
	Token                string `json:"token" validate:"required,max=200"`
	Password             string `json:"password" validate:"required"`
	PasswordConfirmation string `json:"password_confirmation"`
}

type verifyRequest struct {
	Token string `json:"token" validate:"required,max=200"`
}

type messageResponse struct {
	Message string `json:"message"`
}

func (h *Handler) clientIP(r *http.Request) string {
	if h.sec.ClientIP != nil {
		return h.sec.ClientIP(r)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (h *Handler) limits() *RateLimits {
	if h.sec.Limits == nil {
		return &RateLimits{}
	}
	return h.sec.Limits
}

// allowIP takes a token for the client address from the limiter; it writes the 429 response when denied.
func (h *Handler) allowIP(w http.ResponseWriter, r *http.Request, l *ratelimit.Limiter, scope string) bool {
	return ratelimit.Check(l, w, scope+":"+h.clientIP(r))
}

func emailHash(email string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(email))))
	return hex.EncodeToString(sum[:])
}

func strictDecode(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return false
	}
	if err := validation.Struct(dst); err != nil {
		httpresponse.Error(w, validation.Failed(dst, err))
		return false
	}
	return true
}

func (h *Handler) requireRecovery(w http.ResponseWriter) bool {
	if h.sec.Recovery == nil {
		httpresponse.Error(w, domain.ErrVerificationUnavailable)
		return false
	}
	return true
}

// ForgotPassword answers 202 with the same body whether or not the email belongs to an account.
func (h *Handler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	if !h.requireRecovery(w) || !h.allowIP(w, r, h.limits().Forgot, "forgot") {
		return
	}
	var req forgotRequest
	if !strictDecode(w, r, &req) {
		return
	}
	if !ratelimit.Check(h.limits().ForgotAccount, w, "forgot:"+emailHash(req.Email)) {
		return
	}
	if err := h.sec.Recovery.RequestPasswordReset(r.Context(), req.Email); err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusAccepted, messageResponse{Message: forgotMessage})
}

// ResetPassword sets a new password from the emailed token.
func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	if !h.requireRecovery(w) || !h.allowIP(w, r, h.limits().Reset, "reset") {
		return
	}
	var req resetRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	// The confirmation is compared first, like in registration and password change.
	if err := domain.CheckPasswordConfirmation(req.Password, req.PasswordConfirmation, "password_confirmation"); err != nil {
		httpresponse.Error(w, err)
		return
	}
	if err := validation.Struct(req); err != nil {
		httpresponse.Error(w, validation.Failed(req, err))
		return
	}
	if err := h.sec.Recovery.ResetPassword(r.Context(), port.ResetPasswordInput{
		Token: req.Token, Password: req.Password, PasswordConfirmation: req.PasswordConfirmation,
	}); err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusNoContent, nil)
}

// RequestEmailVerification mails a verification link to the authenticated user.
func (h *Handler) RequestEmailVerification(w http.ResponseWriter, r *http.Request) {
	userID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	if !h.requireRecovery(w) || !ratelimit.Check(h.limits().Forgot, w, "verify:"+userID.String()) {
		return
	}
	if err := h.sec.Recovery.RequestEmailVerification(r.Context(), userID); err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusAccepted, messageResponse{Message: "Te enviamos un enlace para verificar tu correo electrónico"})
}

// VerifyEmail marks the address as verified from the emailed token.
func (h *Handler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	if !h.requireRecovery(w) || !h.allowIP(w, r, h.limits().Reset, "verify") {
		return
	}
	var req verifyRequest
	if !strictDecode(w, r, &req) {
		return
	}
	if err := h.sec.Recovery.VerifyEmail(r.Context(), req.Token); err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusNoContent, nil)
}

// lockedResponse answers 429 account_locked with the remaining time.
func lockedResponse(w http.ResponseWriter, retryAfter time.Duration) {
	ratelimit.Write(w, ratelimit.CodeAccountLocked,
		"Demasiados intentos fallidos: la cuenta está bloqueada temporalmente, intenta de nuevo más tarde", retryAfter)
}

// recordLogin clears the failure counter and records the login event once a session is open. Failures here
// never block the login.
func (h *Handler) recordLogin(r *http.Request, user *domain.User) {
	if h.sec.Guard == nil {
		return
	}
	_ = h.sec.Guard.RecordSuccess(r.Context(), user)
}

// announceLogin raises the new login alert once the session is open. It never blocks or fails the login.
func (h *Handler) announceLogin(r *http.Request, userID, sessionID uuid.UUID) {
	if h.events == nil {
		return
	}
	h.events.NewLogin(r.Context(), port.LoginInfo{UserID: userID, SessionID: sessionID, UserAgent: r.UserAgent(), IP: h.clientIP(r)})
}
