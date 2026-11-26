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

// AvatarRepository is the narrow port the avatar handler needs from the user store.
// SetAvatarKey stores key (nil clears it) and returns the previous key; GetAvatarKey returns nil when
// there is no avatar.
type AvatarRepository interface {
	SetAvatarKey(ctx context.Context, userID uuid.UUID, key *string) (*string, error)
	GetAvatarKey(ctx context.Context, userID uuid.UUID) (*string, error)
}

const (
	avatarCodeInvalid  = "user.avatar_invalid"
	avatarCodeTooLarge = "user.avatar_too_large"
	avatarCodeNotFound = "user.avatar_not_found"
	// avatarMultipartMemory is the in-memory budget of ParseMultipartForm; larger parts spill to disk.
	avatarMultipartMemory = 1 << 20
	// avatarBodyOverhead allows multipart framing around the file itself.
	avatarBodyOverhead = 64 << 10
)

// AvatarHandler serves user avatars: PUT and DELETE /users/me/avatar and GET /users/{id}/avatar.
type AvatarHandler struct {
	repo      AvatarRepository
	files     filestorage.Storage
	jwtSecret string
}

// NewAvatarHandler wires the handler. jwtSecret is used by the Authenticate middleware.
func NewAvatarHandler(repo AvatarRepository, files filestorage.Storage, jwtSecret string) *AvatarHandler {
	return &AvatarHandler{repo: repo, files: files, jwtSecret: jwtSecret}
}

// RegisterRoutes registers the avatar routes on the root router with their full paths. All routes
// require a valid JWT.
func (h *AvatarHandler) RegisterRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))
		r.Put("/users/me/avatar", h.UploadAvatar)
		r.Delete("/users/me/avatar", h.DeleteAvatar)
		r.Get("/users/{id}/avatar", h.GetAvatar)
	})
}

// UploadAvatar replaces the caller's avatar. The multipart field "file" is validated and converted to a
// 256x256 PNG before it is stored under a new random key; the previous file is removed afterwards.
func (h *AvatarHandler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	data, status, err := readAvatarUpload(w, r)
	if status == http.StatusRequestEntityTooLarge {
		avatarWriteCodedError(w, status, avatarCodeTooLarge, "La imagen supera el tamaño máximo de 2 MB")
		return
	}
	if err != nil {
		httpresponse.Error(w, err)
		return
	}

	png, err := imageproc.Process(data)
	if err != nil {
		httpresponse.Error(w, avatarImageError(err))
		return
	}

	key, err := filestorage.NewKey()
	if err != nil {
		httpresponse.Error(w, apperror.Wrap(apperror.KindInternal, "failed to create avatar key", err))
		return
	}
	if err := h.files.Save(r.Context(), key, bytes.NewReader(png)); err != nil {
		httpresponse.Error(w, apperror.Wrap(apperror.KindInternal, "failed to store avatar", err))
		return
	}
	old, err := h.repo.SetAvatarKey(r.Context(), actorID, &key)
	if err != nil {
		avatarDiscard(r.Context(), h.files, key)
		httpresponse.Error(w, err)
		return
	}
	if old != nil {
		avatarDiscard(r.Context(), h.files, *old)
	}
	httpresponse.JSON(w, http.StatusOK, map[string]string{"avatar_url": avatarURL(actorID)})
}

// DeleteAvatar removes the caller's avatar. It is idempotent.
func (h *AvatarHandler) DeleteAvatar(w http.ResponseWriter, r *http.Request) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	old, err := h.repo.SetAvatarKey(r.Context(), actorID, nil)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	if old != nil {
		avatarDiscard(r.Context(), h.files, *old)
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetAvatar streams the PNG avatar of any user. Any authenticated user may read it.
func (h *AvatarHandler) GetAvatar(w http.ResponseWriter, r *http.Request) {
	if _, ok := appmw.UserIDFromContext(r.Context()); !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "identificador de usuario no válido"))
		return
	}
	key, err := h.repo.GetAvatarKey(r.Context(), userID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	if key == nil {
		avatarWriteCodedError(w, http.StatusNotFound, avatarCodeNotFound, "El usuario no tiene avatar")
		return
	}
	avatarServe(w, r, h.files, *key, avatarCodeNotFound)
}

