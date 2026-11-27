package main

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	userdomain "github.com/jhonsferg/corelog/internal/user/domain"
)

// defaultOrgID is the fixed id of the organization "corelog" that migration 000024
// creates. Seeders reuse it so the default organization keeps the same id.
var defaultOrgID = uuid.MustParse("00000000-0000-4000-8000-000000000002")

// person is a platform account. platform is the platform role: "admin" or "user".
type person struct {
	name, email, platform string
}

// member links a person (index into the people list) to a role. The meaning of
// role depends on the list it sits in: organization roles (owner, admin, member,
// guest) in orgSpec.members, team roles (maintainer, member) in teamSpec.members
// and project roles (lead, contributor, viewer) in projectSpec.members.
type member struct {
	person int
	role   string
}

type labelSpec struct{ name, color string }

type componentSpec struct{ name, description string }

type teamSpec struct {
	name    string
	members []member
}

type projectSpec struct {
	key, name, description, kind, visibility string
	template, color                          string // template defaults to kind; color defaults to "blue"
	lead                                     int    // index into the people list; must also be a lead member
	members                                  []member
	teamRoles                                []teamRoleSpec
	labels                                   []labelSpec
	components                               []componentSpec
}

// teamRoleSpec grants a project role to every member of a team of the organization.
type teamRoleSpec struct {
	team, role string
}

type orgSpec struct {
	id                      uuid.UUID
	slug, name, description string
	members                 []member
	teams                   []teamSpec
	projects                []projectSpec
}

type projectModel struct {
	id         uuid.UUID
	key        string
	labels     []uuid.UUID
	components []uuid.UUID
}

// orgModel holds the ids produced by buildOrg that the issue generator needs.
type orgModel struct {
	orgID      uuid.UUID
	users      []uuid.UUID // every platform user, same order as the people list
	requesters []uuid.UUID // organization guests
	agents     []uuid.UUID // organization members, admins and owners
	projects   []projectModel
}

var (
	orgCreated     = daysFromBase(-120)
	projCreated    = daysFromBase(-110)
	catalogCreated = daysFromBase(-100)
)

// templateIcons maps a project template to the PrimeIcons class the web client shows for it.
var templateIcons = map[string]string{
	"scrum":         "pi pi-sync",
	"kanban":        "pi pi-table",
	"task_tracking": "pi pi-check-square",
	"bug_tracking":  "pi pi-exclamation-triangle",
}

var standardColumns = []struct {
	name, status string
	position     int
	wip          int // 0 means no limit
}{
	{"Open", "open", 0, 0},
	{"In progress", "in_progress", 1, 5},
	{"Resolved", "resolved", 2, 0},
	{"Closed", "closed", 3, 0},
}

var commonLabels = []labelSpec{
	{"bug", "red"}, {"feature", "green"}, {"infra", "purple"}, {"docs", "gray"}, {"urgente", "orange"},
}

// platformRole returns the platform role of an account. Only the platform
// administrator is an admin; every other account is a regular user.
func platformRole(email string) string {
	if email == "admin@corelog.local" {
		return "admin"
	}
	return "user"
}

// addBoard writes a board with the standard columns and returns its id.
func addBoard(ds *dataset, projectID uuid.UUID, projectKey, name, kind string, isDefault bool, at time.Time) uuid.UUID {
	id := idFor("board", projectKey, name)
	ds.table("boards", "id", "project_id", "name", "type", "config", "is_default", "created_at", "updated_at").
		add(id, projectID, name, kind, "{}", isDefault, at, at)
	cols := ds.table("board_columns", "id", "board_id", "name", "status", "position", "wip_limit", "created_at", "updated_at")
	for _, c := range standardColumns {
		var wip any
		if c.wip > 0 {
			wip = c.wip
		}
		cols.add(idFor("board_column", id.String(), c.status), id, c.name, c.status, c.position, wip, at, at)
	}
	filters := ds.table("board_quick_filters", "id", "board_id", "name", "query_text", "position")
	for i, f := range []struct{ name, query string }{
		{"Mis incidencias", "assignee = currentUser()"},
		{"Sin responsable", "assignee is EMPTY"},
		{"Prioridad alta", "priority in (high, critical)"},
	} {
		filters.add(idFor("board_quick_filter", id.String(), f.name), id, f.name, f.query, i)
	}
	return id
}

// addSprint writes a sprint and returns its id.
func addSprint(ds *dataset, projectID uuid.UUID, projectKey, name, goal, status string, start, end time.Time, closed *time.Time) uuid.UUID {
	id := idFor("sprint", projectKey, name)
	ds.table("sprints", "id", "project_id", "name", "goal", "status", "start_date", "end_date", "closed_at", "created_at", "updated_at").
		add(id, projectID, name, goal, status, start.Format("2006-01-02"), end.Format("2006-01-02"), opt(closed), start, start)
	ds.runFinally(sprintHistorySQL)
	return id
}

