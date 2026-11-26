package http

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/filestorage"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	"github.com/jhonsferg/corelog/internal/platform/imageproc"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
)

// LogoRepository is the narrow port the logo handler needs from the organization store. SetLogoKey stores
// key (nil clears it) and returns the previous key; GetLogoKey returns nil when there is no logo.
type LogoRepository interface {
	SetLogoKey(ctx context.Context, orgID uuid.UUID, key *string) (*string, error)
	GetLogoKey(ctx context.Context, orgID uuid.UUID) (*string, error)
}

// OrgRoleChecker is the narrow port the logo handler needs from the organization service. It returns nil
// when userID has at least the role min (one of "owner", "admin", "member", "guest"). Non-members must get
// an apperror of kind not_found, members without enough rank an apperror of kind forbidden.
type OrgRoleChecker interface {
	RequireRole(ctx context.Context, userID, orgID uuid.UUID, min string) error
}

const (
	logoCodeInvalid  = "org.logo_invalid"
	logoCodeTooLarge = "org.logo_too_large"
	logoCodeNotFound = "org.logo_not_found"
	// logoMultipartMemory is the in-memory budget of ParseMultipartForm; larger parts spill to disk.
	logoMultipartMemory = 1 << 20
	// logoBodyOverhead allows multipart framing around the file itself.
	logoBodyOverhead = 64 << 10
)

// LogoHandler serves organization logos: PUT and DELETE /organizations/{org}/logo (owner and admin) and
// GET /organizations/{org}/logo (any member).
type LogoHandler struct {
	repo      LogoRepository
	files     filestorage.Storage
	roles     OrgRoleChecker
	jwtSecret string
}

// NewLogoHandler wires the handler. jwtSecret is used by the Authenticate middleware.
func NewLogoHandler(repo LogoRepository, files filestorage.Storage, roles OrgRoleChecker, jwtSecret string) *LogoHandler {
	return &LogoHandler{repo: repo, files: files, roles: roles, jwtSecret: jwtSecret}
}

// RegisterRoutes registers the logo routes on the root router with their full paths. All routes require
// a valid JWT.
func (h *LogoHandler) RegisterRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))
		r.Put("/organizations/{org}/logo", h.UploadLogo)
		r.Delete("/organizations/{org}/logo", h.DeleteLogo)
		r.Get("/organizations/{org}/logo", h.GetLogo)
	})
}

// UploadLogo replaces the logo of an organization. The caller must be an owner or admin. The multipart field
// "file" is validated and converted to a 256x256 PNG before it is stored under a new random key.
func (h *LogoHandler) UploadLogo(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.authorize(w, r, "admin")
	if !ok {
		return
	}
	data, status, err := readLogoUpload(w, r)
	if status == http.StatusRequestEntityTooLarge {
		logoWriteCodedError(w, status, logoCodeTooLarge, "La imagen supera el tamaño máximo de 2 MB")
		return
	}
	if err != nil {
		httpresponse.Error(w, err)
		return
	}

	png, err := imageproc.Process(data)
	if err != nil {
		httpresponse.Error(w, logoImageError(err))
		return
	}

	key, err := filestorage.NewKey()
	if err != nil {
		httpresponse.Error(w, apperror.Wrap(apperror.KindInternal, "failed to create logo key", err))
		return
	}
	if err := h.files.Save(r.Context(), key, bytes.NewReader(png)); err != nil {
		httpresponse.Error(w, apperror.Wrap(apperror.KindInternal, "failed to store logo", err))
		return
	}
	old, err := h.repo.SetLogoKey(r.Context(), orgID, &key)
	if err != nil {
		logoDiscard(r.Context(), h.files, key)
		httpresponse.Error(w, err)
		return
	}
	if old != nil {
		logoDiscard(r.Context(), h.files, *old)
	}
	httpresponse.JSON(w, http.StatusOK, map[string]string{"logo_url": logoURL(orgID)})
}

// DeleteLogo removes the logo of an organization. The caller must be an owner or admin. It is idempotent.
func (h *LogoHandler) DeleteLogo(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.authorize(w, r, "admin")
	if !ok {
		return
	}
	old, err := h.repo.SetLogoKey(r.Context(), orgID, nil)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	if old != nil {
		logoDiscard(r.Context(), h.files, *old)
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetLogo streams the PNG logo of an organization. Any member may read it; non-members get 404.
func (h *LogoHandler) GetLogo(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.authorize(w, r, "guest")
	if !ok {
		return
	}
	key, err := h.repo.GetLogoKey(r.Context(), orgID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	if key == nil {
		logoWriteCodedError(w, http.StatusNotFound, logoCodeNotFound, "La organización no tiene logo")
		return
	}
	logoServe(w, r, h.files, *key)
}

// authorize resolves the caller and the organization, then delegates the rank check. It writes the error
// response and returns ok=false when the request must stop.
func (h *LogoHandler) authorize(w http.ResponseWriter, r *http.Request, minRole string) (uuid.UUID, bool) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return uuid.Nil, false
	}
	orgID, err := uuid.Parse(chi.URLParam(r, "org"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "identificador de organización no válido"))
		return uuid.Nil, false
	}
	if err := h.roles.RequireRole(r.Context(), actorID, orgID, minRole); err != nil {
		httpresponse.Error(w, err)
		return uuid.Nil, false
	}
	return orgID, true
}

