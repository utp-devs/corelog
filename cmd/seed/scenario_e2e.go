package main

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

func init() { Register(e2eScenario{}) }

// People indexes of the e2e accounts (see e2ePeople).
const (
	e2eAdmin = iota
	e2eAgent
	e2eRequester
	e2eAna
	e2eOwner
	e2eOrgAdmin
	e2eLead
	e2eMember
	e2eViewer
	e2eGuest
	e2eNewbie
)

// e2eScenario seeds the accounts used by the Playwright suite
// and a small, fixed dataset so every screen has content.
type e2eScenario struct{}

func (e2eScenario) Name() string { return "e2e" }

func (e2eScenario) Run(ctx context.Context, db *sqlx.DB, opts Options) error {
	if err := persist(ctx, db, buildE2E(opts.PasswordHash)); err != nil {
		return err
	}
	fmt.Fprintln(opts.Out, "Cuentas creadas (la contrasena es SEED_PASSWORD):")
	printE2EAccounts(opts.Out)
	return nil
}

// e2ePeople is the account map used by the Playwright suite. The legacy accounts
// (admin, agent, requester, ana) keep their emails for the existing specs.
func e2ePeople() []person {
	return []person{
		{"Admin User", "admin@corelog.local", "admin"},
		{"Agent User", "agent@corelog.local", "user"},
		{"Requester User", "requester@corelog.local", "user"},
		{"Ana Torres Ruiz", "ana.torres@corelog.test", "user"},
		{"Owner User", "owner@corelog.test", "user"},
		{"Org Admin User", "orgadmin@corelog.test", "user"},
		{"Lead User", "lead@corelog.test", "user"},
		{"Member User", "member@corelog.test", "user"},
		{"Viewer User", "viewer@corelog.test", "user"},
		{"Guest User", "guest@corelog.test", "user"},
		{"Newbie User", "newbie@corelog.test", "user"},
	}
}

// e2eCorelogSpec is the default organization. Its legacy members keep access to
// it; the platform admin owns it. Newbie belongs to no organization.
func e2eCorelogSpec() orgSpec {
	return orgSpec{
		id:          defaultOrgID,
		slug:        "corelog",
		name:        "CoreLog",
		description: "Organizacion por defecto de CoreLog",
		members: []member{
			{e2eAdmin, "owner"},
			{e2eAgent, "member"},
			{e2eRequester, "guest"},
			{e2eAna, "guest"},
		},
	}
}

