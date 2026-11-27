package main

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"strconv"
	"time"

	"github.com/google/uuid"
)

var issueSummaries = [...]string{
	"Error 500 al exportar reportes en CSV",
	"Lentitud en el listado de incidencias con filtros",
	"Renovar certificado TLS del dominio de API",
	"Configurar alertas de uso de CPU en produccion",
	"Permisos incorrectos en carpeta compartida",
	"Migrar backups a almacenamiento externo",
	"Agregar paginacion al endpoint de comentarios",
	"Notificaciones por correo duplicadas",
	"Revisar accesos de cuentas inactivas",
	"Documentar flujo de autenticacion JWT",
	"Crear dashboard de SLA por equipo",
	"Timeout al subir adjuntos mayores a 5 MB",
	"Mejorar contraste del modo oscuro en badges",
	"Auditar cambios en workflows de proyecto",
	"Actualizar dependencias con vulnerabilidades criticas",
	"Corregir orden del tablero al mover tarjetas",
	"Politica de rotacion de contrasenas",
	"Integrar login con SSO corporativo",
	"Limpiar labels duplicados en proyecto",
	"Error al vincular issues bloqueantes",
	"Backup nocturno de PostgreSQL falla los domingos",
	"Ajustar limite WIP de la columna En progreso",
	"Crear plantilla de reporte mensual",
	"Falla intermitente del health check de la API",
}

var issueDescriptions = [...]string{
	"Se reporta desde produccion y afecta a varios usuarios del equipo.",
	"Pendiente de analisis tecnico. Se requiere reproducir el caso en staging.",
	"Solicitud creada desde el portal interno; prioridad acordada en la reunion semanal.",
	"Seguimiento de deuda tecnica registrada durante la revision del sprint.",
}

var commentBodies = [...]string{
	"Revisado, necesito mas detalles del entorno.",
	"Ya reproduje el problema en staging.",
	"Subo un fix en la siguiente iteracion.",
	"Esto bloquea la release de la proxima semana.",
	"Coordinemos con infraestructura antes de tocar la base.",
	"Cerrado como duplicado, seguimos en la otra incidencia.",
	"Pendiente de validacion del cliente.",
	"Gracias, ya quedo funcionando en mi entorno.",
}

var storyPointValues = [...]float64{1, 2, 3, 5, 8, 13}

// commentRec is a comment attached to an issue.
type commentRec struct {
	author uuid.UUID
	body   string
	at     time.Time
}

// issueRec is one generated issue with its relations. It is plain data, so the
// generators can be tested without a database.
type issueRec struct {
	orgID                        uuid.UUID
	id, projectID, reporter      uuid.UUID
	assignee, parent, sprint     *uuid.UUID
	summary, description, status string
	priority, itype, rank        string
	due                          *time.Time
	points                       *float64
	created, updated             time.Time
	closed                       *time.Time
	labels, components, watchers []uuid.UUID
	comments                     []commentRec
	custom                       map[string]any
}

func priorityFor(r float64) string {
	switch {
	case r < 0.15:
		return "critical"
	case r < 0.45:
		return "high"
	case r < 0.80:
		return "medium"
	default:
		return "low"
	}
}

// generateIssues builds n issues spread across the projects of org. The
// distribution follows the former demo SQL: the first four issues are epics,
// then bugs, stories, subtasks and tasks by number, with statuses weighted
// towards open work. Labels, components, watchers and comments are drawn from
// the same generator so one scenario always yields the same result.
func generateIssues(rng *rand.Rand, org orgModel, n int) []issueRec {
	requesters := org.requesters
	agents := org.agents
	recs := make([]issueRec, 0, n)

	for i := 1; i <= n; i++ {
		var draws [8]float64
		for k := range draws {
			draws[k] = rng.Float64()
		}
		rStatus, rPriority, rPeople, rText := draws[0], draws[1], draws[2], draws[3]
		rAssign, rCreated, rPlan, rPick := draws[4], draws[5], draws[6], draws[7]

		p := (i - 1) % len(org.projects)
		proj := org.projects[p]

		var itype string
		switch {
		case i <= 4:
			itype = "epic"
		case i%6 == 0:
			itype = "bug"
		case i%5 == 0:
			itype = "story"
		case i%4 == 3:
			itype = "subtask"
		default:
			itype = "task"
		}

		var status string
		switch {
		case i <= 4:
			status = "in_progress"
			if rStatus < 0.5 {
				status = "open"
			}
		case rStatus < 0.25:
			status = "open"
		case rStatus < 0.50:
			status = "in_progress"
		case rStatus < 0.72:
			status = "resolved"
		default:
			status = "closed"
		}

		created := daysFromBase(-(int(rCreated*150) + 1))
		rec := issueRec{
			orgID:       org.orgID,
			id:          issueID(i),
			projectID:   proj.id,
			reporter:    requesters[int(rPeople*float64(len(requesters)))],
			summary:     issueSummaries[int(rPick*float64(len(issueSummaries)))] + " #" + strconv.Itoa(i),
			description: "Caso de prueba #" + strconv.Itoa(i) + ". " + issueDescriptions[int(rText*float64(len(issueDescriptions)))],
			status:      status,
			priority:    priorityFor(rPriority),
			itype:       itype,
			rank:        fmt.Sprintf("r%05d", i),
			created:     created,
		}
		if status == "resolved" || status == "closed" {
			closed := created.Add(time.Duration(1+int(rAssign*10)) * day)
			if closed.After(baseTime) {
				closed = baseTime
			}
			rec.closed = &closed
			rec.updated = closed
		} else {
			rec.updated = minTime(baseTime, created.Add(3*day))
		}
		if !(status == "open" && rText < 0.6) {
			rec.assignee = ptr(agents[int(rAssign*float64(len(agents)))])
		}
		if rPlan < 0.7 {
			rec.due = ptr(daysFromBase(int(rPeople*60) - 20))
		}
		if itype != "epic" && rPlan < 0.8 {
			rec.points = ptr(storyPointValues[int(rPick*float64(len(storyPointValues)))])
		}
		if itype == "subtask" {
			rec.parent = ptr(issueID(p + 1))
		}

		// Labels: about 70% of issues get two distinct labels of their project.
		if rng.Float64() < 0.7 && len(proj.labels) >= 2 {
			a := rng.IntN(len(proj.labels))
			b := rng.IntN(len(proj.labels) - 1)
			if b >= a {
				b++
			}
			rec.labels = []uuid.UUID{proj.labels[a], proj.labels[b]}
		}
		// Components: about 40% of issues get one component of their project.
		if rng.Float64() < 0.4 && len(proj.components) > 0 {
			rec.components = []uuid.UUID{proj.components[rng.IntN(len(proj.components))]}
		}
		for k := 0; k < 2; k++ {
			rec.watchers = append(rec.watchers, org.users[rng.IntN(len(org.users))])
		}
		for k := 0; k < 3; k++ {
			if rng.Float64() < 0.5 {
				author := org.users[rng.IntN(len(org.users))]
				body := commentBodies[rng.IntN(len(commentBodies))]
				at := created.Add(time.Duration(rng.Float64() * float64(baseTime.Sub(created))))
				rec.comments = append(rec.comments, commentRec{author: author, body: body, at: at})
			}
		}
		recs = append(recs, rec)
	}
	return recs
}

