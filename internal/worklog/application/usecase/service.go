// Package usecase implements the worklog in-port: authorization, validation, transactions and activity.
package usecase

import (
	"context"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/events"
	"github.com/jhonsferg/corelog/internal/worklog/application/port"
	"github.com/jhonsferg/corelog/internal/worklog/domain"
)

// Event and activity names written by this module.
const (
	EventWorklogged     = "issue.worklogged"
	actionWorklogAdded  = "worklog_added"
	actionWorklogChange = "worklog_updated"
	actionWorklogRemove = "worklog_deleted"
	actionFieldChanged  = "field_changed"
	fieldTimeSpent      = "time_spent_seconds"
	fieldRemaining      = "remaining_estimate_seconds"
	fieldOriginal       = "original_estimate_seconds"
	maxRangeDays        = 366
	// codeProjectArchived is the archive error code of the authorizer (same value as the project module).
	codeProjectArchived = "project.archived"
)

// Service implements port.Service.
type Service struct {
	repo       port.Repository
	authorizer port.Authorizer
	tx         database.TxManager
	events     events.Publisher
	clock      clock.Clock
}

var _ port.Service = (*Service)(nil)

// NewService wires the use cases. events may be nil; then no event is published.
func NewService(repo port.Repository, authorizer port.Authorizer, tx database.TxManager, pub events.Publisher, clk clock.Clock) *Service {
	return &Service{repo: repo, authorizer: authorizer, tx: tx, events: pub, clock: clk}
}

// authorized reports whether err from the authorizer lets the operation go on. An archive error means the
// caller holds the permission, so it counts as allowed for reads that filter rows.
func authorized(err error) bool {
	return err == nil || apperror.CodeOf(err) == codeProjectArchived
}

func (s *Service) orgTime(ctx context.Context, orgID uuid.UUID) (domain.OrgTime, error) {
	times, err := s.repo.OrgTimes(ctx, []uuid.UUID{orgID})
	if err != nil {
		return domain.OrgTime{}, err
	}
	if z, ok := times[orgID]; ok {
		return z, nil
	}
	return domain.OrgTime{Location: time.UTC, Config: domain.DefaultConfig()}, nil
}

func (s *Service) ListIssueWorklogs(ctx context.Context, actorID uuid.UUID, globalRole string, issueID uuid.UUID) (*port.IssueWorklogs, error) {
	it, err := s.repo.IssueTime(ctx, issueID, false)
	if err != nil {
		return nil, err
	}
	if err := s.authorizer.Authorize(ctx, port.ActionViewProject, actorID, globalRole, it.ProjectID); err != nil {
		return nil, err
	}
	list, err := s.repo.ListIssueWorklogs(ctx, issueID)
	if err != nil {
		return nil, err
	}
	zone, err := s.orgTime(ctx, it.OrganizationID)
	if err != nil {
		return nil, err
	}
	return &port.IssueWorklogs{Worklogs: list, Totals: totals(it), Config: zone.Config}, nil
}

func totals(it port.IssueTime) port.Totals {
	return port.Totals{OriginalSeconds: it.Original, RemainingSeconds: it.Remaining, SpentSeconds: it.Spent}
}

// durationSeconds resolves a Duration. Clear returns nil. zeroOK allows zero (estimates). The text form uses
// the organization config; the seconds form is checked against the same limits.
func durationSeconds(d port.Duration, cfg domain.Config, zeroOK bool) (*int64, error) {
	if d.Clear {
		return nil, nil
	}
	var v int64
	switch {
	case d.Text != nil && d.Seconds != nil, d.Text == nil && d.Seconds == nil:
		return nil, domain.ErrDurationRequired
	case d.Text != nil:
		parse := domain.ParseDuration
		if zeroOK {
			parse = domain.ParseEstimate
		}
		parsed, err := parse(*d.Text, cfg)
		if err != nil {
			return nil, err
		}
		v = parsed
	default:
		v = *d.Seconds
		if v < 0 || (!zeroOK && v == 0) {
			return nil, domain.ErrDurationRange
		}
		limit := domain.MaxEstimateSeconds
		if !zeroOK {
			limit = domain.MaxWorklogSeconds
		}
		if v > limit {
			return nil, domain.ErrDurationRange
		}
	}
	return &v, nil
}

