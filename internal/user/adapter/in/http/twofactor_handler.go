package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/authtoken"
	"github.com/jhonsferg/corelog/internal/platform/crypto"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
	"github.com/jhonsferg/corelog/internal/platform/validation"
	"github.com/jhonsferg/corelog/internal/user/application/port"
	"github.com/jhonsferg/corelog/internal/user/domain"
)

// challengePurpose marks the login challenge token. It is signed with a key derived from the access token
// secret, so an access token can never be replayed as a challenge and the reverse.
const challengePurpose = "two_factor_challenge"

// challengeKeyInfo separates the challenge signing key from the other keys derived from the JWT secret.
const challengeKeyInfo = "corelog two-factor login challenge"

type challengeClaims struct {
	Purpose string `json:"purpose"`
	jwt.RegisteredClaims
}

// signChallengeToken issues the short-lived token that stands for a login waiting for its second factor.
func signChallengeToken(secret string, userID, challengeID uuid.UUID, expiresAt time.Time) (string, error) {
	key, err := crypto.DeriveKey([]byte(secret), challengeKeyInfo)
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	claims := challengeClaims{
		Purpose: challengePurpose,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        challengeID.String(),
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			Issuer:    "corelog-api",
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(key)
}

// parseChallengeToken returns the user and challenge a valid challenge token names.
func parseChallengeToken(secret, raw string) (userID, challengeID uuid.UUID, err error) {
	key, err := crypto.DeriveKey([]byte(secret), challengeKeyInfo)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	claims := &challengeClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, authtoken.ErrInvalidToken
		}
		return key, nil
	})
	if err != nil || !token.Valid || claims.Purpose != challengePurpose {
		return uuid.Nil, uuid.Nil, domain.ErrChallengeExpired
	}
	userID, err = uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, uuid.Nil, domain.ErrChallengeExpired
	}
	challengeID, err = uuid.Parse(claims.ID)
	if err != nil {
		return uuid.Nil, uuid.Nil, domain.ErrChallengeExpired
	}
	return userID, challengeID, nil
}

// WithTwoFactor enables the second factor step of the login and the two-factor endpoints.
func (h *Handler) WithTwoFactor(svc port.TwoFactorService) *Handler {
	h.twoFactor = svc
	return h
}

type twoFactorStatusResponse struct {
	Enabled                bool       `json:"enabled"`
	EnabledAt              *time.Time `json:"enabled_at"`
	RecoveryCodesRemaining int        `json:"recovery_codes_remaining"`
}

type twoFactorEnrollResponse struct {
	Secret     string `json:"secret"`
	OTPAuthURI string `json:"otpauth_uri"`
}

type recoveryCodesResponse struct {
	RecoveryCodes []string `json:"recovery_codes"`
}

type twoFactorCodeRequest struct {
	Code string `json:"code" validate:"required,max=16"`
}

type twoFactorDisableRequest struct {
	Password     string `json:"password" validate:"required"`
	Code         string `json:"code" validate:"omitempty,max=16"`
	RecoveryCode string `json:"recovery_code" validate:"omitempty,max=64"`
}

type twoFactorLoginRequest struct {
	ChallengeToken string `json:"challenge_token" validate:"required"`
	Code           string `json:"code" validate:"omitempty,max=16"`
	RecoveryCode   string `json:"recovery_code" validate:"omitempty,max=64"`
}

type twoFactorChallengeResponse struct {
	TwoFactorRequired bool   `json:"two_factor_required"`
	ChallengeToken    string `json:"challenge_token"`
}

// twoFactorUnavailable is returned by the endpoints when no second factor service is wired.
var twoFactorUnavailable = apperror.New(apperror.KindInternal, "la autenticacion en dos pasos no esta disponible")

func (h *Handler) requireTwoFactor(w http.ResponseWriter) bool {
	if h.twoFactor == nil {
		httpresponse.Error(w, twoFactorUnavailable)
		return false
	}
	return true
}

