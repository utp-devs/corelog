package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/sprint/application/port"
	"github.com/jhonsferg/corelog/internal/sprint/domain"
)

const (
	defaultVelocityLimit = 5
	maxVelocityLimit     = 20
	reportVelocityLimit  = 5
)

// SprintService implementa los casos de uso de sprints. Los permisos se consultan al autorizador del
// proyecto: ver el proyecto para leer y el permiso de sprints (o mover tarjetas) para cambiar.
type SprintService struct {
	repo       port.Repository
	authorizer port.Authorizer
	tx         database.TxManager
	clock      clock.Clock
}

func NewSprintService(repo port.Repository, authorizer port.Authorizer, tx database.TxManager, clk clock.Clock) *SprintService {
	return &SprintService{repo: repo, authorizer: authorizer, tx: tx, clock: clk}
}

var _ port.Service = (*SprintService)(nil)

func (s *SprintService) Create(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, name, goal string, start, end *time.Time) (*domain.Sprint, error) {
	if err := s.authorizer.Authorize(ctx, port.ActionManageSprints, actorID, globalRole, projectID); err != nil {
		return nil, err
	}
	sprint, err := domain.NewSprint(s.clock.Now(), projectID, name, goal, start, end)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Save(ctx, sprint); err != nil {
		return nil, err
	}
	return sprint, nil
}

func (s *SprintService) List(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) ([]*domain.Sprint, error) {
	if err := s.authorizer.Authorize(ctx, port.ActionViewProject, actorID, globalRole, projectID); err != nil {
		return nil, err
	}
	return s.repo.ListByProject(ctx, projectID)
}

func (s *SprintService) Get(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, sprintID uuid.UUID) (*domain.Sprint, error) {
	if err := s.authorizer.Authorize(ctx, port.ActionViewProject, actorID, globalRole, projectID); err != nil {
		return nil, err
	}
	return s.findInProject(ctx, projectID, sprintID)
}

func (s *SprintService) ResolveSprintProject(ctx context.Context, sprintID uuid.UUID) (uuid.UUID, error) {
	sprint, err := s.repo.FindByID(ctx, sprintID)
	if err != nil {
		return uuid.Nil, err
	}
	return sprint.ProjectID, nil
}

// Update edita nombre, objetivo y fechas. Un sprint cerrado no cambia.
func (s *SprintService) Update(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, sprintID uuid.UUID, patch domain.Patch) (*domain.Sprint, error) {
	if err := s.authorizer.Authorize(ctx, port.ActionManageSprints, actorID, globalRole, projectID); err != nil {
		return nil, err
	}
	var result *domain.Sprint
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		sprint, err := s.lockInProject(ctx, projectID, sprintID)
		if err != nil {
			return err
		}
		if err := sprint.Edit(s.clock.Now(), patch); err != nil {
			return err
		}
		result = sprint
		return s.repo.Update(ctx, sprint)
	})
	return result, err
}

// Delete borra un sprint planificado; sus incidencias vuelven al backlog.
func (s *SprintService) Delete(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, sprintID uuid.UUID) error {
	if err := s.authorizer.Authorize(ctx, port.ActionManageSprints, actorID, globalRole, projectID); err != nil {
		return err
	}
	return s.tx.InTx(ctx, func(ctx context.Context) error {
		sprint, err := s.lockInProject(ctx, projectID, sprintID)
		if err != nil {
			return err
		}
		if err := sprint.CheckDeletable(); err != nil {
			return err
		}
		return s.repo.Delete(ctx, sprint.ID)
	})
}