func (s *Service) LogWork(ctx context.Context, actorID uuid.UUID, globalRole string, issueID uuid.UUID, in port.LogInput) (*port.Logged, error) {
	it, err := s.repo.IssueTime(ctx, issueID, false)
	if err != nil {
		return nil, err
	}
	if err := s.authorizer.Authorize(ctx, port.ActionLogWork, actorID, globalRole, it.ProjectID); err != nil {
		return nil, err
	}
	mode := in.Remaining.Mode
	if mode == "" {
		mode = domain.RemainingAuto
	}
	if !domain.ValidRemainingMode(mode) {
		return nil, apperror.WithCode(apperror.KindValidation, domain.CodeModeInvalid, "El modo de ajuste de la estimacion restante no es valido")
	}
	if mode != domain.RemainingLeave {
		if err := s.authorizer.Authorize(ctx, port.ActionEditIssue, actorID, globalRole, it.ProjectID); err != nil {
			return nil, err
		}
	}
	zone, err := s.orgTime(ctx, it.OrganizationID)
	if err != nil {
		return nil, err
	}
	seconds, err := durationSeconds(in.TimeSpent, zone.Config, false)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	startedAt := now
	if in.StartedAt != nil {
		startedAt = in.StartedAt.UTC()
	}
	if err := domain.ValidateEntry(*seconds, startedAt, now, in.Description); err != nil {
		return nil, err
	}
	change := domain.RemainingChange{Mode: mode}
	if mode == domain.RemainingSet || mode == domain.RemainingReduce {
		if in.Remaining.Value == nil {
			return nil, apperror.WithCode(apperror.KindValidation, domain.CodeDurationRequired, "Indica la estimacion restante para el modo elegido")
		}
		value, err := durationSeconds(*in.Remaining.Value, zone.Config, mode == domain.RemainingSet)
		if err != nil {
			return nil, err
		}
		if value != nil {
			change.Value = *value
		}
	}

	var result *port.Logged
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		locked, err := s.repo.IssueTime(ctx, issueID, true)
		if err != nil {
			return err
		}
		next, adjust, err := domain.ApplyRemaining(locked.Remaining, *seconds, change)
		if err != nil {
			return err
		}
		w := domain.Worklog{
			IssueID: issueID, ProjectID: locked.ProjectID, OrganizationID: locked.OrganizationID,
			AuthorID: actorID, StartedAt: startedAt, TimeSpentSeconds: *seconds, Description: in.Description,
			RemainingMode: mode, RemainingAdjustSeconds: adjust,
		}
		if err := s.repo.InsertWorklog(ctx, &w); err != nil {
			return err
		}
		if err := s.repo.RecomputeSpent(ctx, issueID); err != nil {
			return err
		}
		batch := uuid.New()
		rows := []port.Activity{activity(issueID, actorID, now, batch, actionWorklogAdded, fieldTimeSpent, nil, intText(seconds))}
		if !sameValue(locked.Remaining, next) {
			if err := s.repo.SetRemaining(ctx, issueID, next); err != nil {
				return err
			}
			rows = append(rows, activity(issueID, actorID, now, batch, actionFieldChanged, fieldRemaining, intText(locked.Remaining), intText(next)))
		}
		if err := s.repo.AddActivities(ctx, rows); err != nil {
			return err
		}
		if err := s.publish(ctx, EventWorklogged, locked, actorID, map[string]any{"worklog_id": w.ID, "time_spent_seconds": *seconds}); err != nil {
			return err
		}
		after, err := s.repo.IssueTime(ctx, issueID, false)
		if err != nil {
			return err
		}
		w.IssueKey, w.IssueTitle = after.IssueKey, after.IssueTitle
		result = &port.Logged{Worklog: &w, Totals: totals(after), Config: zone.Config}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) UpdateWorklog(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID, in port.UpdateInput) (*port.Logged, error) {
	w, err := s.repo.Worklog(ctx, id, false)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeWorklog(ctx, actorID, globalRole, w); err != nil {
		return nil, err
	}
	zone, err := s.orgTime(ctx, w.OrganizationID)
	if err != nil {
		return nil, err
	}
	seconds := w.TimeSpentSeconds
	timeChanged := false
	if in.TimeSpent != nil {
		v, err := durationSeconds(*in.TimeSpent, zone.Config, false)
		if err != nil {
			return nil, err
		}
		if *v != w.TimeSpentSeconds {
			seconds, timeChanged = *v, true
		}
	}
	startedAt := w.StartedAt
	if in.StartedAt != nil {
		startedAt = in.StartedAt.UTC()
	}
	description := w.Description
	if in.Description != nil {
		description = *in.Description
	}
	now := s.clock.Now()
	if err := domain.ValidateEntry(seconds, startedAt, now, description); err != nil {
		return nil, err
	}

	var result *port.Logged
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		locked, err := s.repo.IssueTime(ctx, w.IssueID, true)
		if err != nil {
			return err
		}
		remaining := locked.Remaining
		adjust := w.RemainingAdjustSeconds
		remainingChanged := false
		if timeChanged && w.RemainingMode == domain.RemainingAuto {
			restored := domain.RestoreRemaining(remaining, w.RemainingAdjustSeconds)
			next, newAdjust, err := domain.ApplyRemaining(restored, seconds, domain.RemainingChange{Mode: domain.RemainingAuto})
			if err != nil {
				return err
			}
			remaining, adjust = next, newAdjust
			remainingChanged = !sameValue(locked.Remaining, remaining)
		}
		updated := w
		updated.TimeSpentSeconds = seconds
		updated.StartedAt = startedAt
		updated.Description = description
		updated.RemainingAdjustSeconds = adjust
		if err := s.repo.UpdateWorklog(ctx, &updated); err != nil {
			return err
		}
		if err := s.repo.RecomputeSpent(ctx, w.IssueID); err != nil {
			return err
		}
		batch := uuid.New()
		var rows []port.Activity
		if timeChanged {
			rows = append(rows, activity(w.IssueID, actorID, now, batch, actionWorklogChange, fieldTimeSpent, intText(&w.TimeSpentSeconds), intText(&seconds)))
		}
		if remainingChanged {
			if err := s.repo.SetRemaining(ctx, w.IssueID, remaining); err != nil {
				return err
			}
			rows = append(rows, activity(w.IssueID, actorID, now, batch, actionFieldChanged, fieldRemaining, intText(locked.Remaining), intText(remaining)))
		}
		if len(rows) > 0 {
			if err := s.repo.AddActivities(ctx, rows); err != nil {
				return err
			}
		}
		after, err := s.repo.IssueTime(ctx, w.IssueID, false)
		if err != nil {
			return err
		}
		updated.IssueKey, updated.IssueTitle, updated.AuthorName = w.IssueKey, w.IssueTitle, w.AuthorName
		result = &port.Logged{Worklog: &updated, Totals: totals(after), Config: zone.Config}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) DeleteWorklog(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID) error {
	w, err := s.repo.Worklog(ctx, id, false)
	if err != nil {
		return err
	}
	if err := s.authorizeWorklog(ctx, actorID, globalRole, w); err != nil {
		return err
	}
	now := s.clock.Now()
	return s.tx.InTx(ctx, func(ctx context.Context) error {
		locked, err := s.repo.IssueTime(ctx, w.IssueID, true)
		if err != nil {
			return err
		}
		remaining := domain.RestoreRemaining(locked.Remaining, w.RemainingAdjustSeconds)
		if err := s.repo.SoftDeleteWorklog(ctx, id, now); err != nil {
			return err
		}
		if err := s.repo.RecomputeSpent(ctx, w.IssueID); err != nil {
			return err
		}
		batch := uuid.New()
		rows := []port.Activity{activity(w.IssueID, actorID, now, batch, actionWorklogRemove, fieldTimeSpent, intText(&w.TimeSpentSeconds), nil)}
		if !sameValue(locked.Remaining, remaining) {
			if err := s.repo.SetRemaining(ctx, w.IssueID, remaining); err != nil {
				return err
			}
			rows = append(rows, activity(w.IssueID, actorID, now, batch, actionFieldChanged, fieldRemaining, intText(locked.Remaining), intText(remaining)))
		}
		return s.repo.AddActivities(ctx, rows)
	})
}

