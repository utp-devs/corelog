package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	automationport "github.com/jhonsferg/corelog/internal/automation/application/port"
	automationdomain "github.com/jhonsferg/corelog/internal/automation/domain"
	importport "github.com/jhonsferg/corelog/internal/importexport/application/port"
	issueport "github.com/jhonsferg/corelog/internal/issue/application/port"
	issuedomain "github.com/jhonsferg/corelog/internal/issue/domain"
	notificationport "github.com/jhonsferg/corelog/internal/notification/application/port"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	searchport "github.com/jhonsferg/corelog/internal/search/application/port"
)

// automationActorID is the identity automation actions run as (created by the automation migration).
var automationActorID = uuid.MustParse("00000000-0000-0000-0000-00000000a070")

const (
	automationRole = "user"
	searchPageSize = 100
)

// searchBridge evaluates CoreQL through the search module. It serves automation conditions, webhook filters and
// the CSV export. Automation and webhook checks run as a platform viewer because they decide about an event that
// already happened; the export runs as the exporting user.
type searchBridge struct {
	db     *sqlx.DB
	search searchport.IssueSearchService
}

var _ automationport.Matcher = searchBridge{}

func (b searchBridge) viewer(userID uuid.UUID, role string) searchport.Viewer {
	return searchport.Viewer{UserID: userID, PlatformAdmin: role == "admin"}
}

// Validate checks the syntax of a query.
func (b searchBridge) Validate(ctx context.Context, query string) error {
	rep, err := b.search.Validate(ctx, searchport.Viewer{UserID: automationActorID, PlatformAdmin: true}, query, nil)
	if err != nil {
		return err
	}
	if !rep.Valid {
		return apperror.New(apperror.KindValidation, "consulta no valida")
	}
	return nil
}