// Start activa un sprint planificado y congela el alcance comprometido. Un proyecto solo puede tener un
// sprint activo: la comprobacion y el indice unico del repositorio hacen que de dos inicios concurrentes
// uno gane y el otro reciba 409.
func (s *SprintService) Start(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, sprintID uuid.UUID, input port.StartInput) (*domain.Sprint, error) {
	if err := s.authorizer.Authorize(ctx, port.ActionManageSprints, actorID, globalRole, projectID); err != nil {
		return nil, err
	}
	var result *domain.Sprint
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		sprint, err := s.lockInProject(ctx, projectID, sprintID)
		if err != nil {
			return err
		}
		if sprint.Status != domain.StatusPlanned {
			return domain.ErrSprintNotPlanned
		}
		active, err := s.repo.HasActive(ctx, projectID, sprint.ID)
		if err != nil {
			return err
		}
		if active {
			return domain.ErrActiveSprintExists
		}
		members, err := s.repo.LockMembers(ctx, sprint.ID)
		if err != nil {
			return err
		}
		if len(members) == 0 && !input.Force {
			return domain.ErrSprintEmpty
		}
		now := s.clock.Now()
		if err := sprint.Start(now, domain.StartParams{Name: input.Name, Goal: input.Goal, StartDate: input.StartDate, EndDate: input.EndDate}); err != nil {
			return err
		}
		events, committed := domain.Snapshot(sprint.ID, &actorID, now, members)
		sprint.CommittedPoints = &committed
		if err := s.repo.Update(ctx, sprint); err != nil {
			return err
		}
		if err := s.repo.AddEvents(ctx, events); err != nil {
			return err
		}
		result = sprint
		return nil
	})
	return result, err
}

// Complete cierra el sprint en una sola transaccion: registra completadas e incompletas con los puntos de ese
// momento, mueve las incompletas al destino y fija los puntos comprometidos y completados.
func (s *SprintService) Complete(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, sprintID uuid.UUID, input port.CompleteInput) (*port.CompleteResult, error) {
	if err := s.authorizer.Authorize(ctx, port.ActionManageSprints, actorID, globalRole, projectID); err != nil {
		return nil, err
	}
	raw := input.Destination
	if raw == "" && input.Unfinished == port.UnfinishedToSprint {
		if input.TargetSprintID == nil {
			return nil, domain.ErrInvalidCloseOption
		}
		raw = "sprint:" + input.TargetSprintID.String()
	} else if raw == "" && input.Unfinished != "" && input.Unfinished != port.UnfinishedToBacklog {
		return nil, domain.ErrInvalidCloseOption
	}
	dest, err := domain.ParseDestination(raw)
	if err != nil {
		return nil, err
	}

	var result *port.CompleteResult
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		sprint, err := s.lockInProject(ctx, projectID, sprintID)
		if err != nil {
			return err
		}
		if sprint.Status != domain.StatusActive {
			return domain.ErrSprintNotActive
		}
		now := s.clock.Now()

		var target *domain.Sprint
		switch dest.Kind {
		case domain.DestinationSprint:
			if dest.SprintID == sprint.ID {
				return domain.ErrInvalidCloseOption
			}
			target, err = s.repo.FindByIDForUpdate(ctx, dest.SprintID)
			if errors.Is(err, domain.ErrSprintNotFound) || (err == nil && (target.ProjectID != projectID || target.Status != domain.StatusPlanned)) {
				return domain.ErrInvalidTargetSprint
			}
			if err != nil {
				return err
			}
		case domain.DestinationNew:
			name := input.NewSprintName
			if name == "" {
				name = domain.NextSprintName(sprint.Name)
			}
			target, err = domain.NewSprint(now, projectID, name, "", nil, nil)
			if err != nil {
				return err
			}
			if err := s.repo.Save(ctx, target); err != nil {
				return err
			}
		}

		members, err := s.repo.LockMembers(ctx, sprint.ID)
		if err != nil {
			return err
		}
		plan := domain.PlanCompletion(sprint.ID, &actorID, now, members)
		if err := sprint.Complete(now, plan.CompletedPoints); err != nil {
			return err
		}
		if err := s.repo.Update(ctx, sprint); err != nil {
			return err
		}
		if err := s.repo.AddEvents(ctx, plan.Events); err != nil {
			return err
		}
		var targetID *uuid.UUID
		if target != nil {
			targetID = &target.ID
		}
		if len(plan.Incomplete) > 0 {
			if err := s.repo.MoveIssues(ctx, plan.Incomplete, targetID); err != nil {
				return err
			}
		}
		committed := 0.0
		if sprint.CommittedPoints != nil {
			committed = *sprint.CommittedPoints
		}
		result = &port.CompleteResult{
			Sprint: sprint, NextSprint: target, CompletedIssues: plan.Completed, MovedIssues: len(plan.Incomplete),
			CompletedPoints: plan.CompletedPoints, CommittedPoints: committed,
		}
		return nil
	})
	return result, err
}