// authorizeWorklog lets the author change their own worklog with log_work and anybody else's with
// manage_worklogs (project leads and organization owners and admins).
func (s *Service) authorizeWorklog(ctx context.Context, actorID uuid.UUID, globalRole string, w domain.Worklog) error {
	action := port.ActionManageWorklog
	if w.AuthorID == actorID {
		action = port.ActionLogWork
	}
	return s.authorizer.Authorize(ctx, action, actorID, globalRole, w.ProjectID)
}

func (s *Service) UpdateEstimates(ctx context.Context, actorID uuid.UUID, globalRole string, issueID uuid.UUID, in port.EstimatesInput) (*port.Estimates, error) {
	it, err := s.repo.IssueTime(ctx, issueID, false)
	if err != nil {
		return nil, err
	}
	if err := s.authorizer.Authorize(ctx, port.ActionEditIssue, actorID, globalRole, it.ProjectID); err != nil {
		return nil, err
	}
	zone, err := s.orgTime(ctx, it.OrganizationID)
	if err != nil {
		return nil, err
	}
	var original, remaining *int64
	setOriginal, setRemaining := in.Original != nil, in.Remaining != nil
	if setOriginal {
		if original, err = durationSeconds(*in.Original, zone.Config, true); err != nil {
			return nil, err
		}
	}
	if setRemaining {
		if remaining, err = durationSeconds(*in.Remaining, zone.Config, true); err != nil {
			return nil, err
		}
	}
	now := s.clock.Now()
	var result *port.Estimates
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		locked, err := s.repo.IssueTime(ctx, issueID, true)
		if err != nil {
			return err
		}
		batch := uuid.New()
		var rows []port.Activity
		if setOriginal && !sameValue(locked.Original, original) {
			if err := s.repo.SetOriginal(ctx, issueID, original); err != nil {
				return err
			}
			rows = append(rows, activity(issueID, actorID, now, batch, actionFieldChanged, fieldOriginal, intText(locked.Original), intText(original)))
		}
		if setRemaining && !sameValue(locked.Remaining, remaining) {
			if err := s.repo.SetRemaining(ctx, issueID, remaining); err != nil {
				return err
			}
			rows = append(rows, activity(issueID, actorID, now, batch, actionFieldChanged, fieldRemaining, intText(locked.Remaining), intText(remaining)))
		}
		if len(rows) > 0 {
			if err := s.repo.AddActivities(ctx, rows); err != nil {
				return err
			}
		}
		after, err := s.repo.IssueTime(ctx, issueID, false)
		if err != nil {
			return err
		}
		result = &port.Estimates{Totals: totals(after), Config: zone.Config}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) Timesheet(ctx context.Context, actorID uuid.UUID, globalRole string, q port.TimesheetQuery) (*domain.Timesheet, domain.Config, error) {
	from, to, err := parseRange(q.From, q.To)
	if err != nil {
		return nil, domain.Config{}, err
	}
	target := actorID
	own := true
	if q.UserID != nil && *q.UserID != actorID {
		target, own = *q.UserID, false
	}
	lo, hi, err := domain.DayWindow(from, to)
	if err != nil {
		return nil, domain.Config{}, domain.ErrRangeInvalid
	}
	rows, err := s.repo.Window(ctx, port.Window{Lo: lo, Hi: hi, AuthorID: &target})
	if err != nil {
		return nil, domain.Config{}, err
	}
	// Visibility: the caller sees their own entries in projects they can browse, and somebody else's
	// entries only in projects where they manage worklogs (leads and organization admins).
	action := port.ActionViewProject
	if !own {
		action = port.ActionManageWorklog
	}
	allowed := map[uuid.UUID]bool{}
	visible := rows[:0]
	for _, row := range rows {
		ok, seen := allowed[row.ProjectID]
		if !seen {
			ok, err = s.projectAllowed(ctx, action, actorID, globalRole, row.ProjectID)
			if err != nil {
				return nil, domain.Config{}, err
			}
			allowed[row.ProjectID] = ok
		}
		if ok {
			visible = append(visible, row)
		}
	}
	zones, err := s.zonesFor(ctx, visible)
	if err != nil {
		return nil, domain.Config{}, err
	}
	ts := domain.BuildTimesheet(visible, zones, from, to)
	return &ts, firstConfig(zones, visible), nil
}

