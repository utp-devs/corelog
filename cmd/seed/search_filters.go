package main

import (
	"github.com/google/uuid"
)

// searchFilter is a saved CoreQL filter of the seed. Shares and subscriptions are optional.
type searchFilter struct {
	owner       uuid.UUID
	name        string
	description string
	query       string
	columns     string // JSON array
	sort        string
	shareOrg    bool        // shared with the whole organization (view)
	shareUsers  []uuid.UUID // shared with these users (edit)
	subscribe   []uuid.UUID // users subscribed with a daily 08:00 digest
}

// addSearchFilters registers the filters, their shares and subscriptions. Every id derives from the owner and
// the name, so running the seed on an empty database always produces the same rows.
func addSearchFilters(ds *dataset, orgID uuid.UUID, filters []searchFilter) {
	rows := ds.table("filters", "id", "owner_id", "organization_id", "name", "description", "query_text", "columns", "sort", "created_at", "updated_at")
	shares := ds.table("filter_shares", "filter_id", "share_type", "target_id", "permission", "created_at")
	subs := ds.table("filter_subscriptions", "id", "filter_id", "user_id", "schedule", "created_at")
	at := daysFromBase(-14)
	for _, f := range filters {
		id := idFor("search_filter", f.owner.String(), f.name)
		columns := f.columns
		if columns == "" {
			columns = `["key","summary","status","assignee","updated"]`
		}
		rows.add(id, f.owner, orgID, f.name, f.description, f.query, columns, f.sort, at, at)
		if f.shareOrg {
			shares.add(id, "organization", orgID, "view", at)
		}
		for _, u := range f.shareUsers {
			shares.add(id, "user", u, "edit", at)
		}
		for _, u := range f.subscribe {
			subs.add(idFor("search_subscription", id.String(), u.String()), id, u,
				`{"frequency":"daily","hour":8,"weekday":0,"timezone":"UTC"}`, at)
		}
	}
}

func addDemoSearchFilters(ds *dataset, orgID uuid.UUID) {
	admin := userID("admin@corelog.local")
	agent := userID("agent@corelog.local")
	addSearchFilters(ds, orgID, []searchFilter{
		{owner: admin, name: "Mis incidencias abiertas", description: "Lo que tengo asignado y sigue sin terminar",
			query: `assignee = currentUser() AND statusCategory != done`, sort: "updated DESC"},
		{owner: admin, name: "Vencidas", description: "Incidencias con fecha límite pasada",
			query: `due < startOfDay() AND statusCategory != done`, sort: "due ASC", shareOrg: true, subscribe: []uuid.UUID{admin}},
		{owner: admin, name: "Errores críticos sin resolver", description: "Errores de prioridad máxima o alta",
			query: `type = "Error" AND priority IN ("Máxima", "Alta") AND resolution IS EMPTY`, sort: "priority ASC, created DESC"},
		{owner: admin, name: "Actualizadas esta semana",
			query: `updated >= startOfWeek()`, sort: "updated DESC", shareOrg: true},
		{owner: agent, name: "Sin asignar por hacer", description: "Cola para repartir",
			query: `assignee IS EMPTY AND statusCategory = todo`, sort: "created ASC", shareUsers: []uuid.UUID{admin}},
	})
}

func addE2ESearchFilters(ds *dataset, orgID, admin uuid.UUID) {
	addSearchFilters(ds, orgID, []searchFilter{
		{owner: admin, name: "Pendientes de la organización", description: "Todo lo que no está hecho",
			query: `statusCategory != done`, sort: "created DESC", shareOrg: true},
		{owner: admin, name: "Mis pendientes", query: `assignee = currentUser() AND statusCategory != done`},
	})
}
