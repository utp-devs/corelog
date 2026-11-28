package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/jhonsferg/corelog/internal/platform/authtoken"
	"github.com/jhonsferg/corelog/internal/platform/config"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
	userpostgres "github.com/jhonsferg/corelog/internal/user/adapter/out/postgres"
)

const streamTestSecret = "stream-e2e-secret"

type liveStream struct {
	t     *testing.T
	lines chan string
}

func openLiveStream(t *testing.T, base, token string) *liveStream {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/v1/notifications/stream", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stream status %d", resp.StatusCode)
	}
	s := &liveStream{t: t, lines: make(chan string, 100)}
	go func() {
		defer close(s.lines)
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			s.lines <- sc.Text()
		}
	}()
	if l := s.next(); l != "retry: 3000" {
		t.Fatalf("first line %q", l)
	}
	return s
}

func (s *liveStream) next() string {
	s.t.Helper()
	select {
	case l, ok := <-s.lines:
		if !ok {
			return "<closed>"
		}
		return l
	case <-time.After(10 * time.Second):
		s.t.Fatal("timed out waiting for a stream line")
		return ""
	}
}

// until returns the first line that has the prefix, skipping blanks and other lines.
func (s *liveStream) until(prefix string) string {
	s.t.Helper()
	for {
		l := s.next()
		if l == "<closed>" {
			s.t.Fatalf("stream closed before %q", prefix)
		}
		if strings.HasPrefix(l, prefix) {
			return l
		}
	}
}

func (s *liveStream) quiet() bool {
	select {
	case l := <-s.lines:
		return l == "" || strings.HasPrefix(l, ":")
	case <-time.After(400 * time.Millisecond):
		return true
	}
}

func TestNewLoginAlertReachesOtherSessionsThroughTheStream(t *testing.T) {
	t.Setenv("ATTACHMENTS_DIR", t.TempDir())
	t.Setenv("AVATARS_DIR", t.TempDir())
	t.Setenv("LOGOS_DIR", t.TempDir())
	db := testdb.New(t)
	cfg := &config.Config{Auth: config.AuthConfig{JWTSecret: streamTestSecret, JWTExpiration: time.Hour}}
	router, err := buildRouter(cfg, db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	appmw.SetSessionValidator(userpostgres.NewSessionRepository(db).Validate)
	t.Cleanup(func() { appmw.SetSessionValidator(nil) })
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	hash, err := bcrypt.GenerateFromPassword([]byte("Aa1!aaaaaaaa"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	user := uuid.New()
	if _, err := db.Exec(`INSERT INTO users (id, name, display_name, email, password_hash, role, email_verified_at) VALUES ($1, 'Ana', 'Ana', 'ana@example.test', $2, 'user', now())`, user, string(hash)); err != nil {
		t.Fatal(err)
	}

	login := func(agent string) (token string, session uuid.UUID) {
		t.Helper()
		body, _ := json.Marshal(map[string]string{"email": "ana@example.test", "password": "Aa1!aaaaaaaa"})
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", agent)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		raw, _ := io.ReadAll(resp.Body)
		var out struct {
			Data struct {
				Token string `json:"token"`
			} `json:"data"`
		}
		if resp.StatusCode != http.StatusOK || json.Unmarshal(raw, &out) != nil || out.Data.Token == "" {
			t.Fatalf("login: %d %s", resp.StatusCode, raw)
		}
		claims, err := authtoken.Parse(streamTestSecret, out.Data.Token)
		if err != nil {
			t.Fatal(err)
		}
		return out.Data.Token, claims.SessionID()
	}

	const chromeWindows = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36"
	const firefoxLinux = "Mozilla/5.0 (X11; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0"

	// The very first login of the account has nothing to compare with: no alert.
	tokenA, _ := login(chromeWindows)
	streamA := openLiveStream(t, srv.URL, tokenA)

	// A login from a new device (the incognito window) alerts the open session, not itself.
	tokenB, sessionB := login(firefoxLinux)
	streamA.until("event: notification")
	data := streamA.until("data: ")
	var note map[string]any
	if err := json.Unmarshal([]byte(strings.TrimPrefix(data, "data: ")), &note); err != nil {
		t.Fatal(err)
	}
	if note["kind"] != "security.new_login" || note["message"] != nil && !strings.Contains(note["message"].(string), "Firefox en Linux (IP 127.0.0.x)") {
		t.Fatalf("notification: %v", note)
	}
	extra, _ := note["data"].(map[string]any)
	if extra["session_id"] != sessionB.String() || extra["title"] != "Nuevo inicio de sesión" {
		t.Fatalf("alert data: %v", extra)
	}
	streamA.until("event: session")
	if line := streamA.until("data: "); !strings.Contains(line, `"type":"new_login"`) {
		t.Fatalf("session event: %s", line)
	}

	// The alert is persisted for the next time the app opens.
	var stored int
	if err := db.Get(&stored, `SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND kind = 'security.new_login'`, user); err != nil || stored != 1 {
		t.Fatalf("stored alerts: %d %v", stored, err)
	}

	// The same device logging in again within 30 days raises no new alert.
	streamB := openLiveStream(t, srv.URL, tokenB)
	login(firefoxLinux)
	if !streamA.quiet() || !streamB.quiet() {
		t.Fatal("a known device must not alert")
	}

	// "No fui yo": revoking the new session from the first one closes its live stream.
	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/users/me/sessions/"+sessionB.String(), nil)
	req.Header.Set("Authorization", "Bearer "+tokenA)
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusNoContent {
		t.Fatalf("revoke: %v %v", resp, err)
	}
	_ = resp.Body.Close()
	if line := streamB.until("data: "); !strings.Contains(line, `"type":"session_revoked"`) {
		t.Fatalf("revoked event: %s", line)
	}
}