// Report calcula el informe del sprint a partir de su historial de alcance.
func (s *SprintService) Report(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, sprintID uuid.UUID) (*port.SprintReport, error) {
	if err := s.authorizer.Authorize(ctx, port.ActionViewProject, actorID, globalRole, projectID); err != nil {
		return nil, err
	}
	sprint, err := s.findInProject(ctx, projectID, sprintID)
	if err != nil {
		return nil, err
	}
	report, err := s.buildReport(ctx, sprint)
	if err != nil {
		return nil, err
	}
	velocity, err := s.repo.Velocity(ctx, projectID, reportVelocityLimit)
	if err != nil {
		return nil, err
	}
	return &port.SprintReport{Report: report, Velocity: velocity}, nil
}

func (s *SprintService) buildReport(ctx context.Context, sprint *domain.Sprint) (*domain.Report, error) {
	events, err := s.repo.Events(ctx, sprint.ID)
	if err != nil {
		return nil, err
	}
	domain.SortEvents(events)
	seen := map[uuid.UUID]bool{}
	var ids []uuid.UUID
	for _, e := range events {
		if !seen[e.IssueID] {
			seen[e.IssueID] = true
			ids = append(ids, e.IssueID)
		}
	}
	categories, err := s.repo.LiveCategories(ctx, ids)
	if err != nil {
		return nil, err
	}
	timelines, err := s.repo.DoneTimelines(ctx, ids)
	if err != nil {
		return nil, err
	}
	return domain.BuildReport(sprint, events, categories, timelines, s.clock.Now()), nil
}

func (s *SprintService) ListIssues(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, sprintID uuid.UUID) ([]port.IssueRef, error) {
	if err := s.authorizer.Authorize(ctx, port.ActionViewProject, actorID, globalRole, projectID); err != nil {
		return nil, err
	}
	if _, err := s.findInProject(ctx, projectID, sprintID); err != nil {
		return nil, err
	}
	return s.repo.ListIssues(ctx, sprintID)
}

// AssignIssue pone una incidencia del proyecto en el sprint. Un sprint cerrado ya no cambia.
func (s *SprintService) AssignIssue(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, sprintID, issueID uuid.UUID) error {
	if err := s.authorizer.Authorize(ctx, port.ActionManageSprints, actorID, globalRole, projectID); err != nil {
		return err
	}
	return s.tx.InTx(ctx, func(ctx context.Context) error {
		sprint, err := s.findInProject(ctx, projectID, sprintID)
		if err != nil {
			return err
		}
		if sprint.Status == domain.StatusClosed {
			return domain.ErrSprintClosed
		}
		states, err := s.repo.LockIssues(ctx, projectID, []uuid.UUID{issueID})
		if err != nil {
			return err
		}
		if len(states) != 1 {
			return domain.ErrIssueNotInProject
		}
		return s.relocate(ctx, projectID, actorID, states, sprint)
	})
}