// e2eAcmeSpec is the organization "Acme" used by the role specs. The legacy
// accounts also belong to it so the seeded issues they are assigned to stay visible.
func e2eAcmeSpec() orgSpec {
	return orgSpec{
		id:          idFor("organization", "acme"),
		slug:        "acme",
		name:        "Acme",
		description: "Organizacion de prueba para los roles de la suite e2e",
		members: []member{
			{e2eAdmin, "owner"},
			{e2eOwner, "owner"},
			{e2eOrgAdmin, "admin"},
			{e2eLead, "member"},
			{e2eMember, "member"},
			{e2eViewer, "member"},
			{e2eGuest, "guest"},
			{e2eAgent, "member"},
			{e2eRequester, "guest"},
			{e2eAna, "guest"},
		},
		teams: []teamSpec{
			{name: "Platform", members: []member{{e2eLead, "maintainer"}, {e2eMember, "member"}, {e2eAgent, "member"}}},
			{name: "Design", members: []member{{e2eViewer, "member"}}},
		},
		projects: []projectSpec{
			{
				key: "SCRUM", name: "Proyecto Scrum", description: "Proyecto de prueba con sprint activo",
				kind: "scrum", template: "scrum", color: "palette-3", visibility: "open", lead: e2eLead,
				members: []member{
					{e2eLead, "lead"}, {e2eAdmin, "contributor"}, {e2eOwner, "contributor"}, {e2eOrgAdmin, "contributor"},
					{e2eMember, "contributor"}, {e2eAgent, "contributor"}, {e2eViewer, "viewer"},
					{e2eRequester, "viewer"}, {e2eAna, "viewer"},
				},
				teamRoles:  []teamRoleSpec{{"Design", "viewer"}},
				labels:     []labelSpec{{"bug", "red"}, {"feature", "green"}, {"docs", "gray"}},
				components: []componentSpec{{"API", "Endpoints REST del backend"}, {"Frontend", "Interfaz React del panel"}},
			},
			{
				key: "KAN", name: "Proyecto Kanban", description: "Proyecto de prueba de flujo continuo",
				kind: "kanban", template: "kanban", color: "palette-5", visibility: "open", lead: e2eOwner,
				members: []member{
					{e2eOwner, "lead"}, {e2eAdmin, "contributor"}, {e2eMember, "contributor"}, {e2eAgent, "contributor"},
					{e2eGuest, "viewer"}, {e2eRequester, "viewer"}, {e2eAna, "viewer"},
				},
				teamRoles:  []teamRoleSpec{{"Platform", "contributor"}},
				labels:     []labelSpec{{"bug", "red"}, {"infra", "purple"}},
				components: []componentSpec{{"Operaciones", "Hosts, certificados y respaldos"}},
			},
			{
				key: "SEC", name: "Seguridad", description: "Proyecto privado: el miembro comun no tiene acceso",
				kind: "kanban", template: "kanban", color: "palette-9", visibility: "private", lead: e2eOwner,
				members: []member{{e2eOwner, "lead"}, {e2eOrgAdmin, "contributor"}},
				labels:  []labelSpec{{"bug", "red"}},
			},
			{
				key: "EMP", name: "Proyecto vacio", description: "Proyecto sin incidencias, para estados vacios",
				kind: "kanban", template: "task_tracking", color: "palette-11", visibility: "open", lead: e2eOwner,
				members: []member{{e2eOwner, "lead"}},
			},
		},
	}
}

// e2eAccountLines describes the memberships of every e2e account, for the printed summary.
func e2eAccountLines() []string {
	people := e2ePeople()
	orgs := []orgSpec{e2eCorelogSpec(), e2eAcmeSpec()}
	lines := make([]string, 0, len(people))
	for i, p := range people {
		var parts []string
		for _, o := range orgs {
			for _, m := range o.members {
				if m.person == i {
					parts = append(parts, fmt.Sprintf("org %s:%s", o.slug, m.role))
				}
			}
		}
		for _, o := range orgs {
			for _, ps := range o.projects {
				for _, m := range ps.members {
					if m.person == i {
						parts = append(parts, fmt.Sprintf("%s:%s", ps.key, m.role))
					}
				}
			}
		}
		if len(parts) == 0 {
			parts = []string{"sin organizacion"}
		}
		lines = append(lines, fmt.Sprintf("  %s platform=%s  %s", p.email, p.platform, strings.Join(parts, "  ")))
	}
	return lines
}

func printE2EAccounts(out io.Writer) {
	for _, line := range e2eAccountLines() {
		fmt.Fprintln(out, line)
	}
}

type e2eComment struct {
	author int
	body   string
}

// e2eIssue is one hand-written issue. People, projects, labels and components are
// indexes; parent is the 1-based issue number; -1 means "none".
type e2eIssue struct {
	project, assignee, parent int
	itype, title, description string
	status, priority          string
	requester                 int
	sprint, closedSprint      bool
	points                    float64
	labels, components        []int
	watchers                  []int
	comments                  []e2eComment
	createdDays, closedDays   int
}