// seedPhone returns the fake phone of the i-th seeded person, or ok=false for people without one.
// Numbers follow the documented fake pattern +51 9000 nnnnn (Peru, a block that is not assigned to any
// operator, so no SMS can ever reach a real person); every 5th person has a phone and every 15th has it
// verified. The result is a pure function of the position, so seeding is deterministic and idempotent.
func seedPhone(i int) (national string, verified, ok bool) {
	if i%5 != 0 {
		return "", false, false
	}
	return fmt.Sprintf("9000%05d", i), i%15 == 0, true
}

// buildUsers writes the platform accounts once. Organizations reference them by index. name is the full
// name; first_name and last_name come from the same split the application uses.
func buildUsers(ds *dataset, people []person, passwordHash string) []uuid.UUID {
	users := ds.table("users", "id", "name", "first_name", "last_name", "display_name", "email", "password_hash", "role",
		"phone_country", "phone_number", "phone_e164", "phone_verified_at", "created_at", "updated_at")
	ids := make([]uuid.UUID, len(people))
	for i, p := range people {
		ids[i] = userID(p.email)
		first, last := userdomain.SplitName(p.name)
		var country, national, e164, verifiedAt any
		if n, verified, ok := seedPhone(i); ok {
			country, national, e164 = "PE", n, "+51"+n
			if verified {
				verifiedAt = orgCreated
			}
		}
		users.add(ids[i], p.name, first, last, p.name, p.email, passwordHash, p.platform,
			country, national, e164, verifiedAt, orgCreated, orgCreated)
	}
	return ids
}

// buildOrg writes one organization with its members, teams, projects, labels,
// components, default boards and default workflows. users are the ids returned
// by buildUsers. The organization row is skipped when it already exists, which
// is the case for the default "corelog" organization created by the migrations.
func buildOrg(ds *dataset, spec orgSpec, users []uuid.UUID) orgModel {
	orgs := ds.ignoreDuplicates("organizations", "id", "slug", "name", "description", "created_by", "created_at", "updated_at")
	orgs.add(spec.id, spec.slug, spec.name, spec.description, nil, orgCreated, orgCreated)
	buildCatalogs(ds, spec.id, spec.slug)

	model := orgModel{users: users, orgID: spec.id}
	orgMembers := ds.table("organization_members", "organization_id", "user_id", "role", "joined_at")
	for _, m := range spec.members {
		uid := users[m.person]
		orgMembers.add(spec.id, uid, m.role, orgCreated)
		if m.role == "guest" {
			model.requesters = append(model.requesters, uid)
		} else {
			model.agents = append(model.agents, uid)
		}
	}

	teams := ds.table("teams", "id", "organization_id", "name", "created_at", "updated_at")
	teamMembers := ds.table("team_members", "team_id", "user_id", "role", "created_at")
	for _, ts := range spec.teams {
		tid := idFor("team", spec.slug, ts.name)
		teams.add(tid, spec.id, ts.name, orgCreated, orgCreated)
		for _, m := range ts.members {
			teamMembers.add(tid, users[m.person], m.role, orgCreated)
		}
	}

	projects := ds.table("projects", "id", "organization_id", "key", "name", "description", "type", "template",
		"color", "icon", "lead_id", "visibility", "default_assignee_policy", "next_issue_number", "archived_at", "created_at", "updated_at")
	projectMembers := ds.table("project_members", "project_id", "user_id", "role", "created_at")
	projectTeamRoles := ds.table("project_team_roles", "project_id", "team_id", "role", "created_at")
	labels := ds.table("labels", "id", "project_id", "name", "color", "created_at", "updated_at")
	components := ds.table("components", "id", "project_id", "name", "description", "created_at", "updated_at")

	for _, ps := range spec.projects {
		pid := projectID(ps.key)
		template := ps.template
		if template == "" {
			template = ps.kind
		}
		color := ps.color
		if color == "" {
			color = "blue"
		}
		// next_issue_number starts at 1; syncIssueCounters moves it past the seeded issues.
		projects.add(pid, spec.id, ps.key, ps.name, ps.description, ps.kind, template, color, templateIcons[template],
			users[ps.lead], ps.visibility, "unassigned", 1, nil, projCreated, projCreated)

		pm := projectModel{id: pid, key: ps.key}
		for _, m := range ps.members {
			projectMembers.add(pid, users[m.person], m.role, projCreated)
		}
		for _, tr := range ps.teamRoles {
			projectTeamRoles.add(pid, idFor("team", spec.slug, tr.team), tr.role, projCreated)
		}

		for _, l := range ps.labels {
			lid := idFor("label", ps.key, l.name)
			labels.add(lid, pid, l.name, l.color, catalogCreated, catalogCreated)
			pm.labels = append(pm.labels, lid)
		}
		for _, c := range ps.components {
			cid := idFor("component", ps.key, c.name)
			components.add(cid, pid, c.name, c.description, catalogCreated, catalogCreated)
			pm.components = append(pm.components, cid)
		}

		addBoard(ds, pid, ps.key, "Tablero principal", ps.kind, true, catalogCreated)

		model.projects = append(model.projects, pm)
	}
	// Every project gets the workflow of its template and its board columns are bound to its statuses before
	// the first issue is written; the issues take their status from the workflow of the project.
	ds.runBefore("issues", ensureWorkflowsSQL)
	ds.runFinally(ensureWorkflowsSQL)
	ds.runFinally(statusHistorySQL)
	return model
}