// Matches reports whether one issue satisfies the query.
func (b searchBridge) Matches(ctx context.Context, projectID, issueID uuid.UUID, query string) (bool, error) {
	var key string
	err := sqlx.GetContext(ctx, b.db, &key, `SELECT p.key || '-' || i.key_number FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE i.id = $1 AND i.project_id = $2`, issueID, projectID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	out, err := b.search.Search(ctx, searchport.SearchInput{
		Viewer: searchport.Viewer{UserID: automationActorID, PlatformAdmin: true}, Query: fmt.Sprintf(`(%s) AND key = "%s"`, query, key),
		ProjectID: &projectID, Page: 1, PageSize: 1,
	})
	if err != nil {
		return false, err
	}
	return len(out.Page.Items) > 0, nil
}

// Find returns up to limit issue ids of the project that match the query, as the automation identity.
func (b searchBridge) Find(ctx context.Context, projectID uuid.UUID, query string, limit int) ([]uuid.UUID, error) {
	return b.find(ctx, searchport.Viewer{UserID: automationActorID, PlatformAdmin: true}, projectID, query, limit)
}

func (b searchBridge) find(ctx context.Context, viewer searchport.Viewer, projectID uuid.UUID, query string, limit int) ([]uuid.UUID, error) {
	ids := []uuid.UUID{}
	for page := 1; len(ids) < limit; page++ {
		out, err := b.search.Search(ctx, searchport.SearchInput{Viewer: viewer, Query: query, ProjectID: &projectID, Page: page, PageSize: searchPageSize})
		if err != nil {
			return nil, err
		}
		for _, item := range out.Page.Items {
			if len(ids) < limit {
				ids = append(ids, item.ID)
			}
		}
		if len(out.Page.Items) < searchPageSize {
			break
		}
	}
	return ids, nil
}

// exportSearcher adapts the bridge to the export port: the query runs as the exporting user.
type exportSearcher struct{ bridge searchBridge }

var _ importport.Searcher = exportSearcher{}

func (e exportSearcher) Find(ctx context.Context, actorID uuid.UUID, role string, projectID uuid.UUID, query string, limit int) ([]uuid.UUID, error) {
	return e.bridge.find(ctx, e.bridge.viewer(actorID, role), projectID, query, limit)
}

// automationData reads issue snapshots and answers directory questions with SQL.
type automationData struct{ db *sqlx.DB }

var (
	_ automationport.IssueReader = automationData{}
	_ automationport.Directory   = automationData{}
)

// Snapshot reads the facts of an issue.
func (d automationData) Snapshot(ctx context.Context, issueID uuid.UUID) (automationdomain.Snapshot, error) {
	var row struct {
		ID          uuid.UUID  `db:"id"`
		ProjectID   uuid.UUID  `db:"project_id"`
		Key         string     `db:"key"`
		Summary     string     `db:"summary"`
		Status      string     `db:"status"`
		Category    string     `db:"category"`
		Priority    string     `db:"priority"`
		IssueType   string     `db:"issue_type"`
		AssigneeID  *uuid.UUID `db:"assignee_id"`
		ReporterID  uuid.UUID  `db:"reporter_id"`
		ProjectKey  string     `db:"project_key"`
		ProjectName string     `db:"project_name"`
	}
	err := sqlx.GetContext(ctx, d.db, &row, `
		SELECT i.id, i.project_id, p.key || '-' || i.key_number AS key, i.summary,
		       COALESCE(s.name, i.status) AS status, COALESCE(s.category, '') AS category,
		       COALESCE(pr.name, i.priority) AS priority, COALESCE(t.name, i.issue_type) AS issue_type,
		       i.assignee_id, i.reporter_id, p.key AS project_key, p.name AS project_name
		FROM issues i JOIN projects p ON p.id = i.project_id
		LEFT JOIN statuses s ON s.id = i.status_id LEFT JOIN priorities pr ON pr.id = i.priority_id
		LEFT JOIN issue_types t ON t.id = i.issue_type_id
		WHERE i.id = $1 AND i.deleted_at IS NULL`, issueID)
	if err != nil {
		return automationdomain.Snapshot{}, err
	}
	var labels []string
	if err := sqlx.SelectContext(ctx, d.db, &labels, `SELECT l.name FROM issue_labels il JOIN labels l ON l.id = il.label_id
		WHERE il.issue_id = $1 ORDER BY l.name`, issueID); err != nil {
		return automationdomain.Snapshot{}, err
	}
	return automationdomain.Snapshot{IssueID: row.ID, ProjectID: row.ProjectID, Key: row.Key, Summary: row.Summary, Status: row.Status,
		StatusCategory: row.Category, Priority: row.Priority, IssueType: row.IssueType, AssigneeID: row.AssigneeID,
		ReporterID: row.ReporterID, ProjectKey: row.ProjectKey, ProjectName: row.ProjectName, Labels: labels}, nil
}

// ProjectLead returns the lead of a project.
func (d automationData) ProjectLead(ctx context.Context, projectID uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := sqlx.GetContext(ctx, d.db, &id, `SELECT lead_id FROM projects WHERE id = $1`, projectID)
	return id, err
}

// TeamMembers returns the members of a team ordered by user id, so round robin is stable.
func (d automationData) TeamMembers(ctx context.Context, teamID uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := sqlx.SelectContext(ctx, d.db, &ids, `SELECT tm.user_id FROM team_members tm JOIN users u ON u.id = tm.user_id
		WHERE tm.team_id = $1 AND u.deactivated_at IS NULL ORDER BY tm.user_id`, teamID)
	return ids, err
}

// ActorRole returns lead, contributor, viewer or org_admin for a user in a project.
func (d automationData) ActorRole(ctx context.Context, projectID, userID uuid.UUID) (string, error) {
	var lead uuid.UUID
	var org uuid.UUID
	if err := d.db.QueryRowxContext(ctx, `SELECT lead_id, organization_id FROM projects WHERE id = $1`, projectID).Scan(&lead, &org); err != nil {
		return "", err
	}
	if lead == userID {
		return "lead", nil
	}
	var role string
	err := sqlx.GetContext(ctx, d.db, &role, `SELECT role FROM project_members WHERE project_id = $1 AND user_id = $2`, projectID, userID)
	if err == nil {
		return role, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	err = sqlx.GetContext(ctx, d.db, &role, `SELECT role FROM organization_members WHERE organization_id = $1 AND user_id = $2`, org, userID)
	if err == nil && (role == "owner" || role == "admin") {
		return "org_admin", nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	return "", nil
}

// UserName returns the display name of a user.
func (d automationData) UserName(ctx context.Context, userID uuid.UUID) (string, error) {
	var name string
	err := sqlx.GetContext(ctx, d.db, &name, `SELECT display_name FROM users WHERE id = $1`, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return name, err
}

// automationNotifier sends in-app notifications.
type automationNotifier struct{ svc notificationport.Service }

func (n automationNotifier) Notify(ctx context.Context, userID uuid.UUID, kind, message, link string) error {
	if utf8.RuneCountInString(message) > 290 {
		message = string([]rune(message)[:290])
	}
	return n.svc.Notify(ctx, userID, kind, message, link)
}

// automationActions performs the actions of rules through the issue use cases as the automation identity.
type automationActions struct {
	db          *sqlx.DB
	data        automationData
	issues      issueport.Service
	transitions issueport.TransitionService
	comments    issueport.CommentService
}

var _ automationport.IssueActions = automationActions{}

// Transition applies the workflow transition that leads to the named status.
func (a automationActions) Transition(ctx context.Context, issueID uuid.UUID, status string) error {
	if err := a.ensureLead(ctx, issueID); err != nil {
		return err
	}
	options, err := a.transitions.ListTransitions(ctx, issueID, automationActorID, automationRole)
	if err != nil {
		return err
	}
	for _, o := range options {
		if !strings.EqualFold(o.ToStatusName, status) {
			continue
		}
		if o.NeedsScreen() {
			return apperror.New(apperror.KindValidation, "la transicion a "+status+" necesita datos adicionales")
		}
		_, err := a.transitions.TransitionIssue(ctx, issueport.TransitionRequest{
			IssueID: issueID, ActorID: automationActorID, GlobalRole: automationRole, TransitionID: o.ID,
		})
		return err
	}
	return apperror.New(apperror.KindValidation, "no hay una transicion disponible hacia "+status)
}

// Assign sets the assignee.
func (a automationActions) Assign(ctx context.Context, issueID, userID uuid.UUID) error {
	if err := a.ensureLead(ctx, issueID); err != nil {
		return err
	}
	_, err := a.issues.Assign(ctx, issueID, automationActorID, userID, automationRole)
	return err
}

// SetField writes a system or custom field.
func (a automationActions) SetField(ctx context.Context, issueID uuid.UUID, field, value string) error {
	if err := a.ensureLead(ctx, issueID); err != nil {
		return err
	}
	in := issueport.PatchInput{GlobalRole: automationRole}
	switch {
	case field == "summary":
		in.Summary = issueport.Some(value)
	case field == "description":
		in.Description = issueport.Some(value)
	case field == "priority":
		id, err := a.catalogID(ctx, issueID, "priorities", value)
		if err != nil {
			return err
		}
		in.PriorityID = issueport.Some(id)
	case field == "story_points":
		p, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(value), ",", "."), 64)
		if err != nil {
			return apperror.New(apperror.KindValidation, "los puntos de historia no son un numero")
		}
		in.StoryPoints = issueport.Some(p)
	case field == "due_date":
		if strings.TrimSpace(value) == "" {
			in.DueDate = issueport.Null[time.Time]()
		} else {
			t, err := time.Parse("2006-01-02", strings.TrimSpace(value))
			if err != nil {
				return apperror.New(apperror.KindValidation, "la fecha de vencimiento debe ser AAAA-MM-DD")
			}
			in.DueDate = issueport.Some(t)
		}
	case strings.HasPrefix(field, "custom:"):
		raw := json.RawMessage(strconv.Quote(value))
		if json.Valid([]byte(value)) && (value == "true" || value == "false" || value == "null" || isNumber(value)) {
			raw = json.RawMessage(value)
		}
		in.Custom = map[string]json.RawMessage{strings.TrimPrefix(field, "custom:"): raw}
	default:
		return apperror.New(apperror.KindValidation, "campo no editable por automatizacion")
	}
	_, err := a.issues.Patch(ctx, issueID, automationActorID, automationRole, in)
	return err
}

func isNumber(s string) bool { _, err := strconv.ParseFloat(s, 64); return err == nil }

// catalogID resolves a priority by name in the organization of the issue.
func (a automationActions) catalogID(ctx context.Context, issueID uuid.UUID, table, name string) (uuid.UUID, error) {
	if table != "priorities" {
		return uuid.Nil, errors.New("catalogo no admitido")
	}
	var id uuid.UUID
	err := sqlx.GetContext(ctx, a.db, &id, `SELECT c.id FROM priorities c JOIN projects p ON p.organization_id = c.organization_id
		JOIN issues i ON i.project_id = p.id WHERE i.id = $1 AND lower(c.name) = lower($2)`, issueID, strings.TrimSpace(name))
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, apperror.New(apperror.KindValidation, "prioridad desconocida: "+name)
	}
	return id, err
}

// AddComment posts a public comment.
func (a automationActions) AddComment(ctx context.Context, issueID uuid.UUID, body string) error {
	if err := a.ensureLead(ctx, issueID); err != nil {
		return err
	}
	_, err := a.comments.CreateComment(ctx, issueport.CommentActor{ID: automationActorID, GlobalRole: automationRole},
		issueport.CommentCreateInput{IssueID: issueID, Body: body})
	return err
}

// AddLabel adds a label when missing.
func (a automationActions) AddLabel(ctx context.Context, issueID uuid.UUID, label string) error {
	if err := a.ensureLead(ctx, issueID); err != nil {
		return err
	}
	return a.editLabels(ctx, issueID, func(cur []string) []string {
		for _, l := range cur {
			if strings.EqualFold(l, label) {
				return cur
			}
		}
		return append(cur, strings.TrimSpace(label))
	})
}

// RemoveLabel removes a label when present.
func (a automationActions) RemoveLabel(ctx context.Context, issueID uuid.UUID, label string) error {
	if err := a.ensureLead(ctx, issueID); err != nil {
		return err
	}
	return a.editLabels(ctx, issueID, func(cur []string) []string {
		out := make([]string, 0, len(cur))
		for _, l := range cur {
			if !strings.EqualFold(l, label) {
				out = append(out, l)
			}
		}
		return out
	})
}

func (a automationActions) editLabels(ctx context.Context, issueID uuid.UUID, change func([]string) []string) error {
	snap, err := a.data.Snapshot(ctx, issueID)
	if err != nil {
		return err
	}
	next := change(append([]string(nil), snap.Labels...))
	if len(next) == len(snap.Labels) && strings.Join(next, "\x00") == strings.Join(snap.Labels, "\x00") {
		return nil
	}
	_, err = a.issues.Patch(ctx, issueID, automationActorID, automationRole, issueport.PatchInput{GlobalRole: automationRole, Labels: issueport.Some(next)})
	return err
}

// CreateSubtask creates a subtask of the issue and returns its key.
func (a automationActions) CreateSubtask(ctx context.Context, parentID uuid.UUID, summary string) (string, error) {
	if err := a.ensureLead(ctx, parentID); err != nil {
		return "", err
	}
	snap, err := a.data.Snapshot(ctx, parentID)
	if err != nil {
		return "", err
	}
	// Smart values are HTML-escaped; a summary is plain text, so undo it for display.
	summary = html.UnescapeString(summary)
	created, err := a.issues.Create(ctx, issueport.CreateInput{
		Summary: summary, Description: "Subtarea creada por una regla de automatizacion.", ReporterID: automationActorID,
		GlobalRole: automationRole, ProjectID: &snap.ProjectID, IssueType: issuedomain.IssueTypeSubtask, ParentID: &parentID,
	})
	if err != nil {
		return "", err
	}
	return created.Key(), nil
}

// rowImporter creates issues for the CSV import through the issue use cases.
type rowImporter struct {
	db          *sqlx.DB
	issues      issueport.Service
	transitions issueport.TransitionService
}

var _ importport.RowImporter = rowImporter{}

func (r rowImporter) lookup(ctx context.Context, query string, args ...any) (uuid.UUID, bool, error) {
	var id uuid.UUID
	err := sqlx.GetContext(ctx, r.db, &id, query, args...)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	return id, err == nil, err
}

func (r rowImporter) user(ctx context.Context, org uuid.UUID, email string) (uuid.UUID, error) {
	id, ok, err := r.lookup(ctx, `SELECT u.id FROM users u JOIN organization_members m ON m.user_id = u.id AND m.organization_id = $1
		WHERE lower(u.email) = lower($2) AND u.deactivated_at IS NULL`, org, email)
	if err != nil {
		return uuid.Nil, err
	}
	if !ok {
		return uuid.Nil, apperror.New(apperror.KindValidation, "usuario no encontrado en la organizacion: "+email)
	}
	return id, nil
}

func (r rowImporter) catalog(ctx context.Context, table string, org uuid.UUID, name, label string) (*uuid.UUID, error) {
	if name == "" {
		return nil, nil
	}
	var query string
	var aliases catalogAliases
	switch table {
	case "issue_types":
		query = `SELECT id FROM issue_types WHERE organization_id = $1 AND (lower(name) = lower($2) OR legacy_key = $3)`
		aliases = typeAliases
	case "priorities":
		query = `SELECT id FROM priorities WHERE organization_id = $1 AND (lower(name) = lower($2) OR legacy_key = $3)`
		aliases = priorityAliases
	default:
		return nil, errors.New("catalogo no admitido")
	}
	id, ok, err := r.lookup(ctx, query, org, name, aliases.key(name))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, apperror.New(apperror.KindValidation, label+" desconocido: "+name)
	}
	return &id, nil
}