// RemoveIssue quita una incidencia del sprint, que pasa a no tener sprint (backlog).
func (s *SprintService) RemoveIssue(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, sprintID, issueID uuid.UUID) error {
	if err := s.authorizer.Authorize(ctx, port.ActionManageSprints, actorID, globalRole, projectID); err != nil {
		return err
	}
	return s.tx.InTx(ctx, func(ctx context.Context) error {
		sprint, err := s.findInProject(ctx, projectID, sprintID)
		if err != nil {
			return err
		}
		if sprint.Status == domain.StatusClosed {
			return domain.ErrSprintClosed
		}
		states, err := s.repo.LockIssues(ctx, projectID, []uuid.UUID{issueID})
		if err != nil {
			return err
		}
		if len(states) != 1 || states[0].SprintID == nil || *states[0].SprintID != sprintID {
			return domain.ErrIssueNotInProject
		}
		return s.relocate(ctx, projectID, actorID, states, nil)
	})
}

// relocate pone las incidencias en el sprint de destino (nil = backlog) y registra los eventos de alcance
// de los sprints activos implicados. Las que ya estan en el destino no cambian.
func (s *SprintService) relocate(ctx context.Context, projectID, actorID uuid.UUID, states []port.IssueState, target *domain.Sprint) error {
	var targetID *uuid.UUID
	if target != nil {
		targetID = &target.ID
	}
	now := s.clock.Now()
	cache := map[uuid.UUID]*domain.Sprint{}
	var events []domain.Event
	for _, st := range states {
		if sameSprint(st.SprintID, targetID) {
			continue
		}
		var fromActive, toActive *uuid.UUID
		if st.SprintID != nil {
			from, err := s.cachedSprint(ctx, cache, *st.SprintID)
			if err != nil {
				return err
			}
			if from.Status == domain.StatusClosed {
				return domain.ErrSprintClosed
			}
			if from.Status == domain.StatusActive {
				fromActive = &from.ID
			}
		}
		if target != nil && target.Status == domain.StatusActive {
			toActive = &target.ID
		}
		leaving := domain.EventRemoved
		if target != nil {
			leaving = domain.EventMovedOut
		}
		events = append(events, domain.MoveEvents(st.ID, st.Points, st.Category, &actorID, now, fromActive, toActive, leaving)...)
		if err := s.repo.SetIssueSprint(ctx, projectID, st.ID, targetID); err != nil {
			return err
		}
	}
	return s.repo.AddEvents(ctx, events)
}

// Burndown calcula, dia a dia, el trabajo restante frente a la linea ideal que baja hasta cero, a partir
// del historial de alcance. En un sprint activo la serie llega hasta hoy.
func (s *SprintService) Burndown(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, sprintID uuid.UUID) ([]port.BurndownPoint, error) {
	if err := s.authorizer.Authorize(ctx, port.ActionViewProject, actorID, globalRole, projectID); err != nil {
		return nil, err
	}
	sprint, err := s.findInProject(ctx, projectID, sprintID)
	if err != nil {
		return nil, err
	}
	if sprint.StartDate == nil || sprint.EndDate == nil {
		return nil, domain.ErrSprintWithoutDates
	}
	report, err := s.buildReport(ctx, sprint)
	if err != nil {
		return nil, err
	}
	points := make([]port.BurndownPoint, 0, len(report.Burndown))
	for _, d := range report.Burndown {
		ideal := 0.0
		if d.Ideal != nil {
			ideal = *d.Ideal
		}
		points = append(points, port.BurndownPoint{Date: d.Date, Ideal: ideal, Remaining: d.Remaining})
	}
	return points, nil
}

// Velocity devuelve los puntos completados de los ultimos sprints cerrados del proyecto.
func (s *SprintService) Velocity(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, limit int) ([]port.VelocityPoint, error) {
	if err := s.authorizer.Authorize(ctx, port.ActionViewProject, actorID, globalRole, projectID); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = defaultVelocityLimit
	}
	if limit > maxVelocityLimit {
		limit = maxVelocityLimit
	}
	return s.repo.Velocity(ctx, projectID, limit)
}

