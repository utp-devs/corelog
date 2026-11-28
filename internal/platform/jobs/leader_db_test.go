package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/authtoken"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
)

func TestOnlyOneInstanceLeadsAndTheOtherTakesOver(t *testing.T) {
	db := testdb.New(t)
	var first, second atomic.Int32
	s1 := NewScheduler(db, discardLogger()).WithLeaderTiming(20*time.Millisecond, 20*time.Millisecond)
	s2 := NewScheduler(db, discardLogger()).WithLeaderTiming(20*time.Millisecond, 20*time.Millisecond)
	s1.Every("tick", 5*time.Millisecond, func(context.Context) error { first.Add(1); return nil })
	s2.Every("tick", 5*time.Millisecond, func(context.Context) error { second.Add(1); return nil })
	ctx := context.Background()

	s1.Start(ctx)
	waitFor(t, 10*time.Second, func() bool { return first.Load() >= 3 })
	s2.Start(ctx)
	time.Sleep(150 * time.Millisecond)
	if second.Load() != 0 {
		t.Fatalf("the follower ran the job %d times while the leader was alive", second.Load())
	}
	if !s1.IsLeader() || s2.IsLeader() {
		t.Fatalf("leader flags: s1=%v s2=%v", s1.IsLeader(), s2.IsLeader())
	}

	s1.Stop()
	if s1.IsLeader() {
		t.Fatal("a stopped scheduler still claims leadership")
	}
	waitFor(t, 10*time.Second, func() bool { return second.Load() >= 3 })
	if !s2.IsLeader() {
		t.Fatal("the follower did not take over")
	}
	s2.Stop()

	var runs int
	if err := db.Get(&runs, `SELECT count(*) FROM scheduler_runs WHERE job = 'tick' AND status = 'ok'`); err != nil || runs < 6 {
		t.Fatalf("recorded ok runs = %d (%v)", runs, err)
	}
	var registered string
	if err := db.Get(&registered, `SELECT schedule FROM scheduler_jobs WHERE name = 'tick'`); err != nil || registered != "every 5ms" {
		t.Fatalf("registered schedule %q (%v)", registered, err)
	}
}

func TestRunsRecordFailuresAndAbandonedRunsAreClosed(t *testing.T) {
	db := testdb.New(t)
	if _, err := db.Exec(`INSERT INTO scheduler_runs (job, started_at, status) VALUES ('old', now() - interval '1 hour', 'running')`); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	s := NewScheduler(db, discardLogger()).WithLeaderTiming(20*time.Millisecond, 20*time.Millisecond)
	s.Every("failing", 5*time.Millisecond, func(context.Context) error {
		calls.Add(1)
		return errors.New("boom")
	})
	if err := s.Register("bad schedule", "whenever", func(context.Context) error { return nil }); err == nil {
		t.Fatal("an invalid schedule was accepted")
	}
	if err := s.Register("daily-job", "daily at 03:00 UTC", func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	s.Start(context.Background())
	waitFor(t, 10*time.Second, func() bool { return calls.Load() >= 2 })
	s.Stop()
	s.Stop() // idempotent

	var status, message string
	if err := db.QueryRow(`SELECT status, error FROM scheduler_runs WHERE job = 'failing' ORDER BY id LIMIT 1`).Scan(&status, &message); err != nil {
		t.Fatal(err)
	}
	if status != "error" || message != "boom" {
		t.Fatalf("failing run recorded as %q %q", status, message)
	}
	var old string
	if err := db.Get(&old, `SELECT status FROM scheduler_runs WHERE job = 'old'`); err != nil || old != "abandoned" {
		t.Fatalf("stale run status %q (%v)", old, err)
	}
	infos := s.Jobs()
	if len(infos) != 2 || infos[0].Name != "daily-job" || infos[0].Schedule != "daily at 03:00 UTC" {
		t.Fatalf("Jobs() = %+v", infos)
	}
}

func TestAdminJobsEndpoint(t *testing.T) {
	db := testdb.New(t)
	if _, err := db.Exec(`INSERT INTO scheduler_jobs (name, schedule) VALUES ('email-outbox', 'every 10s'), ('never-ran', 'every 1h')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO scheduler_runs (job, started_at, finished_at, status, duration_ms) VALUES
		('email-outbox', now() - interval '20 seconds', now() - interval '20 seconds', 'ok', 12),
		('email-outbox', now() - interval '10 seconds', now() - interval '10 seconds', 'error', 3)`); err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	NewAdminHandler(db, "secret").RegisterRoutes(router)
	call := func(role string, authenticated bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/admin/jobs", nil)
		if authenticated {
			token, err := authtoken.Generate("secret", time.Hour, uuid.New(), role)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	if code := call("", false).Code; code != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", code)
	}
	if code := call("requester", true).Code; code != http.StatusForbidden {
		t.Fatalf("non admin: %d", code)
	}
	rec := call("admin", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin: %d %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data []struct {
			Name          string     `json:"name"`
			Schedule      string     `json:"schedule"`
			LastSuccessAt *time.Time `json:"last_success_at"`
			LastRun       *struct {
				Status     string `json:"status"`
				DurationMS *int64 `json:"duration_ms"`
			} `json:"last_run"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data) != 2 || body.Data[0].Name != "email-outbox" || body.Data[1].Name != "never-ran" {
		t.Fatalf("jobs = %+v", body.Data)
	}
	if run := body.Data[0].LastRun; run == nil || run.Status != "error" || body.Data[0].LastSuccessAt == nil {
		t.Fatalf("email-outbox = %+v", body.Data[0])
	}
	if body.Data[1].LastRun != nil || body.Data[1].LastSuccessAt != nil {
		t.Fatalf("never-ran = %+v", body.Data[1])
	}
}

func TestPurgeRunsKeepsRecentAndRunningRows(t *testing.T) {
	db := testdb.New(t)
	if _, err := db.Exec(`INSERT INTO scheduler_runs (job, started_at, status) VALUES
		('a', now() - interval '30 days', 'ok'), ('a', now() - interval '30 days', 'running'), ('a', now(), 'ok')`); err != nil {
		t.Fatal(err)
	}
	removed, err := PurgeRuns(context.Background(), db, time.Now().Add(-14*24*time.Hour))
	if err != nil || removed != 1 {
		t.Fatalf("removed %d (%v)", removed, err)
	}
}