var e2eIssues = []e2eIssue{
	{project: 0, assignee: -1, parent: -1, itype: "epic", title: "Portal de autoservicio", description: "Agrupa las historias del portal de autoservicio para solicitantes.", status: "open", priority: "high", requester: e2eAdmin, createdDays: 30},
	{project: 0, assignee: e2eAgent, parent: -1, itype: "story", title: "Formulario de alta de incidencia", description: "Crear incidencias desde el portal.", status: "in_progress", priority: "high", requester: e2eAna, sprint: true, points: 5, labels: []int{1}, watchers: []int{e2eAdmin, e2eAna}, comments: []e2eComment{{e2eAgent, "Ya tengo el borrador de la validacion."}}, createdDays: 20},
	{project: 0, assignee: -1, parent: -1, itype: "task", title: "Validar campos obligatorios en el formulario", description: "Mostrar el error junto a cada control.", status: "open", priority: "medium", requester: e2eAna, sprint: true, points: 2, comments: []e2eComment{{e2eAna, "Pendiente de validacion del cliente."}}, createdDays: 12},
	{project: 0, assignee: e2eAgent, parent: -1, itype: "bug", title: "Error 500 al guardar borrador", description: "El guardado responde 500 cuando el titulo supera 120 caracteres.", status: "in_progress", priority: "critical", requester: e2eRequester, sprint: true, points: 3, labels: []int{0}, watchers: []int{e2eAdmin, e2eAgent}, comments: []e2eComment{{e2eAdmin, "Ya reproduje el problema en staging."}, {e2eAgent, "Subo un fix en la siguiente iteracion."}}, createdDays: 9},
	{project: 0, assignee: e2eAgent, parent: -1, itype: "bug", title: "Mejorar contraste de badges en modo oscuro", description: "Algunos badges son dificiles de leer sobre fondo oscuro.", status: "resolved", priority: "medium", requester: e2eAdmin, sprint: true, points: 1, labels: []int{0}, createdDays: 15, closedDays: 4},
	{project: 0, assignee: e2eAdmin, parent: -1, itype: "task", title: "Documentar endpoints de comentarios", description: "Actualizar la referencia de la API.", status: "closed", priority: "low", requester: e2eAdmin, createdDays: 25, closedDays: 18},
	{project: 0, assignee: -1, parent: -1, itype: "story", title: "Notificaciones por correo", description: "Avisar por correo cuando asignen una incidencia.", status: "open", priority: "medium", requester: e2eRequester, labels: []int{1}, createdDays: 6},
	{project: 0, assignee: e2eAgent, parent: 7, itype: "subtask", title: "Plantilla del correo de asignacion", description: "Redactar la plantilla del aviso.", status: "open", priority: "low", requester: e2eRequester, createdDays: 5},
	{project: 0, assignee: e2eAgent, parent: -1, itype: "task", title: "Paginar listado de incidencias", description: "El listado debe paginar con page y page_size.", status: "open", priority: "high", requester: e2eAna, points: 3, labels: []int{2}, createdDays: 3},
	{project: 0, assignee: e2eAdmin, parent: -1, itype: "task", title: "Revisar permisos de la carpeta compartida", description: "Confirmar accesos del equipo de soporte.", status: "in_progress", priority: "high", requester: e2eRequester, components: []int{1}, createdDays: 8},
	{project: 1, assignee: -1, parent: -1, itype: "task", title: "Migrar backups a almacenamiento externo", description: "Mover los respaldos nocturnos a un almacenamiento externo.", status: "open", priority: "high", requester: e2eRequester, components: []int{0}, createdDays: 14},
	{project: 1, assignee: e2eAgent, parent: -1, itype: "task", title: "Renovar certificado TLS del dominio de API", description: "El certificado vence en 30 dias.", status: "in_progress", priority: "critical", requester: e2eAdmin, components: []int{0}, watchers: []int{e2eAgent}, comments: []e2eComment{{e2eAdmin, "Coordinemos con infraestructura antes de tocar la base."}}, createdDays: 10},
	{project: 1, assignee: -1, parent: -1, itype: "bug", title: "Notificaciones duplicadas al reasignar", description: "Se envia el aviso dos veces al reasignar.", status: "open", priority: "medium", requester: e2eAna, labels: []int{0}, createdDays: 2},
	{project: 1, assignee: e2eAgent, parent: -1, itype: "task", title: "Configurar alertas de CPU en produccion", description: "Alertar por uso de CPU sostenido.", status: "resolved", priority: "medium", requester: e2eAdmin, labels: []int{1}, createdDays: 21, closedDays: 7},
	{project: 1, assignee: e2eAdmin, parent: -1, itype: "bug", title: "Actualizar dependencias con vulnerabilidades criticas", description: "Actualizar paquetes con CVE criticos.", status: "closed", priority: "critical", requester: e2eAdmin, labels: []int{0}, createdDays: 28, closedDays: 22},
	{project: 0, assignee: -1, parent: -1, itype: "epic", title: "Notificaciones y avisos", description: "Agrupa el trabajo de avisos por correo y dentro de la aplicacion.", status: "in_progress", priority: "medium", requester: e2eAdmin, createdDays: 40},
	{project: 0, assignee: e2eAgent, parent: 1, itype: "story", title: "Registro de actividad del solicitante", description: "Mostrar al solicitante el historial de sus incidencias.", status: "closed", priority: "high", requester: e2eAna, closedSprint: true, points: 5, createdDays: 35, closedDays: 16},
	{project: 0, assignee: e2eMember, parent: 16, itype: "task", title: "Plantilla de correo de bienvenida", description: "Redactar el correo que recibe quien se registra.", status: "resolved", priority: "medium", requester: e2eAdmin, closedSprint: true, points: 3, createdDays: 34, closedDays: 17},
	{project: 0, assignee: e2eAgent, parent: -1, itype: "bug", title: "Corregir enlace roto en el portal", description: "El enlace de ayuda del pie de pagina devuelve 404.", status: "closed", priority: "high", requester: e2eRequester, closedSprint: true, points: 2, labels: []int{0}, createdDays: 33, closedDays: 15},
	{project: 0, assignee: e2eMember, parent: 16, itype: "task", title: "Migrar estilos al sistema de diseno", description: "No terminada en el sprint: pasa al siguiente.", status: "in_progress", priority: "low", requester: e2eAdmin, closedSprint: true, points: 8, createdDays: 32},
}