// userIDOrFail returns the authenticated user or writes the 401 response.
func userIDOrFail(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	userID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return uuid.Nil, false
	}
	return userID, true
}

// decodeValidated decodes a strict JSON body and runs the struct validation.
func decodeValidated(w http.ResponseWriter, r *http.Request, dst any) bool {
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

func (h *Handler) TwoFactorStatus(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDOrFail(w, r)
	if !ok || !h.requireTwoFactor(w) {
		return
	}
	status, err := h.twoFactor.Status(r.Context(), userID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, twoFactorStatusResponse{
		Enabled:                status.Enabled,
		EnabledAt:              status.EnabledAt,
		RecoveryCodesRemaining: status.RecoveryCodesRemaining,
	})
}

func (h *Handler) TwoFactorEnroll(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDOrFail(w, r)
	if !ok || !h.requireTwoFactor(w) {
		return
	}
	enrollment, err := h.twoFactor.Enroll(r.Context(), userID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, twoFactorEnrollResponse{Secret: enrollment.Secret, OTPAuthURI: enrollment.OTPAuthURI})
}

func (h *Handler) TwoFactorConfirm(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDOrFail(w, r)
	if !ok || !h.requireTwoFactor(w) {
		return
	}
	var req twoFactorCodeRequest
	if !decodeValidated(w, r, &req) {
		return
	}
	codes, err := h.twoFactor.ConfirmEnrollment(r.Context(), userID, req.Code)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, recoveryCodesResponse{RecoveryCodes: codes})
}

func (h *Handler) TwoFactorDisable(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDOrFail(w, r)
	if !ok || !h.requireTwoFactor(w) {
		return
	}
	var req twoFactorDisableRequest
	if !decodeValidated(w, r, &req) {
		return
	}
	if err := h.twoFactor.Disable(r.Context(), userID, req.Password, port.CompleteLoginInput{
		Code:         req.Code,
		RecoveryCode: req.RecoveryCode,
	}); err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusNoContent, nil)
}

func (h *Handler) TwoFactorRegenerateRecoveryCodes(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDOrFail(w, r)
	if !ok || !h.requireTwoFactor(w) {
		return
	}
	var req twoFactorCodeRequest
	if !decodeValidated(w, r, &req) {
		return
	}
	codes, err := h.twoFactor.RegenerateRecoveryCodes(r.Context(), userID, req.Code)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, recoveryCodesResponse{RecoveryCodes: codes})
}

// LoginSecondFactor completes a login that was answered with two_factor_required. It is public: the
// challenge token is the proof that the password was already checked.
func (h *Handler) LoginSecondFactor(w http.ResponseWriter, r *http.Request) {
	if !h.requireTwoFactor(w) || !h.allowIP(w, r, h.limits().LoginTwoFact, "login2fa") {
		return
	}
	var req twoFactorLoginRequest
	if !decodeValidated(w, r, &req) {
		return
	}
	userID, challengeID, err := parseChallengeToken(h.jwtSecret, req.ChallengeToken)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	user, err := h.twoFactor.CompleteChallenge(r.Context(), challengeID, userID, port.CompleteLoginInput{
		Code:         req.Code,
		RecoveryCode: req.RecoveryCode,
	})
	if err != nil {
		httpresponse.Error(w, err)
		return
	}

	sessionID, err := h.service.OpenSession(r.Context(), user.ID, r.UserAgent(), time.Now().Add(h.jwtExpiration))
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	h.recordLogin(r, user)
	h.announceLogin(r, user.ID, sessionID)
	token, err := authtoken.GenerateWithSession(h.jwtSecret, h.jwtExpiration, user.ID, string(user.Role), sessionID)
	if err != nil {
		httpresponse.Error(w, apperror.Wrap(apperror.KindInternal, "failed to issue token", err))
		return
	}
	h.writeSession(w, r, token, user)
}
