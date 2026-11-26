package http

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/authtoken"
	"github.com/jhonsferg/corelog/internal/platform/filestorage"
)

const avatarTestSecret = "avatar-test-secret"

type fakeAvatarRepo struct {
	keys map[uuid.UUID]string
}

func (r *fakeAvatarRepo) SetAvatarKey(_ context.Context, userID uuid.UUID, key *string) (*string, error) {
	old, had := r.keys[userID]
	if key == nil {
		delete(r.keys, userID)
	} else {
		r.keys[userID] = *key
	}
	if !had {
		return nil, nil
	}
	return &old, nil
}

func (r *fakeAvatarRepo) GetAvatarKey(_ context.Context, userID uuid.UUID) (*string, error) {
	k, ok := r.keys[userID]
	if !ok {
		return nil, nil
	}
	return &k, nil
}

type avatarEnv struct {
	router *chi.Mux
	repo   *fakeAvatarRepo
	files  *filestorage.DiskStorage
	dir    string
	owner  uuid.UUID
	token  string
}

func newAvatarEnv(t *testing.T) *avatarEnv {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "avatars")
	owner := uuid.New()
	token, err := authtoken.Generate(avatarTestSecret, time.Hour, owner, "user")
	if err != nil {
		t.Fatal(err)
	}
	env := &avatarEnv{
		repo:  &fakeAvatarRepo{keys: map[uuid.UUID]string{}},
		files: filestorage.NewDiskStorage(dir),
		dir:   dir,
		owner: owner,
		token: token,
	}
	env.router = chi.NewRouter()
	NewAvatarHandler(env.repo, env.files, avatarTestSecret).RegisterRoutes(env.router)
	return env
}

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		img.Set(x, 0, color.RGBA{255, 0, 0, 255})
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// multipartBody builds a multipart body with one part. contentType is the declared part type.
func multipartBody(t *testing.T, field, filename, contentType string, data []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	h := make(map[string][]string)
	h["Content-Disposition"] = []string{`form-data; name="` + field + `"; filename="` + filename + `"`}
	if contentType != "" {
		h["Content-Type"] = []string{contentType}
	}
	part, err := mw.CreatePart(h)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf, mw.FormDataContentType()
}

func (env *avatarEnv) do(t *testing.T, method, path, token string, body *bytes.Buffer, contentType string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var rd *bytes.Buffer
	if body == nil {
		rd = &bytes.Buffer{}
	} else {
		rd = body
	}
	req := httptest.NewRequest(method, path, rd)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)
	return rec
}

func (env *avatarEnv) upload(t *testing.T, data []byte, declaredType string) *httptest.ResponseRecorder {
	t.Helper()
	body, ct := multipartBody(t, "file", "avatar.bin", declaredType, data)
	return env.do(t, http.MethodPut, "/users/me/avatar", env.token, body, ct, nil)
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode error body %q: %v", rec.Body.String(), err)
	}
	return env.Error.Code
}

func TestAvatarRoutesRequireAuthentication(t *testing.T) {
	env := newAvatarEnv(t)
	id := uuid.New().String()
	cases := []struct{ method, path string }{
		{http.MethodPut, "/users/me/avatar"},
		{http.MethodDelete, "/users/me/avatar"},
		{http.MethodGet, "/users/" + id + "/avatar"},
	}
	for _, c := range cases {
		rec := env.do(t, c.method, c.path, "", nil, "", nil)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without token = %d, want 401", c.method, c.path, rec.Code)
		}
	}
}