func (r rowImporter) named(ctx context.Context, query string, project uuid.UUID, names []string, label string) ([]uuid.UUID, error) {
	var out []uuid.UUID
	for _, n := range names {
		id, ok, err := r.lookup(ctx, query, project, n)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, apperror.New(apperror.KindValidation, label+" desconocido: "+n)
		}
		out = append(out, id)
	}
	return out, nil
}

// Import validates the references of one row and, unless it is a dry run, creates the issue.
func (r rowImporter) Import(ctx context.Context, in importport.ImportRow) (string, error) {
	row := in.Row
	var org uuid.UUID
	if err := sqlx.GetContext(ctx, r.db, &org, `SELECT organization_id FROM projects WHERE id = $1 AND deleted_at IS NULL`, in.ProjectID); err != nil {
		return "", apperror.New(apperror.KindNotFound, "proyecto no encontrado")
	}
	typeID, err := r.catalog(ctx, "issue_types", org, row.Type, "tipo de incidencia")
	if err != nil {
		return "", err
	}
	priorityID, err := r.catalog(ctx, "priorities", org, row.Priority, "prioridad")
	if err != nil {
		return "", err
	}
	reporter := in.ActorID
	if row.ReporterEmail != "" {
		if reporter, err = r.user(ctx, org, row.ReporterEmail); err != nil {
			return "", err
		}
	}
	var assignee *uuid.UUID
	if row.AssigneeEmail != "" {
		id, err := r.user(ctx, org, row.AssigneeEmail)
		if err != nil {
			return "", err
		}
		assignee = &id
	}
	var parent *uuid.UUID
	if row.ParentKey != "" {
		id, err := r.issues.ResolveKey(ctx, in.ActorID, in.Role, org, row.ParentKey)
		if err != nil {
			return "", apperror.New(apperror.KindValidation, "incidencia padre no encontrada: "+row.ParentKey)
		}
		parent = &id
	}
	components, err := r.named(ctx, `SELECT id FROM components WHERE project_id = $1 AND lower(name) = lower($2)`, in.ProjectID, row.Components, "componente")
	if err != nil {
		return "", err
	}
	versions, err := r.named(ctx, `SELECT id FROM versions WHERE project_id = $1 AND lower(name) = lower($2)`, in.ProjectID, row.FixVersions, "version")
	if err != nil {
		return "", err
	}
	custom := map[string]json.RawMessage{}
	for name, value := range row.Custom {
		id, ok, err := r.lookup(ctx, `SELECT id FROM custom_fields WHERE organization_id = $1 AND lower(name) = lower($2)`, org, name)
		if err != nil {
			return "", err
		}
		if !ok {
			return "", apperror.New(apperror.KindValidation, "campo personalizado desconocido: "+name)
		}
		custom[id.String()] = json.RawMessage(strconv.Quote(value))
	}
	if row.Status != "" {
		_, ok, err := r.lookup(ctx, `SELECT id FROM statuses WHERE organization_id = $1 AND lower(name) = lower($2)`, org, row.Status)
		if err != nil {
			return "", err
		}
		if !ok {
			return "", apperror.New(apperror.KindValidation, "estado desconocido: "+row.Status)
		}
	}
	if in.DryRun {
		return "", nil
	}
	description := row.Description
	if description == "" {
		description = row.Summary
	}
	create := issueport.CreateInput{
		Summary: row.Summary, Description: description, ReporterID: reporter, GlobalRole: in.Role, ProjectID: &in.ProjectID,
		IssueTypeID: typeID, PriorityID: priorityID, ParentID: parent, DueDate: row.DueDate, StartDate: row.StartDate,
		StoryPoints: row.StoryPoints, AssigneeID: assignee, Components: components,
	}
	if len(custom) > 0 {
		create.Custom = custom
	}
	created, err := r.issues.Create(ctx, create)
	if err != nil {
		return "", err
	}
	key := created.Key()
	patch := issueport.PatchInput{GlobalRole: in.Role}
	needPatch := false
	if len(row.Labels) > 0 {
		patch.Labels, needPatch = issueport.Some(row.Labels), true
	}
	if len(versions) > 0 {
		patch.FixVersions, needPatch = issueport.Some(versions), true
	}
	if needPatch {
		if _, err := r.issues.Patch(ctx, created.ID, in.ActorID, in.Role, patch); err != nil {
			return key, fmt.Errorf("creada %s, pero no se aplicaron etiquetas o versiones: %w", key, err)
		}
	}
	if row.Status != "" {
		if err := r.moveToStatus(ctx, created.ID, in, row.Status); err != nil {
			return key, fmt.Errorf("creada %s, pero no se pudo pasar al estado %s: %w", key, row.Status, err)
		}
	}
	return key, nil
}