// buildE2E builds the e2e dataset. Issue n (1-based) is e2eIssues[n-1].
func buildE2E(passwordHash string) *dataset {
	ds := newDataset()
	users := buildUsers(ds, e2ePeople(), passwordHash)
	buildOrg(ds, e2eCorelogSpec(), users)
	org := buildOrg(ds, e2eAcmeSpec(), users)
	sprintID := addSprint(ds, org.projects[0].id, org.projects[0].key, "Sprint 1",
		"Entregar el flujo de alta de incidencias", "active", daysFromBase(-7), daysFromBase(7), nil)

	closedSprintID := addSprint(ds, org.projects[0].id, org.projects[0].key, "Sprint 0",
		"Primera entrega del portal", "closed", daysFromBase(-28), daysFromBase(-14), ptr(daysFromBase(-14)))

	recs := make([]issueRec, 0, len(e2eIssues))
	for n, spec := range e2eIssues {
		proj := org.projects[spec.project]
		created := daysFromBase(-spec.createdDays)
		rec := issueRec{
			orgID:       org.orgID,
			id:          issueID(n + 1),
			projectID:   proj.id,
			reporter:    org.users[spec.requester],
			summary:     spec.title,
			description: spec.description,
			status:      spec.status,
			priority:    spec.priority,
			itype:       spec.itype,
			rank:        fmt.Sprintf("r%05d", n+1),
			created:     created,
			updated:     minTime(baseTime, created.Add(3*day)),
		}
		if spec.assignee >= 0 {
			rec.assignee = ptr(org.users[spec.assignee])
		}
		if spec.parent > 0 {
			rec.parent = ptr(issueID(spec.parent))
		}
		if spec.sprint {
			rec.sprint = ptr(sprintID)
		}
		if spec.closedSprint {
			rec.sprint = ptr(closedSprintID)
		}
		if spec.points > 0 {
			rec.points = ptr(spec.points)
		}
		if spec.closedDays > 0 {
			closed := daysFromBase(-spec.closedDays)
			rec.closed = &closed
			rec.updated = closed
		}
		for _, l := range spec.labels {
			rec.labels = append(rec.labels, proj.labels[l])
		}
		for _, c := range spec.components {
			rec.components = append(rec.components, proj.components[c])
		}
		for _, w := range spec.watchers {
			rec.watchers = append(rec.watchers, org.users[w])
		}
		for k, c := range spec.comments {
			rec.comments = append(rec.comments, commentRec{
				author: org.users[c.author],
				body:   c.body,
				at:     minTime(baseTime, created.Add(time.Duration(k+1)*day)),
			})
		}
		recs = append(recs, rec)
	}
	addE2EConversations(recs, org)
	addE2ECustomFields(ds, org, recs)
	emitIssues(ds, recs)
	syncIssueCounters(ds, recs)
	addE2EFilterAndNotifications(ds, org, recs)
	addE2ESearchFilters(ds, org.orgID, org.users[e2eAdmin])
	addVersions(ds, org, recs)
	addComponentLeads(ds)
	addEpicDates(ds, recs)
	addE2EWorklogs(ds, org, recs)
	addPlanningSeed(ds, org.orgID, org.users[e2eAdmin], org.users[e2eMember], org.projects[0].id)
	addE2EFavoritesAndRecents(ds, users, org.projects)
	return ds
}