// Backlog devuelve las secciones del proyecto (sprint activo, planificados y backlog) con totales y epicas.
func (s *SprintService) Backlog(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, filter port.BacklogFilter) (*port.Backlog, error) {
	if err := s.authorizer.Authorize(ctx, port.ActionViewProject, actorID, globalRole, projectID); err != nil {
		return nil, err
	}
	data, err := s.repo.Backlog(ctx, projectID, filter)
	if err != nil {
		return nil, err
	}
	bySprint := map[uuid.UUID]*port.BacklogSection{}
	var active, planned []*port.BacklogSection
	for _, sp := range data.Sprints {
		section := &port.BacklogSection{Sprint: sp, Kind: string(sp.Status), Issues: []port.BacklogIssue{}}
		bySprint[sp.ID] = section
		if sp.Status == domain.StatusActive {
			active = append(active, section)
		} else {
			planned = append(planned, section)
		}
	}
	backlog := &port.BacklogSection{Kind: "backlog", Issues: []port.BacklogIssue{}}
	for _, issue := range data.Issues {
		section := backlog
		if issue.SprintID != nil {
			if found, ok := bySprint[*issue.SprintID]; ok {
				section = found
			}
		}
		section.Issues = append(section.Issues, issue)
	}
	out := &port.Backlog{ProjectID: projectID, Epics: data.Epics, Truncated: data.Truncated}
	for _, group := range [][]*port.BacklogSection{active, planned, {backlog}} {
		for _, section := range group {
			section.Totals = totalsOf(section.Issues)
			out.Sections = append(out.Sections, *section)
		}
	}
	out.CanManageSprint = s.authorizer.Authorize(ctx, port.ActionManageSprints, actorID, globalRole, projectID) == nil
	out.CanMoveCards = s.authorizer.Authorize(ctx, port.ActionMoveCards, actorID, globalRole, projectID) == nil
	return out, nil
}

func totalsOf(issues []port.BacklogIssue) port.SectionTotals {
	t := port.SectionTotals{ByCategory: map[string]port.CategoryTotals{
		domain.CategoryTodo: {}, domain.CategoryInProgress: {}, domain.CategoryDone: {},
	}}
	for _, i := range issues {
		points := 0.0
		if i.StoryPoints != nil {
			points = *i.StoryPoints
		}
		var estimate int64
		if i.OriginalEstimateSeconds != nil {
			estimate = *i.OriginalEstimateSeconds
		}
		t.Issues++
		t.Points += points
		t.EstimateSeconds += estimate
		c := t.ByCategory[i.StatusCategory]
		c.Issues++
		c.Points += points
		c.EstimateSeconds += estimate
		t.ByCategory[i.StatusCategory] = c
	}
	return t
}