// ensureWorkflowsSQL gives every project without one the workflow of its template (idempotent).
const ensureWorkflowsSQL = `SELECT corelog_ensure_workflow(id) FROM projects`

// statusHistorySQL writes the initial status history of the issues that have none (idempotent).
const statusHistorySQL = `
INSERT INTO issue_status_history (issue_id, from_status_id, to_status_id, changed_by, changed_at)
SELECT i.id, NULL, (w.definition ->> 'initial')::uuid, i.reporter_id, i.created_at
FROM issues i JOIN workflows w ON w.project_id = i.project_id
WHERE NOT EXISTS (SELECT 1 FROM issue_status_history h WHERE h.issue_id = i.id);
INSERT INTO issue_status_history (issue_id, from_status_id, to_status_id, changed_by, changed_at)
SELECT i.id, (w.definition ->> 'initial')::uuid, i.status_id, NULL, GREATEST(i.updated_at, i.created_at)
FROM issues i JOIN workflows w ON w.project_id = i.project_id
WHERE i.status_id <> (w.definition ->> 'initial')::uuid
  AND (SELECT count(*) FROM issue_status_history h WHERE h.issue_id = i.id) = 1`

// sprintHistorySQL rebuilds the scope history of the seeded sprints that are active or closed: lifecycle
// timestamps, the committed snapshot, the completed/incomplete verdicts of closed sprints and their points.
// It only touches sprints that have no events yet, so running it again changes nothing.
const sprintHistorySQL = `
UPDATE sprints SET started_at = COALESCE(started_at, start_date::timestamptz), completed_at = COALESCE(completed_at, closed_at)
WHERE status IN ('active', 'closed');
INSERT INTO sprint_issue_events (sprint_id, issue_id, event, points_at_time, status_category_at_time, happened_at, after_start)
SELECT s.id, i.id, 'added', i.story_points, 'todo', s.started_at, FALSE
FROM sprints s JOIN issues i ON i.sprint_id = s.id AND i.deleted_at IS NULL
WHERE s.status IN ('active', 'closed') AND NOT EXISTS (SELECT 1 FROM sprint_issue_events e WHERE e.sprint_id = s.id);
INSERT INTO sprint_issue_events (sprint_id, issue_id, event, points_at_time, status_category_at_time, happened_at, after_start)
SELECT s.id, i.id, CASE WHEN corelog_issue_category(i.status_id, i.status) = 'done' THEN 'completed' ELSE 'incomplete' END,
       i.story_points, corelog_issue_category(i.status_id, i.status), COALESCE(s.closed_at, s.updated_at), TRUE
FROM sprints s JOIN issues i ON i.sprint_id = s.id AND i.deleted_at IS NULL
WHERE s.status = 'closed'
  AND NOT EXISTS (SELECT 1 FROM sprint_issue_events e WHERE e.sprint_id = s.id AND e.event IN ('completed', 'incomplete'));
UPDATE sprints s SET committed_points = COALESCE((SELECT SUM(e.points_at_time) FROM sprint_issue_events e
    WHERE e.sprint_id = s.id AND e.event = 'added' AND NOT e.after_start), 0)
WHERE s.status IN ('active', 'closed') AND s.committed_points IS NULL;
UPDATE sprints s SET completed_points = COALESCE((SELECT SUM(e.points_at_time) FROM sprint_issue_events e
    WHERE e.sprint_id = s.id AND e.event = 'completed'), 0)
WHERE s.status = 'closed' AND s.completed_points IS NULL`
