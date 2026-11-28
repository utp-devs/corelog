package jobs

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
)

// AdminHandler serves GET /admin/jobs for platform administrators.
type AdminHandler struct {
	db        *sqlx.DB
	jwtSecret string
}

// NewAdminHandler returns the handler.
func NewAdminHandler(db *sqlx.DB, jwtSecret string) *AdminHandler {
	return &AdminHandler{db: db, jwtSecret: jwtSecret}
}

// RegisterRoutes mounts the admin routes.
func (h *AdminHandler) RegisterRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))
		r.Use(appmw.RequireRole("admin"))
		r.Get("/admin/jobs", h.List)
	})
}

type lastRunResponse struct {
	Status     string     `json:"status"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
	DurationMS *int64     `json:"duration_ms"`
	Error      string     `json:"error"`
	Result     string     `json:"result"`
}

type jobResponse struct {
	Name          string           `json:"name"`
	Schedule      string           `json:"schedule"`
	RegisteredAt  time.Time        `json:"registered_at"`
	LastSeenAt    time.Time        `json:"last_seen_at"`
	LastRun       *lastRunResponse `json:"last_run"`
	LastSuccessAt *time.Time       `json:"last_success_at"`
}

type jobRow struct {
	Name          string     `db:"name"`
	Schedule      string     `db:"schedule"`
	RegisteredAt  time.Time  `db:"registered_at"`
	LastSeenAt    time.Time  `db:"last_seen_at"`
	RunStatus     *string    `db:"run_status"`
	RunStartedAt  *time.Time `db:"run_started_at"`
	RunFinishedAt *time.Time `db:"run_finished_at"`
	RunDuration   *int64     `db:"run_duration_ms"`
	RunError      *string    `db:"run_error"`
	RunResult     *string    `db:"run_result"`
	LastSuccessAt *time.Time `db:"last_success_at"`
}

// List returns every registered job with its latest run.
func (h *AdminHandler) List(w http.ResponseWriter, r *http.Request) {
	var rows []jobRow
	err := h.db.SelectContext(r.Context(), &rows, `
		SELECT j.name, j.schedule, j.registered_at, j.last_seen_at,
		       lr.status AS run_status, lr.started_at AS run_started_at, lr.finished_at AS run_finished_at,
		       lr.duration_ms AS run_duration_ms, lr.error AS run_error, lr.result AS run_result,
		       (SELECT max(finished_at) FROM scheduler_runs ok WHERE ok.job = j.name AND ok.status = 'ok') AS last_success_at
		FROM scheduler_jobs j
		LEFT JOIN LATERAL (
			SELECT status, started_at, finished_at, duration_ms, error, result
			FROM scheduler_runs WHERE job = j.name ORDER BY id DESC LIMIT 1
		) lr ON true
		ORDER BY j.name`)
	if err != nil {
		httpresponse.Error(w, apperror.Wrap(apperror.KindInternal, "no se pudieron leer los trabajos programados", err))
		return
	}
	out := make([]jobResponse, 0, len(rows))
	for _, row := range rows {
		item := jobResponse{Name: row.Name, Schedule: row.Schedule, RegisteredAt: row.RegisteredAt, LastSeenAt: row.LastSeenAt, LastSuccessAt: row.LastSuccessAt}
		if row.RunStatus != nil && row.RunStartedAt != nil {
			run := &lastRunResponse{Status: *row.RunStatus, StartedAt: *row.RunStartedAt, FinishedAt: row.RunFinishedAt, DurationMS: row.RunDuration}
			if row.RunError != nil {
				run.Error = *row.RunError
			}
			if row.RunResult != nil {
				run.Result = *row.RunResult
			}
			item.LastRun = run
		}
		out = append(out, item)
	}
	httpresponse.JSON(w, http.StatusOK, out)
}