func (s *Service) projectAllowed(ctx context.Context, action string, actorID uuid.UUID, globalRole string, projectID uuid.UUID) (bool, error) {
	err := s.authorizer.Authorize(ctx, action, actorID, globalRole, projectID)
	if authorized(err) {
		return true, nil
	}
	if apperror.KindOf(err) == apperror.KindForbidden || apperror.KindOf(err) == apperror.KindNotFound {
		return false, nil
	}
	return false, err
}

func (s *Service) TimeSummary(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, q port.SummaryQuery) (*domain.Summary, domain.Config, error) {
	if !domain.ValidGroupBy(q.GroupBy) {
		return nil, domain.Config{}, domain.ErrGroupByInvalid
	}
	from, to, err := parseRange(q.From, q.To)
	if err != nil {
		return nil, domain.Config{}, err
	}
	if err := s.authorizer.Authorize(ctx, port.ActionViewReports, actorID, globalRole, projectID); err != nil {
		return nil, domain.Config{}, err
	}
	lo, hi, err := domain.DayWindow(from, to)
	if err != nil {
		return nil, domain.Config{}, domain.ErrRangeInvalid
	}
	rows, err := s.repo.Window(ctx, port.Window{Lo: lo, Hi: hi, ProjectID: &projectID})
	if err != nil {
		return nil, domain.Config{}, err
	}
	zones, err := s.zonesFor(ctx, rows)
	if err != nil {
		return nil, domain.Config{}, err
	}
	sum := domain.BuildSummary(rows, zones, q.GroupBy, from, to)
	return &sum, firstConfig(zones, rows), nil
}

