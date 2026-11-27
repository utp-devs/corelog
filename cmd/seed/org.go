package main

import (
	"time"

	"github.com/google/uuid"
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

var standardTransitions = []struct{ from, to, roles string }{
	{"open", "in_progress", "admin,lead,contributor,viewer"},
	{"open", "closed", "admin,lead,contributor"},
	{"in_progress", "resolved", "admin,lead,contributor"},
	{"in_progress", "closed", "admin"},
	{"resolved", "closed", "admin,lead,contributor,viewer"},
	{"resolved", "in_progress", "admin,lead,contributor"},
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
	return id
}

// addSprint writes a sprint and returns its id.
func addSprint(ds *dataset, projectID uuid.UUID, projectKey, name, goal, status string, start, end time.Time, closed *time.Time) uuid.UUID {
	id := idFor("sprint", projectKey, name)
	ds.table("sprints", "id", "project_id", "name", "goal", "status", "start_date", "end_date", "closed_at", "created_at", "updated_at").
		add(id, projectID, name, goal, status, start.Format("2006-01-02"), end.Format("2006-01-02"), opt(closed), start, start)
	return id
}

// buildUsers writes the platform accounts once. Organizations reference them by index.
func buildUsers(ds *dataset, people []person, passwordHash string) []uuid.UUID {
	users := ds.table("users", "id", "name", "display_name", "email", "password_hash", "role", "created_at", "updated_at")
	ids := make([]uuid.UUID, len(people))
	for i, p := range people {
		ids[i] = userID(p.email)
		users.add(ids[i], p.name, p.name, p.email, passwordHash, p.platform, orgCreated, orgCreated)
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

	model := orgModel{users: users}
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
	workflows := ds.table("workflows", "id", "project_id", "name", "is_default", "created_at", "updated_at")
	transitions := ds.table("workflow_transitions", "workflow_id", "from_status", "to_status", "allowed_roles")

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

		wf := idFor("workflow", ps.key, "default")
		workflows.add(wf, pid, "Workflow por defecto", true, catalogCreated, catalogCreated)
		for _, t := range standardTransitions {
			transitions.add(wf, t.from, t.to, t.roles)
		}
		model.projects = append(model.projects, pm)
	}
	return model
}
