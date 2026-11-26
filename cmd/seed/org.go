package main

import (
	"time"

	"github.com/google/uuid"
)

// person is a seeded account. team is a 1-based index into orgSpec.teams (0 for none).
type person struct {
	name, email, role string
	team              int
}

type labelSpec struct{ name, color string }

type componentSpec struct{ name, description string }

type projectSpec struct {
	key, name, description, kind, visibility string
	lead                                     int // index into orgSpec.people
	labels                                   []labelSpec
	components                               []componentSpec
}

type orgSpec struct {
	teams    []string
	people   []person
	projects []projectSpec
}

type projectModel struct {
	id         uuid.UUID
	key        string
	labels     []uuid.UUID
	components []uuid.UUID
}

// orgModel holds the ids produced by buildOrg that the issue generator needs.
type orgModel struct {
	people   []person
	users    []uuid.UUID // same order as people
	projects []projectModel
}

func (o orgModel) idsWithRole(role string) []uuid.UUID {
	var out []uuid.UUID
	for i, p := range o.people {
		if p.role == role {
			out = append(out, o.users[i])
		}
	}
	return out
}

var (
	orgCreated     = daysFromBase(-120)
	projCreated    = daysFromBase(-110)
	catalogCreated = daysFromBase(-100)
)

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

// buildOrg writes teams, users, projects, memberships, labels, components,
// default boards and default workflows. It is shared by every scenario.
func buildOrg(ds *dataset, spec orgSpec, passwordHash string) orgModel {
	teams := ds.table("teams", "id", "name", "created_at", "updated_at")
	teamIDs := make([]uuid.UUID, len(spec.teams))
	for i, name := range spec.teams {
		teamIDs[i] = idFor("team", name)
		teams.add(teamIDs[i], name, orgCreated, orgCreated)
	}

	users := ds.table("users", "id", "name", "email", "password_hash", "role", "created_at", "updated_at")
	teamMembers := ds.table("team_members", "team_id", "user_id", "role", "created_at")
	model := orgModel{people: spec.people, users: make([]uuid.UUID, len(spec.people))}
	for i, p := range spec.people {
		id := userID(p.email)
		model.users[i] = id
		users.add(id, p.name, p.email, passwordHash, p.role, orgCreated, orgCreated)
		if p.team > 0 {
			role := "member"
			if p.role == "admin" {
				role = "owner"
			}
			teamMembers.add(teamIDs[p.team-1], id, role, orgCreated)
		}
	}

	projects := ds.table("projects", "id", "key", "name", "description", "type", "lead_id", "visibility", "archived_at", "created_at", "updated_at")
	projectMembers := ds.table("project_members", "project_id", "user_id", "role", "created_at")
	labels := ds.table("labels", "id", "project_id", "name", "color", "created_at", "updated_at")
	components := ds.table("components", "id", "project_id", "name", "description", "created_at", "updated_at")
	workflows := ds.table("workflows", "id", "project_id", "name", "is_default", "created_at", "updated_at")
	transitions := ds.table("workflow_transitions", "workflow_id", "from_status", "to_status", "allowed_roles")

	for _, ps := range spec.projects {
		pid := projectID(ps.key)
		projects.add(pid, ps.key, ps.name, ps.description, ps.kind, model.users[ps.lead], ps.visibility, nil, projCreated, projCreated)

		pm := projectModel{id: pid, key: ps.key}
		for i, p := range spec.people {
			role := "contributor"
			switch {
			case i == ps.lead:
				role = "lead"
			case p.role == "requester":
				role = "viewer"
			}
			projectMembers.add(pid, model.users[i], role, projCreated)
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

// demoOrgSpec is the organization of the demo scenario (the former scripts/seed-demo.sql).
func demoOrgSpec() orgSpec {
	return orgSpec{
		teams: []string{"Core Development", "Soporte Nivel 1", "Soporte Nivel 2", "Seguridad", "Infraestructura"},
		people: []person{
			{"Admin User", "admin@corelog.local", "admin", 1},
			{"Sebastian Castillo Silva", "sebastian.castillo@corelog.test", "admin", 3},
			{"Pablo Medina Salazar", "pablo.medina@corelog.test", "admin", 4},
			{"Agent User", "agent@corelog.local", "agent", 2},
			{"Lucia Medina Torres", "lucia.medina@corelog.test", "agent", 2},
			{"Karina Navarro Salazar", "karina.navarro@corelog.test", "agent", 2},
			{"Luis Paredes Flores", "luis.paredes@corelog.test", "agent", 5},
			{"Javier Castillo Ortega", "javier.castillo@corelog.test", "agent", 4},
			{"Ricardo Campos Salazar", "ricardo.campos@corelog.test", "agent", 4},
			{"Camila Rojas Vega", "camila.rojas@corelog.test", "agent", 3},
			{"Requester User", "requester@corelog.local", "requester", 1},
			{"Ana Torres Ruiz", "ana.torres@corelog.test", "requester", 1},
			{"Carlos Vega Mendoza", "carlos.vega@corelog.test", "requester", 5},
			{"Maria Gonzalez Rios", "maria.gonzalez@corelog.test", "requester", 2},
			{"Diego Herrera Luna", "diego.herrera@corelog.test", "requester", 3},
			{"Valeria Soto Pena", "valeria.soto@corelog.test", "requester", 4},
			{"Jorge Quispe Huanca", "jorge.quispe@corelog.test", "requester", 5},
			{"Paola Mendoza Diaz", "paola.mendoza@corelog.test", "requester", 2},
			{"Andres Llanos Vidal", "andres.llanos@corelog.test", "requester", 1},
			{"Rosa Infante Castro", "rosa.infante@corelog.test", "requester", 3},
			{"Miguel Ortiz Tapia", "miguel.ortiz@corelog.test", "requester", 4},
		},
		projects: []projectSpec{
			{
				key: "CORE", name: "Plataforma CoreLog", description: "Proyecto principal de la plataforma",
				kind: "kanban", visibility: "team", lead: 0, labels: commonLabels,
				components: []componentSpec{{"API", "Endpoints REST del backend"}, {"Frontend", "Interfaz React del panel"}, {"Base de datos", "Esquema, migraciones y consultas"}},
			},
			{
				key: "OPS", name: "Operaciones e infraestructura", description: "Operacion, despliegues y monitoreo",
				kind: "kanban", visibility: "team", lead: 1, labels: commonLabels,
				components: []componentSpec{{"Red", "Conectividad y balanceo"}, {"Servidores", "Hosts, VPS y contenedores"}, {"Monitoreo", "Alertas, metricas y logs"}},
			},
			{
				key: "SEC", name: "Seguridad", description: "Hallazgos de seguridad y cumplimiento",
				kind: "scrum", visibility: "private", lead: 2, labels: commonLabels,
				components: []componentSpec{{"Autenticacion", "Login, sesiones y JWT"}, {"Auditoria", "Trazabilidad de cambios"}, {"Politicas", "Politicas de acceso y contrasenas"}},
			},
			{
				key: "WEB", name: "Portal web", description: "Sitio publico y experiencia de usuario",
				kind: "scrum", visibility: "team", lead: 0, labels: commonLabels,
				components: []componentSpec{{"UI", "Componentes visuales"}, {"Accesibilidad", "Contraste, teclado y lectores de pantalla"}, {"Rendimiento", "Tiempos de carga y bundle"}},
			},
		},
	}
}