func (s *Service) zonesFor(ctx context.Context, rows []domain.Entry) (map[uuid.UUID]domain.OrgTime, error) {
	seen := map[uuid.UUID]bool{}
	var ids []uuid.UUID
	for _, r := range rows {
		if !seen[r.OrganizationID] {
			seen[r.OrganizationID] = true
			ids = append(ids, r.OrganizationID)
		}
	}
	if len(ids) == 0 {
		return map[uuid.UUID]domain.OrgTime{}, nil
	}
	return s.repo.OrgTimes(ctx, ids)
}

func firstConfig(zones map[uuid.UUID]domain.OrgTime, rows []domain.Entry) domain.Config {
	if len(rows) > 0 {
		if z, ok := zones[rows[0].OrganizationID]; ok {
			return z.Config
		}
	}
	return domain.DefaultConfig()
}

// parseRange validates the inclusive date range: both dates in YYYY-MM-DD, from not after to, and at most
// maxRangeDays days.
func parseRange(from, to string) (string, string, error) {
	start, err := time.Parse("2006-01-02", from)
	if err != nil {
		return "", "", domain.ErrRangeInvalid
	}
	end, err := time.Parse("2006-01-02", to)
	if err != nil || end.Before(start) || end.Sub(start) > time.Duration(maxRangeDays-1)*24*time.Hour {
		return "", "", domain.ErrRangeInvalid
	}
	return from, to, nil
}

func (s *Service) publish(ctx context.Context, typ string, it port.IssueTime, actorID uuid.UUID, payload map[string]any) error {
	if s.events == nil {
		return nil
	}
	org, project, issue, actor := it.OrganizationID, it.ProjectID, it.IssueID, actorID
	return s.events.Publish(ctx, events.Event{
		OrgID: &org, ProjectID: &project, EntityID: &issue, ActorID: &actor,
		Type: typ, EntityType: "issue", Payload: payload, CreatedAt: s.clock.Now(),
	})
}

func activity(issueID, actorID uuid.UUID, at time.Time, batch uuid.UUID, action, field string, oldValue, newValue *string) port.Activity {
	return port.Activity{IssueID: issueID, ActorID: actorID, Action: action, Field: field, OldValue: oldValue, NewValue: newValue, At: at, BatchID: batch}
}

func intText(v *int64) *string {
	if v == nil {
		return nil
	}
	s := strconv.FormatInt(*v, 10)
	return &s
}

func sameValue(a, b *int64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