func minTime(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}

// syncIssueCounters sets next_issue_number of every project to one more than its seeded issues,
// so issues created after seeding continue the numbering without collisions.
func syncIssueCounters(ds *dataset, recs []issueRec) {
	projects := ds.byName["projects"]
	idIdx, counterIdx := columnIndex(projects, "id"), columnIndex(projects, "next_issue_number")
	counts := map[uuid.UUID]int{}
	for _, r := range recs {
		counts[r.projectID]++
	}
	for _, row := range projects.rows {
		row[counterIdx] = counts[row[idIdx].(uuid.UUID)] + 1
	}
}

// columnIndex returns the position of a column of a table, panicking when it is missing.
func columnIndex(t *table, column string) int {
	for i, c := range t.cols {
		if c == column {
			return i
		}
	}
	panic(fmt.Sprintf("seed: table %s has no column %s", t.name, column))
}

// emitIssues writes issues and their relations. Parents must come before
// children in recs, which generateIssues guarantees.
func emitIssues(ds *dataset, recs []issueRec) {
	issues := ds.table("issues", "id", "project_id", "summary", "description", "status", "priority",
		"reporter_id", "assignee_id", "issue_type", "parent_id", "due_date", "story_points",
		"rank", "closed_at", "created_at", "updated_at", "sprint_id", "key_number",
		"issue_type_id", "priority_id", "resolution_id", "resolved_at", "custom")
	issueLabels := ds.ignoreDuplicates("issue_labels", "issue_id", "label_id")
	issueComponents := ds.ignoreDuplicates("issue_components", "issue_id", "component_id")
	watchers := ds.ignoreDuplicates("watchers", "issue_id", "user_id", "created_at")
	activities := ds.table("activities", "id", "issue_id", "actor_id", "action", "field_name", "old_value", "new_value", "created_at")
	comments := ds.table("comments", "id", "issue_id", "author_id", "body", "created_at", "updated_at")

	keyNumbers := map[uuid.UUID]int{}
	for _, r := range recs {
		var resolutionID any
		var resolvedAt any
		if r.status == "resolved" || r.status == "closed" {
			resolutionID = catalogRef{org: r.orgID, table: "resolutions"}
			resolvedAt = r.updated
			if r.closed != nil {
				resolvedAt = *r.closed
			}
		}
		keyNumbers[r.projectID]++
		issues.add(r.id, r.projectID, r.summary, r.description, r.status, r.priority,
			r.reporter, opt(r.assignee), r.itype, opt(r.parent), opt(r.due), opt(r.points),
			r.rank, opt(r.closed), r.created, r.updated, opt(r.sprint), keyNumbers[r.projectID],
			catalogRef{org: r.orgID, table: "issue_types", legacy: r.itype},
			catalogRef{org: r.orgID, table: "priorities", legacy: r.priority}, resolutionID, resolvedAt, customJSON(r.custom))
		for _, l := range r.labels {
			issueLabels.add(r.id, l)
		}
		for _, c := range r.components {
			issueComponents.add(r.id, c)
		}
		for _, w := range r.watchers {
			watchers.add(r.id, w, r.created)
		}
		activities.add(idFor("activity", r.id.String(), "created"), r.id, r.reporter, "created", nil, nil, nil, r.created)
		if r.status != "open" {
			actor := r.reporter
			if r.assignee != nil {
				actor = *r.assignee
			}
			activities.add(idFor("activity", r.id.String(), "status"), r.id, actor, "status_changed",
				"status", "open", r.status, r.updated)
		}
		for k, c := range r.comments {
			comments.add(idFor("comment", r.id.String(), strconv.Itoa(k)), r.id, c.author, c.body, c.at, c.at)
		}
	}
}

// customJSON encodes the custom field values of an issue. Issues without values store an empty object.
func customJSON(values map[string]any) json.RawMessage {
	if len(values) == 0 {
		return json.RawMessage("{}")
	}
	raw, err := json.Marshal(values)
	if err != nil {
		panic(fmt.Sprintf("seed: encode custom values: %v", err))
	}
	return raw
}
