package main

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/jmoiron/sqlx"
)

func init() { Register(e2eScenario{}) }

// People indexes of the e2e organization.
const (
	e2eAdmin = iota
	e2eAgent
	e2eRequester
	e2eAna
)

// e2eScenario seeds the accounts used by the Playwright suite (testing strategy
// 3.1, mapped to the global roles that exist until phase P2) and a small, fixed
// dataset so every screen has content.
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

func printE2EAccounts(out io.Writer) {
	for _, p := range e2eOrgSpec().people {
		fmt.Fprintf(out, "  %s [%s]\n", p.email, p.role)
	}
}

func e2eOrgSpec() orgSpec {
	return orgSpec{
		teams: []string{"Platform"},
		people: []person{
			{"Admin User", "admin@corelog.local", "admin", 1},
			{"Agent User", "agent@corelog.local", "agent", 1},
			{"Requester User", "requester@corelog.local", "requester", 1},
			{"Ana Torres Ruiz", "ana.torres@corelog.test", "requester", 1},
		},
		projects: []projectSpec{
			{
				key: "SCRUM", name: "Proyecto Scrum", description: "Proyecto de prueba con sprint activo",
				kind: "scrum", visibility: "team", lead: e2eAdmin,
				labels:     []labelSpec{{"bug", "red"}, {"feature", "green"}, {"docs", "gray"}},
				components: []componentSpec{{"API", "Endpoints REST del backend"}, {"Frontend", "Interfaz React del panel"}},
			},
			{
				key: "KAN", name: "Proyecto Kanban", description: "Proyecto de prueba de flujo continuo",
				kind: "kanban", visibility: "team", lead: e2eAdmin,
				labels:     []labelSpec{{"bug", "red"}, {"infra", "purple"}},
				components: []componentSpec{{"Operaciones", "Hosts, certificados y respaldos"}},
			},
		},
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
	sprint                    bool
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
}

// buildE2E builds the e2e dataset. Issue n (1-based) is e2eIssues[n-1].
func buildE2E(passwordHash string) *dataset {
	ds := newDataset()
	org := buildOrg(ds, e2eOrgSpec(), passwordHash)
	sprintID := addSprint(ds, org.projects[0].id, org.projects[0].key, "Sprint 1",
		"Entregar el flujo de alta de incidencias", "active", daysFromBase(-7), daysFromBase(7), nil)

	recs := make([]issueRec, 0, len(e2eIssues))
	for n, spec := range e2eIssues {
		proj := org.projects[spec.project]
		created := daysFromBase(-spec.createdDays)
		rec := issueRec{
			id:          issueID(n + 1),
			projectID:   proj.id,
			requester:   org.users[spec.requester],
			title:       spec.title,
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
	emitIssues(ds, recs)
	addE2EFilterAndNotifications(ds, org, recs)
	return ds
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