func TestAvatarUploadSuccessAndReplace(t *testing.T) {
	env := newAvatarEnv(t)

	rec := env.upload(t, pngBytes(t, 640, 480), "image/png")
	if rec.Code != http.StatusOK {
		t.Fatalf("upload = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Data struct {
			AvatarURL string `json:"avatar_url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Data.AvatarURL != "/users/"+env.owner.String()+"/avatar" {
		t.Fatalf("avatar_url = %q", resp.Data.AvatarURL)
	}
	first := env.repo.keys[env.owner]
	if first == "" {
		t.Fatal("avatar key not stored")
	}
	info, err := env.files.Stat(context.Background(), first)
	if err != nil || info.Size == 0 {
		t.Fatalf("stored file missing: %v", err)
	}

	rec = env.upload(t, pngBytes(t, 100, 300), "image/png")
	if rec.Code != http.StatusOK {
		t.Fatalf("second upload = %d", rec.Code)
	}
	second := env.repo.keys[env.owner]
	if second == first {
		t.Fatal("replacement must use a new key")
	}
	if _, err := env.files.Stat(context.Background(), first); err == nil {
		t.Fatal("old avatar file must be removed after replace")
	}
	if _, err := env.files.Stat(context.Background(), second); err != nil {
		t.Fatalf("new avatar file missing: %v", err)
	}
}

func TestAvatarUploadSniffsContentNotHeader(t *testing.T) {
	env := newAvatarEnv(t)

	// A real PNG declared as text/plain is accepted: the bytes decide.
	if rec := env.upload(t, pngBytes(t, 50, 50), "text/plain"); rec.Code != http.StatusOK {
		t.Fatalf("valid PNG with lying header = %d", rec.Code)
	}
	// Plain text declared as image/png is rejected: the declared type is ignored in favour of the bytes.
	rec := env.upload(t, []byte("<html>no soy una imagen</html>"), "image/png")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("text with image header = %d, want 422", rec.Code)
	}
	if code := errorCode(t, rec); code != "user.avatar_invalid" {
		t.Fatalf("code = %q, want user.avatar_invalid", code)
	}
}

func TestAvatarUploadRejections(t *testing.T) {
	env := newAvatarEnv(t)

	t.Run("wrong type 422", func(t *testing.T) {
		rec := env.upload(t, []byte("GIF89a......"), "image/gif")
		if rec.Code != http.StatusUnprocessableEntity || errorCode(t, rec) != "user.avatar_invalid" {
			t.Fatalf("got %d %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("dimension bomb 422", func(t *testing.T) {
		rec := env.upload(t, pngBytes(t, 5000, 1), "image/png")
		if rec.Code != http.StatusUnprocessableEntity || errorCode(t, rec) != "user.avatar_invalid" {
			t.Fatalf("got %d %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("truncated 422", func(t *testing.T) {
		data := pngBytes(t, 64, 64)
		rec := env.upload(t, data[:len(data)-12], "image/png")
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("got %d", rec.Code)
		}
	})
	t.Run("oversize 413", func(t *testing.T) {
		rec := env.upload(t, make([]byte, 2<<20+1), "image/png")
		if rec.Code != http.StatusRequestEntityTooLarge || errorCode(t, rec) != "user.avatar_too_large" {
			t.Fatalf("got %d %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("missing file field 422", func(t *testing.T) {
		body, ct := multipartBody(t, "other", "a.png", "image/png", pngBytes(t, 10, 10))
		rec := env.do(t, http.MethodPut, "/users/me/avatar", env.token, body, ct, nil)
		if rec.Code != http.StatusUnprocessableEntity || errorCode(t, rec) != "user.avatar_invalid" {
			t.Fatalf("got %d %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("not multipart 422", func(t *testing.T) {
		rec := env.do(t, http.MethodPut, "/users/me/avatar", env.token, bytes.NewBufferString("{}"), "application/json", nil)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("got %d", rec.Code)
		}
	})
	if _, ok := env.repo.keys[env.owner]; ok {
		t.Fatal("rejected uploads must not change the avatar")
	}
}

func TestAvatarDelete(t *testing.T) {
	env := newAvatarEnv(t)
	if rec := env.upload(t, pngBytes(t, 40, 40), "image/png"); rec.Code != http.StatusOK {
		t.Fatalf("upload = %d", rec.Code)
	}
	key := env.repo.keys[env.owner]

	rec := env.do(t, http.MethodDelete, "/users/me/avatar", env.token, nil, "", nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete = %d", rec.Code)
	}
	if _, ok := env.repo.keys[env.owner]; ok {
		t.Fatal("avatar key must be cleared")
	}
	if _, err := env.files.Stat(context.Background(), key); err == nil {
		t.Fatal("file must be removed")
	}
	// Deleting again is idempotent.
	if rec := env.do(t, http.MethodDelete, "/users/me/avatar", env.token, nil, "", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("second delete = %d", rec.Code)
	}
}

func TestAvatarGetHeadersAndConditional(t *testing.T) {
	env := newAvatarEnv(t)
	if rec := env.upload(t, pngBytes(t, 90, 60), "image/png"); rec.Code != http.StatusOK {
		t.Fatalf("upload = %d", rec.Code)
	}
	path := "/users/" + env.owner.String() + "/avatar"

	// Any logged-in user can read another user's avatar.
	otherToken, err := authtoken.Generate(avatarTestSecret, time.Hour, uuid.New(), "user")
	if err != nil {
		t.Fatal(err)
	}
	rec := env.do(t, http.MethodGet, path, otherToken, nil, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("Content-Type = %q", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "private, max-age=3600" {
		t.Errorf("Cache-Control = %q", cc)
	}
	if xcto := rec.Header().Get("X-Content-Type-Options"); xcto != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q", xcto)
	}
	etag := rec.Header().Get("ETag")
	if etag == "" || strings.Contains(etag, env.repo.keys[env.owner]) {
		t.Fatalf("ETag = %q must be set and must not expose the storage key", etag)
	}
	if _, err := png.Decode(rec.Body); err != nil {
		t.Fatalf("body is not a PNG: %v", err)
	}

	rec = env.do(t, http.MethodGet, path, otherToken, nil, "", map[string]string{"If-None-Match": etag})
	if rec.Code != http.StatusNotModified {
		t.Fatalf("conditional get = %d, want 304", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatal("304 must have no body")
	}

	rec = env.do(t, http.MethodGet, path, otherToken, nil, "", map[string]string{"If-None-Match": `"stale"`})
	if rec.Code != http.StatusOK {
		t.Fatalf("stale etag get = %d, want 200", rec.Code)
	}
}

func TestAvatarGetNotFoundAndInvalidID(t *testing.T) {
	env := newAvatarEnv(t)
	rec := env.do(t, http.MethodGet, "/users/"+env.owner.String()+"/avatar", env.token, nil, "", nil)
	if rec.Code != http.StatusNotFound || errorCode(t, rec) != "user.avatar_not_found" {
		t.Fatalf("no avatar = %d %s", rec.Code, rec.Body.String())
	}
	rec = env.do(t, http.MethodGet, "/users/not-a-uuid/avatar", env.token, nil, "", nil)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid id = %d", rec.Code)
	}
}

func TestAvatarGetMissingFileIsNotFound(t *testing.T) {
	env := newAvatarEnv(t)
	key, err := filestorage.NewKey()
	if err != nil {
		t.Fatal(err)
	}
	env.repo.keys[env.owner] = key // key recorded in the database, file never written
	rec := env.do(t, http.MethodGet, "/users/"+env.owner.String()+"/avatar", env.token, nil, "", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("dangling key = %d, want 404", rec.Code)
	}
}

func TestAvatarStorageDirectoryCreatedOnUpload(t *testing.T) {
	env := newAvatarEnv(t)
	if _, err := os.Stat(env.dir); !os.IsNotExist(err) {
		t.Fatalf("directory should not exist before the first upload: %v", err)
	}
	if rec := env.upload(t, pngBytes(t, 8, 8), "image/png"); rec.Code != http.StatusOK {
		t.Fatalf("upload = %d", rec.Code)
	}
	if _, err := os.Stat(env.dir); err != nil {
		t.Fatalf("directory not created: %v", err)
	}
}

// TestAvatarRoutesCoexistWithUserRoutes guards the wiring: the avatar routes use full paths on the root
// router and must not clash with the existing /users mount, in either registration order.
func TestAvatarRoutesCoexistWithUserRoutes(t *testing.T) {
	env := newAvatarEnv(t)
	userHandler := NewHandler(nil, avatarTestSecret, time.Hour)

	for name, order := range map[string]func(r chi.Router){
		"avatar first": func(r chi.Router) {
			NewAvatarHandler(env.repo, env.files, avatarTestSecret).RegisterRoutes(r)
			userHandler.RegisterRoutes(r)
		},
		"user first": func(r chi.Router) {
			userHandler.RegisterRoutes(r)
			NewAvatarHandler(env.repo, env.files, avatarTestSecret).RegisterRoutes(r)
		},
	} {
		t.Run(name, func(t *testing.T) {
			root := chi.NewRouter()
			order(root)
			req := httptest.NewRequest(http.MethodGet, "/users/"+env.owner.String()+"/avatar", nil)
			req.Header.Set("Authorization", "Bearer "+env.token)
			rec := httptest.NewRecorder()
			root.ServeHTTP(rec, req)
			if rec.Code != http.StatusNotFound || errorCode(t, rec) != "user.avatar_not_found" {
				t.Fatalf("avatar route not reached: %d %s", rec.Code, rec.Body.String())
			}
		})
	}
}
