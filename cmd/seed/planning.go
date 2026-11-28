package main

import (
	"time"

	"github.com/google/uuid"
)

// planningGadget is one gadget of a seeded dashboard; config is JSON in the canonical form the API stores.
type planningGadget struct {
	kind, title, config string
	column              int
}

// addPlanningSeed registers a shared dashboard and calendar events (a weekly series with a cancelled
// occurrence, an all-day event and a project event with a guest). Every id derives from natural keys, so a seed
// on an empty database is reproducible.
func addPlanningSeed(ds *dataset, orgID, owner, guest, project uuid.UUID) {
	addSeedDashboard(ds, orgID, owner, "Resumen del equipo", "Estado de las incidencias y del sprint para todo el equipo", "2-1", []planningGadget{
		{"issue_statistics", "Incidencias por estado", `{"project_id":null,"group_by":"status","include_done":true,"limit":10}`, 0},
		{"created_vs_resolved", "Creadas frente a resueltas", `{"project_id":null,"days":30}`, 0},
		{"filter_results", "Vencidas", `{"filter_id":null,"query":"due < startOfDay() AND statusCategory != done","columns":["key","summary","assignee","due"],"limit":10}`, 0},
		{"sprint_burndown", "Burndown del sprint activo", `{"project_id":null,"sprint_id":null}`, 1},
		{"pie", "Prioridades abiertas", `{"project_id":null,"group_by":"priority","include_done":false,"limit":10}`, 1},
		{"text", "Notas", `{"markdown":"**Recordatorio**: la daily es a las 09:30."}`, 1},
	}, true)

	events := ds.table("calendar_events", "id", "owner_id", "organization_id", "project_id", "title", "description", "location",
		"starts_at", "ends_at", "all_day", "timezone", "recurrence_rule", "recurrence_exceptions", "visibility", "color", "created_at", "updated_at")
	attendees := ds.table("calendar_event_attendees", "event_id", "user_id", "response", "created_at")
	at := daysFromBase(-7)

	daily := idFor("calendar_event", owner.String(), "Daily del equipo")
	// Monday 2026-11-30 09:30 in Madrid (UTC+1 in winter), every weekday; the Tuesday holiday is cancelled.
	start := time.Date(2026, 11, 30, 8, 30, 0, 0, time.UTC)
	cancelled := time.Date(2026, 12, 8, 8, 30, 0, 0, time.UTC)
	events.add(daily, owner, orgID, nil, "Daily del equipo", "Seguimiento diario del equipo", "Sala Atenea",
		start, start.Add(15*time.Minute), false, "Europe/Madrid", "FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR",
		`[{"original_start":"`+cancelled.Format(time.RFC3339)+`","cancelled":true}]`, "organization", "teal", at, at)
	attendees.add(daily, guest, "accepted", at)

	holiday := idFor("calendar_event", owner.String(), "Cierre de año")
	day0 := time.Date(2026, 12, 24, 0, 0, 0, 0, time.UTC)
	events.add(holiday, owner, orgID, nil, "Cierre de año", "", "", day0, day0.AddDate(0, 0, 1), true, "UTC", nil, `[]`, "organization", "red", at, at)

	planning := idFor("calendar_event", owner.String(), "Planificación del sprint")
	pstart := time.Date(2026, 12, 7, 10, 0, 0, 0, time.UTC)
	events.add(planning, owner, orgID, project, "Planificación del sprint", "Revisión del backlog y compromiso", "Videollamada",
		pstart, pstart.Add(90*time.Minute), false, "UTC", "FREQ=WEEKLY;INTERVAL=2;COUNT=6", `[]`, "project", "indigo", at, at)
	attendees.add(planning, guest, "needs_action", at)
}

// addSeedDashboard stores a dashboard with its gadgets and, when shareOrg is set, shares it with the organization.
func addSeedDashboard(ds *dataset, orgID, owner uuid.UUID, name, description, layout string, gadgets []planningGadget, shareOrg bool) {
	dashboards := ds.table("dashboards", "id", "owner_id", "organization_id", "name", "description", "layout", "is_default", "created_at", "updated_at")
	rows := ds.table("dashboard_gadgets", "id", "dashboard_id", "type", "title", "config", "position", "column_index", "created_at", "updated_at")
	shares := ds.table("dashboard_shares", "dashboard_id", "share_type", "target_id", "permission", "created_at")
	at := daysFromBase(-10)
	id := idFor("dashboard", owner.String(), name)
	dashboards.add(id, owner, orgID, name, description, `{"template":"`+layout+`"}`, false, at, at)
	next := map[int]int{}
	for _, g := range gadgets {
		rows.add(idFor("dashboard_gadget", id.String(), g.title), id, g.kind, g.title, g.config, next[g.column], g.column, at, at)
		next[g.column]++
	}
	if shareOrg {
		shares.add(id, "organization", orgID, "view", at)
	}
}
