package main

import (
	"context"
	"math/rand/v2"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

const demoIssueCount = 120

func init() { Register(demoScenario{}) }

// demoScenario reproduces the former scripts/seed-demo.sql: 21 accounts, 5 teams,
// 4 projects, 120 issues with comments, watchers, links, sprints, saved filters
// and notifications. All accounts share the password from SEED_PASSWORD.
type demoScenario struct{}

func (demoScenario) Name() string { return "demo" }

func (demoScenario) Run(ctx context.Context, db *sqlx.DB, opts Options) error {
	return persist(ctx, db, buildDemo(opts.PasswordHash))
}

// demoEntry describes one member of the demo organization.
type demoEntry struct {
	name, email, orgRole string
	team                 int // 1-based index into demoTeams, 0 for none
}

var demoTeams = []string{"Core Development", "Soporte Nivel 1", "Soporte Nivel 2", "Seguridad", "Infraestructura"}

// demoRoster lists the people of the demo organization "corelog". Former agents
// are organization members and former requesters are guests.
func demoRoster() []demoEntry {
	return []demoEntry{
		{"Admin User", "admin@corelog.local", "owner", 1},
		{"Sebastian Castillo Silva", "sebastian.castillo@corelog.test", "admin", 3},
		{"Pablo Medina Salazar", "pablo.medina@corelog.test", "admin", 4},
		{"Agent User", "agent@corelog.local", "member", 2},
		{"Lucia Medina Torres", "lucia.medina@corelog.test", "member", 2},
		{"Karina Navarro Salazar", "karina.navarro@corelog.test", "member", 2},
		{"Luis Paredes Flores", "luis.paredes@corelog.test", "member", 5},
		{"Javier Castillo Ortega", "javier.castillo@corelog.test", "member", 4},
		{"Ricardo Campos Salazar", "ricardo.campos@corelog.test", "member", 4},
		{"Camila Rojas Vega", "camila.rojas@corelog.test", "member", 3},
		{"Requester User", "requester@corelog.local", "guest", 1},
		{"Ana Torres Ruiz", "ana.torres@corelog.test", "guest", 1},
		{"Carlos Vega Mendoza", "carlos.vega@corelog.test", "guest", 5},
		{"Maria Gonzalez Rios", "maria.gonzalez@corelog.test", "guest", 2},
		{"Diego Herrera Luna", "diego.herrera@corelog.test", "guest", 3},
		{"Valeria Soto Pena", "valeria.soto@corelog.test", "guest", 4},
		{"Jorge Quispe Huanca", "jorge.quispe@corelog.test", "guest", 5},
		{"Paola Mendoza Diaz", "paola.mendoza@corelog.test", "guest", 2},
		{"Andres Llanos Vidal", "andres.llanos@corelog.test", "guest", 1},
		{"Rosa Infante Castro", "rosa.infante@corelog.test", "guest", 3},
		{"Miguel Ortiz Tapia", "miguel.ortiz@corelog.test", "guest", 4},
	}
}

func rosterPeople(entries []demoEntry) []person {
	out := make([]person, len(entries))
	for i, e := range entries {
		out[i] = person{name: e.name, email: e.email, platform: platformRole(e.email)}
	}
	return out
}

// demoOrgSpec turns the roster into the organization "corelog" with its teams and projects.
func demoOrgSpec(entries []demoEntry) orgSpec {
	spec := orgSpec{
		id:          defaultOrgID,
		slug:        "corelog",
		name:        "CoreLog",
		description: "Organizacion por defecto de CoreLog",
	}
	teamMembers := make([][]member, len(demoTeams))
	for i, e := range entries {
		spec.members = append(spec.members, member{person: i, role: e.orgRole})
		if e.team > 0 {
			role := "member"
			if e.orgRole == "owner" || e.orgRole == "admin" {
				role = "maintainer"
			}
			teamMembers[e.team-1] = append(teamMembers[e.team-1], member{person: i, role: role})
		}
	}
	for i, name := range demoTeams {
		spec.teams = append(spec.teams, teamSpec{name: name, members: teamMembers[i]})
	}
	for _, ps := range demoProjects() {
		ps.members = projectMembersFor(entries, ps.lead)
		spec.projects = append(spec.projects, ps)
	}
	return spec
}

// projectMembersFor gives every organization member a project role: the lead is
// lead, guests are viewers and everyone else contributes.
func projectMembersFor(entries []demoEntry, lead int) []member {
	out := make([]member, 0, len(entries))
	for i, e := range entries {
		role := "contributor"
		switch {
		case i == lead:
			role = "lead"
		case e.orgRole == "guest":
			role = "viewer"
		}
		out = append(out, member{person: i, role: role})
	}
	return out
}

func demoProjects() []projectSpec {
	return []projectSpec{
		{
			key: "CORE", name: "Plataforma CoreLog", description: "Proyecto principal de la plataforma",
			kind: "kanban", visibility: "open", lead: 0, labels: commonLabels,
			components: []componentSpec{{"API", "Endpoints REST del backend"}, {"Frontend", "Interfaz React del panel"}, {"Base de datos", "Esquema, migraciones y consultas"}},
		},
		{
			key: "OPS", name: "Operaciones e infraestructura", description: "Operacion, despliegues y monitoreo",
			kind: "kanban", visibility: "open", lead: 1, labels: commonLabels,
			components: []componentSpec{{"Red", "Conectividad y balanceo"}, {"Servidores", "Hosts, VPS y contenedores"}, {"Monitoreo", "Alertas, metricas y logs"}},
		},
		{
			key: "SEC", name: "Seguridad", description: "Hallazgos de seguridad y cumplimiento",
			kind: "scrum", visibility: "private", lead: 2, labels: commonLabels,
			components: []componentSpec{{"Autenticacion", "Login, sesiones y JWT"}, {"Auditoria", "Trazabilidad de cambios"}, {"Politicas", "Politicas de acceso y contrasenas"}},
		},
		{
			key: "WEB", name: "Portal web", description: "Sitio publico y experiencia de usuario",
			kind: "scrum", visibility: "open", lead: 0, labels: commonLabels,
			components: []componentSpec{{"UI", "Componentes visuales"}, {"Accesibilidad", "Contraste, teclado y lectores de pantalla"}, {"Rendimiento", "Tiempos de carga y bundle"}},
		},
	}
}

// buildDemoOrg writes the users and the corelog organization of a roster.
func buildDemoOrg(ds *dataset, entries []demoEntry, passwordHash string) orgModel {
	users := buildUsers(ds, rosterPeople(entries), passwordHash)
	return buildOrg(ds, demoOrgSpec(entries), users)
}

func buildDemo(passwordHash string) *dataset {
	ds := newDataset()
	rng := newRNG(streamDemo)
	org := buildDemoOrg(ds, demoRoster(), passwordHash)

	core := org.projects[0]
	sprint11 := addSprint(ds, core.id, core.key, "Sprint 11", "Cierre de la iteracion anterior", "closed",
		daysFromBase(-35), daysFromBase(-21), ptr(daysFromBase(-21)))
	sprint12 := addSprint(ds, core.id, core.key, "Sprint 12", "Estabilizar la API y el tablero", "active",
		daysFromBase(-7), daysFromBase(7), nil)
	addSprint(ds, org.projects[1].id, org.projects[1].key, "Sprint 1", "Primera iteracion de operaciones", "planned",
		daysFromBase(14), daysFromBase(28), nil)
	addBoard(ds, core.id, core.key, "Sprint 12", "kanban", false, catalogCreated)

	recs := generateIssues(rng, org, demoIssueCount)
	assignDemoSprints(recs, core.id, sprint11, sprint12)
	emitIssues(ds, recs)
	addDemoLinks(ds, rng, recs, org.users[0])
	addDemoFilters(ds)
	addDemoNotifications(ds, recs)
	return ds
}

// assignDemoSprints puts part of the CORE issues into the closed and active sprints.
func assignDemoSprints(recs []issueRec, core uuid.UUID, closedSprint, activeSprint uuid.UUID) {
	for idx := range recs {
		r := &recs[idx]
		if r.projectID != core {
			continue
		}
		n := idx + 1
		switch {
		case (r.status == "open" || r.status == "in_progress") && n%3 == 0:
			r.sprint = ptr(activeSprint)
		case (r.status == "resolved" || r.status == "closed") && n%3 == 1:
			r.sprint = ptr(closedSprint)
		}
	}
}

// addDemoLinks links about 3% of the issue pairs of a project, creating the link
// from the older issue to the newer one as the former SQL did.
func addDemoLinks(ds *dataset, rng *rand.Rand, recs []issueRec, admin uuid.UUID) {
	links := ds.table("issue_links", "id", "source_issue_id", "target_issue_id", "kind", "created_at", "created_by")
	kinds := []string{"blocks", "relates", "duplicates"}
	for a := range recs {
		for b := range recs {
			if a == b || recs[a].projectID != recs[b].projectID || !recs[b].created.After(recs[a].created) {
				continue
			}
			if rng.Float64() < 0.03 {
				links.add(idFor("issue_link", recs[a].id.String(), recs[b].id.String()),
					recs[a].id, recs[b].id, kinds[rng.IntN(len(kinds))], baseTime, admin)
			}
		}
	}
}

func addDemoFilters(ds *dataset) {
	filters := ds.table("saved_filters", "id", "owner_id", "name", "filters", "created_at", "updated_at")
	admin := userID("admin@corelog.local")
	agent := userID("agent@corelog.local")
	at := daysFromBase(-30)
	for _, f := range []struct {
		owner         uuid.UUID
		name, payload string
	}{
		{admin, "Criticos abiertos", `{"status":"open","priority":"critical"}`},
		{admin, "En progreso", `{"status":"in_progress"}`},
		{admin, "Bugs", `{"issue_type":"bug"}`},
		{agent, "Mis pendientes", `{"status":"in_progress"}`},
		{agent, "Alta prioridad", `{"priority":"high"}`},
	} {
		filters.add(idFor("saved_filter", f.owner.String(), f.name), f.owner, f.name, f.payload, at, at)
	}
}

// addDemoNotifications gives the admin, the agent and the requester a few
// notifications about existing issues, half of them already read.
func addDemoNotifications(ds *dataset, recs []issueRec) {
	notes := ds.table("notifications", "id", "user_id", "kind", "message", "link", "read_at", "created_at")
	for ri, email := range []string{"admin@corelog.local", "agent@corelog.local", "requester@corelog.local"} {
		uid := userID(email)
		for j := 0; j < 5; j++ {
			rec := recs[(ri*17+j*11)%len(recs)]
			kind, message := "issue_assigned", "Te asignaron "+rec.title
			if j%2 == 1 {
				kind, message = "comment_added", "Nuevo comentario en "+rec.title
			}
			created := daysFromBase(-(j + 1))
			var readAt any
			if j < 2 {
				readAt = created.Add(12 * time.Hour)
			}
			notes.add(idFor("notification", email, strconv.Itoa(j)), uid, kind, message,
				"/issues/"+rec.id.String(), readAt, created)
		}
	}
}