// Rank mueve un conjunto de incidencias a una seccion (sprint o backlog) antes o despues de una ancla, en una
// sola transaccion que actualiza sprint, rank y el historial de los sprints activos. Reordenar o pasar de
// backlog a sprint activo (y viceversa) solo pide mover tarjetas; tocar un sprint planificado pide ademas
// gestionar sprints.
func (s *SprintService) Rank(ctx context.Context, actorID uuid.UUID, globalRole string, input port.RankInput) (uuid.UUID, error) {
	ids := dedupe(input.IssueIDs)
	if len(ids) == 0 {
		return uuid.Nil, domain.ErrNoIssues
	}
	if len(ids) > domain.MaxMoveIssues {
		return uuid.Nil, domain.ErrTooManyIssues
	}
	projectID := uuid.Nil
	if input.ProjectID != nil {
		projectID = *input.ProjectID
	} else {
		owners, err := s.repo.IssueProjects(ctx, ids)
		if err != nil {
			return uuid.Nil, err
		}
		for _, id := range ids {
			p, ok := owners[id]
			if !ok || (projectID != uuid.Nil && p != projectID) {
				return uuid.Nil, domain.ErrIssueNotInProject
			}
			projectID = p
		}
	}
	if err := s.authorizer.Authorize(ctx, port.ActionMoveCards, actorID, globalRole, projectID); err != nil {
		return uuid.Nil, err
	}

	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		states, err := s.repo.LockIssues(ctx, projectID, ids)
		if err != nil {
			return err
		}
		if len(states) != len(ids) {
			return domain.ErrIssueNotInProject
		}
		cache := map[uuid.UUID]*domain.Sprint{}
		var target *domain.Sprint
		if input.TargetSprintID != nil {
			target, err = s.cachedSprint(ctx, cache, *input.TargetSprintID)
			if err != nil {
				return err
			}
			if target.ProjectID != projectID {
				return domain.ErrSprintNotFound
			}
			if target.Status == domain.StatusClosed {
				return domain.ErrSprintClosed
			}
		}
		needsManage := false
		for _, st := range states {
			if sameSprint(st.SprintID, input.TargetSprintID) {
				continue
			}
			if st.SprintID != nil {
				from, err := s.cachedSprint(ctx, cache, *st.SprintID)
				if err != nil {
					return err
				}
				if from.Status == domain.StatusClosed {
					return domain.ErrSprintClosed
				}
				needsManage = needsManage || from.Status == domain.StatusPlanned
			}
			needsManage = needsManage || (target != nil && target.Status == domain.StatusPlanned)
		}
		if needsManage {
			if err := s.authorizer.Authorize(ctx, port.ActionManageSprints, actorID, globalRole, projectID); err != nil {
				return err
			}
		}

		existing, err := s.repo.SectionRanks(ctx, projectID, input.TargetSprintID, ids)
		if err != nil {
			return err
		}
		pos, err := domain.PositionFor(existing, input.BeforeIssueID, input.AfterIssueID)
		if err != nil {
			return err
		}
		ranks, _ := domain.PlanSection(existing, ids, pos)

		now := s.clock.Now()
		var events []domain.Event
		byID := map[uuid.UUID]port.IssueState{}
		for _, st := range states {
			byID[st.ID] = st
		}
		for _, id := range ids {
			st := byID[id]
			if sameSprint(st.SprintID, input.TargetSprintID) {
				continue
			}
			var fromActive, toActive *uuid.UUID
			if st.SprintID != nil && cache[*st.SprintID].Status == domain.StatusActive {
				fromActive = st.SprintID
			}
			if target != nil && target.Status == domain.StatusActive {
				toActive = &target.ID
			}
			leaving := domain.EventRemoved
			if target != nil {
				leaving = domain.EventMovedOut
			}
			events = append(events, domain.MoveEvents(st.ID, st.Points, st.Category, &actorID, now, fromActive, toActive, leaving)...)
		}
		if err := s.repo.PlaceIssues(ctx, projectID, input.TargetSprintID, ranks); err != nil {
			return err
		}
		return s.repo.AddEvents(ctx, events)
	})
	return projectID, err
}

func (s *SprintService) cachedSprint(ctx context.Context, cache map[uuid.UUID]*domain.Sprint, id uuid.UUID) (*domain.Sprint, error) {
	if sp, ok := cache[id]; ok {
		return sp, nil
	}
	sp, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	cache[id] = sp
	return sp, nil
}

func sameSprint(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func dedupe(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]bool, len(ids))
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// findInProject carga un sprint y comprueba que pertenece al proyecto; si no, no existe para el llamador.
func (s *SprintService) findInProject(ctx context.Context, projectID, sprintID uuid.UUID) (*domain.Sprint, error) {
	sprint, err := s.repo.FindByID(ctx, sprintID)
	if err != nil {
		return nil, err
	}
	if sprint.ProjectID != projectID {
		return nil, domain.ErrSprintNotFound
	}
	return sprint, nil
}

func (s *SprintService) lockInProject(ctx context.Context, projectID, sprintID uuid.UUID) (*domain.Sprint, error) {
	sprint, err := s.repo.FindByIDForUpdate(ctx, sprintID)
	if err != nil {
		return nil, err
	}
	if sprint.ProjectID != projectID {
		return nil, domain.ErrSprintNotFound
	}
	return sprint, nil
}