// readAvatarUpload extracts the "file" part and returns its bytes. On failure it returns the status that
// describes the problem: 413 for oversize requests and files, 422 for malformed multipart or a missing file.
func readAvatarUpload(w http.ResponseWriter, r *http.Request) ([]byte, int, error) {
	r.Body = http.MaxBytesReader(w, r.Body, imageproc.MaxUploadBytes+avatarBodyOverhead)
	if err := r.ParseMultipartForm(avatarMultipartMemory); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			return nil, http.StatusRequestEntityTooLarge, nil
		}
		return nil, http.StatusUnprocessableEntity, apperror.WithCode(apperror.KindValidation, avatarCodeInvalid,
			"El formulario multipart no es válido")
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()
	f, _, err := r.FormFile("file")
	if err != nil {
		return nil, http.StatusUnprocessableEntity, apperror.WithCode(apperror.KindValidation, avatarCodeInvalid,
			"El campo file es obligatorio")
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(io.LimitReader(f, imageproc.MaxUploadBytes+1))
	if err != nil {
		return nil, http.StatusInternalServerError, apperror.Wrap(apperror.KindInternal, "failed to read avatar", err)
	}
	if len(data) > imageproc.MaxUploadBytes {
		return nil, http.StatusRequestEntityTooLarge, nil
	}
	return data, http.StatusOK, nil
}

// avatarImageError maps imageproc failures to the public error. The message is in Spanish and never
// exposes decoder details.
func avatarImageError(err error) error {
	switch {
	case errors.Is(err, imageproc.ErrTooLarge):
		return apperror.WithCode(apperror.KindValidation, avatarCodeTooLarge, "La imagen supera el tamaño máximo de 2 MB")
	case errors.Is(err, imageproc.ErrUnsupportedType), errors.Is(err, imageproc.ErrInvalid):
		return apperror.WithCode(apperror.KindValidation, avatarCodeInvalid,
			"El archivo debe ser una imagen PNG, JPEG o WebP válida de hasta 4096 px por lado")
	default:
		return apperror.Wrap(apperror.KindInternal, "failed to process avatar", err)
	}
}

func avatarURL(userID uuid.UUID) string {
	return fmt.Sprintf("/users/%s/avatar", userID)
}

// avatarDiscard removes a file that is no longer referenced. Failures are logged, not returned: the
// database already points at the right key.
func avatarDiscard(ctx context.Context, files filestorage.Storage, key string) {
	if err := files.Delete(context.WithoutCancel(ctx), key); err != nil {
		slog.Warn("failed to remove unreferenced avatar file", "error", err)
	}
}

// avatarServe writes the image stored under key with cache validators. The ETag is derived from the key,
// which changes on every upload, so a 304 is returned when the client already holds that version.
func avatarServe(w http.ResponseWriter, r *http.Request, files filestorage.Storage, key, notFoundCode string) {
	info, err := files.Stat(r.Context(), key)
	if errors.Is(err, filestorage.ErrNotFound) {
		avatarWriteCodedError(w, http.StatusNotFound, notFoundCode, "La imagen no existe")
		return
	}
	if err != nil {
		httpresponse.Error(w, apperror.Wrap(apperror.KindInternal, "failed to stat image", err))
		return
	}
	etag := imageETag(key)
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if avatarETagMatches(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	file, err := files.Open(r.Context(), key)
	if errors.Is(err, filestorage.ErrNotFound) {
		avatarWriteCodedError(w, http.StatusNotFound, notFoundCode, "La imagen no existe")
		return
	}
	if err != nil {
		httpresponse.Error(w, apperror.Wrap(apperror.KindInternal, "failed to open image", err))
		return
	}
	defer func() { _ = file.Close() }()
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size))
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, io.LimitReader(file, info.Size))
}

// imageETag hashes the storage key so the key itself never appears in a response header.
func imageETag(key string) string {
	sum := sha256.Sum256([]byte(key))
	return `"` + hex.EncodeToString(sum[:16]) + `"`
}

// avatarETagMatches implements the If-None-Match comparison for a single strong ETag, including "*"
// and comma-separated lists. Weak prefixes are accepted for comparison.
func avatarETagMatches(header, etag string) bool {
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

// avatarWriteCodedError writes the standard {"error":{message,code}} envelope with a status that
// httpresponse does not map (413).
func avatarWriteCodedError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]map[string]string{
		"error": {"message": message, "code": code},
	})
}