// moveToStatus applies the transition that leads to the status when it differs from the current one.
func (r rowImporter) moveToStatus(ctx context.Context, issueID uuid.UUID, in importport.ImportRow, status string) error {
	options, err := r.transitions.ListTransitions(ctx, issueID, in.ActorID, in.Role)
	if err != nil {
		return err
	}
	for _, o := range options {
		if strings.EqualFold(o.ToStatusName, status) {
			if o.NeedsScreen() {
				return errors.New("la transicion necesita datos adicionales")
			}
			_, err := r.transitions.TransitionIssue(ctx, issueport.TransitionRequest{IssueID: issueID, ActorID: in.ActorID, GlobalRole: in.Role, TransitionID: o.ID})
			return err
		}
	}
	return nil
}

// catalogAliases maps the English names of Jira exports to the legacy keys of the catalogs.
type catalogAliases map[string]string

func (a catalogAliases) key(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	if k, ok := a[n]; ok {
		return k
	}
	return n
}

var (
	typeAliases     = catalogAliases{"sub-task": "subtask", "subtask": "subtask", "task": "task", "bug": "bug", "story": "story", "epic": "epic"}
	priorityAliases = catalogAliases{"highest": "critical", "blocker": "critical", "critical": "critical", "high": "high", "major": "high",
		"medium": "medium", "low": "low", "minor": "low", "lowest": "low", "trivial": "low"}
)