// readLogoUpload extracts the "file" part and returns its bytes. It returns status 413 for oversize requests
// and files, 422 for malformed multipart or a missing file.
func readLogoUpload(w http.ResponseWriter, r *http.Request) ([]byte, int, error) {
	r.Body = http.MaxBytesReader(w, r.Body, imageproc.MaxUploadBytes+logoBodyOverhead)
	if err := r.ParseMultipartForm(logoMultipartMemory); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			return nil, http.StatusRequestEntityTooLarge, nil
		}
		return nil, http.StatusUnprocessableEntity, apperror.WithCode(apperror.KindValidation, logoCodeInvalid,
			"El formulario multipart no es válido")
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()
	f, _, err := r.FormFile("file")
	if err != nil {
		return nil, http.StatusUnprocessableEntity, apperror.WithCode(apperror.KindValidation, logoCodeInvalid,
			"El campo file es obligatorio")
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(io.LimitReader(f, imageproc.MaxUploadBytes+1))
	if err != nil {
		return nil, http.StatusInternalServerError, apperror.Wrap(apperror.KindInternal, "failed to read logo", err)
	}
	if len(data) > imageproc.MaxUploadBytes {
		return nil, http.StatusRequestEntityTooLarge, nil
	}
	return data, http.StatusOK, nil
}

// logoImageError maps imageproc failures to the public error. Messages are in Spanish and never expose
// decoder details.
func logoImageError(err error) error {
	switch {
	case errors.Is(err, imageproc.ErrTooLarge):
		return apperror.WithCode(apperror.KindValidation, logoCodeTooLarge, "La imagen supera el tamaño máximo de 2 MB")
	case errors.Is(err, imageproc.ErrUnsupportedType), errors.Is(err, imageproc.ErrInvalid):
		return apperror.WithCode(apperror.KindValidation, logoCodeInvalid,
			"El archivo debe ser una imagen PNG, JPEG o WebP válida de hasta 4096 px por lado")
	default:
		return apperror.Wrap(apperror.KindInternal, "failed to process logo", err)
	}
}

func logoURL(orgID uuid.UUID) string {
	return fmt.Sprintf("/organizations/%s/logo", orgID)
}

// logoDiscard removes a file that is no longer referenced. Failures are logged, not returned.
func logoDiscard(ctx context.Context, files filestorage.Storage, key string) {
	if err := files.Delete(context.WithoutCancel(ctx), key); err != nil {
		slog.Warn("failed to remove unreferenced logo file", "error", err)
	}
}

// logoServe writes the image stored under key with cache validators. The ETag is derived from the key,
// which changes on every upload, so a 304 is returned when the client already holds that version.
func logoServe(w http.ResponseWriter, r *http.Request, files filestorage.Storage, key string) {
	info, err := files.Stat(r.Context(), key)
	if errors.Is(err, filestorage.ErrNotFound) {
		logoWriteCodedError(w, http.StatusNotFound, logoCodeNotFound, "La imagen no existe")
		return
	}
	if err != nil {
		httpresponse.Error(w, apperror.Wrap(apperror.KindInternal, "failed to stat logo", err))
		return
	}
	etag := logoETag(key)
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if logoETagMatches(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	file, err := files.Open(r.Context(), key)
	if errors.Is(err, filestorage.ErrNotFound) {
		logoWriteCodedError(w, http.StatusNotFound, logoCodeNotFound, "La imagen no existe")
		return
	}
	if err != nil {
		httpresponse.Error(w, apperror.Wrap(apperror.KindInternal, "failed to open logo", err))
		return
	}
	defer func() { _ = file.Close() }()
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size))
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, io.LimitReader(file, info.Size))
}

// logoETag hashes the storage key so the key itself never appears in a response header.
func logoETag(key string) string {
	sum := sha256.Sum256([]byte(key))
	return `"` + hex.EncodeToString(sum[:16]) + `"`
}

// logoETagMatches implements the If-None-Match comparison for one strong ETag, including "*" and lists.
func logoETagMatches(header, etag string) bool {
	if header == "" {
		return false
	}
	for _, candidate := range strings.Split(header, ",") {
		c := strings.TrimSpace(candidate)
		if c == "*" || strings.TrimPrefix(c, "W/") == etag {
			return true
		}
	}
	return false
}

// logoWriteCodedError writes the standard {"error":{message,code}} envelope with a status that
// httpresponse does not map (413).
func logoWriteCodedError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]map[string]string{
		"error": {"message": message, "code": code},
	})
}