// addE2EFavoritesAndRecents stars one project and records one recent view for the
// e2e member, so the favorites and recents lists are not empty in the suite.
func addE2EFavoritesAndRecents(ds *dataset, users []uuid.UUID, projects []projectModel) {
	favorites := ds.table("favorites", "user_id", "entity_type", "entity_id", "created_at")
	favorites.add(users[e2eMember], "project", projects[0].id, daysFromBase(-3))
	recents := ds.table("recent_items", "user_id", "entity_type", "entity_id", "viewed_at")
	recents.add(users[e2eMember], "project", projects[1].id, daysFromBase(-1))
}

func addE2EFilterAndNotifications(ds *dataset, org orgModel, recs []issueRec) {
	filters := ds.table("saved_filters", "id", "owner_id", "name", "filters", "created_at", "updated_at")
	admin := org.users[e2eAdmin]
	filters.add(idFor("saved_filter", admin.String(), "Criticos abiertos"), admin, "Criticos abiertos",
		`{"status":"open","priority":"critical"}`, daysFromBase(-5), daysFromBase(-5))

	notes := ds.table("notifications", "id", "user_id", "kind", "message", "link", "read_at", "created_at")
	for i, item := range []struct {
		user    int
		kind    string
		message string
		issue   int
	}{
		{e2eAna, "issue_assigned", "Te asignaron " + e2eIssues[1].title, 1},
		{e2eAgent, "issue_assigned", "Te asignaron " + e2eIssues[3].title, 3},
		{e2eAdmin, "comment_added", "Nuevo comentario en " + e2eIssues[3].title, 3},
	} {
		created := daysFromBase(-(i + 1))
		var readAt any
		if i == 0 {
			readAt = created.Add(12 * time.Hour)
		}
		notes.add(idFor("notification", strconv.Itoa(i)), org.users[item.user], item.kind, item.message,
			"/issues/"+recs[item.issue].id.String(), readAt, created)
	}
}

// addE2EConversations adds a conversation (reply, mention, internal note and reactions) to two issues of the first
// project.
func addE2EConversations(recs []issueRec, org orgModel) {
	people := func(author, replier, reactor int, authorName, replierName string) threadPeople {
		return threadPeople{author: org.users[author], replier: org.users[replier], reactor: org.users[reactor],
			authorName: authorName, replierName: replierName}
	}
	addCommentThread(&recs[1], people(e2eAgent, e2eAdmin, e2eAna, "Agent User", "Admin User"))
	addCommentThread(&recs[3], people(e2eAdmin, e2eAgent, e2eMember, "Admin User", "Agent User"))
}

// addE2EWorklogs logs time and sets estimates on three issues of the first project.
func addE2EWorklogs(ds *dataset, org orgModel, recs []issueRec) {
	agent, admin := org.users[e2eAgent], org.users[e2eAdmin]
	addIssueWorklogs(ds, recs[1].id, agent, -2, 8*3600, 7200, 5400, 3600)
	addIssueWorklogs(ds, recs[3].id, agent, -1, 4*3600, 3600, 1800)
	addIssueWorklogs(ds, recs[9].id, admin, -3, 2*3600, 1800)
}