// ensureLead makes the automation identity a lead of the project of the issue, so the permission checks of the
// issue use cases pass as for any project lead. The membership is created on first use.
func (a automationActions) ensureLead(ctx context.Context, issueID uuid.UUID) error {
	// The identity is created the first time a rule runs, never by a migration. It cannot log in: its password
	// hash is not a bcrypt hash.
	if _, err := a.db.ExecContext(ctx, `INSERT INTO users (id, name, email, password_hash, role, display_name, first_name, last_name)
		VALUES ($1, 'Automatización', 'automation@system.invalid', '!', 'user', 'Automatización', 'Automatización', 'Sistema')
		ON CONFLICT DO NOTHING`, automationActorID); err != nil {
		return err
	}
	if _, err := a.db.ExecContext(ctx, `INSERT INTO organization_members (organization_id, user_id, role)
		SELECT p.organization_id, $2, 'member' FROM issues i JOIN projects p ON p.id = i.project_id WHERE i.id = $1
		ON CONFLICT DO NOTHING`, issueID, automationActorID); err != nil {
		return err
	}
	_, err := a.db.ExecContext(ctx, `INSERT INTO project_members (project_id, user_id, role)
		SELECT project_id, $2, 'lead' FROM issues WHERE id = $1 ON CONFLICT DO NOTHING`, issueID, automationActorID)
	return err
}
