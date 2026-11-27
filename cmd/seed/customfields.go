package main

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// seedCustomField describes one organization-level field of the e2e dataset. Options use stable ids so the
// issue values can reference them.
type seedCustomField struct {
	key, name, description, fieldType string
	options                           []seedOption
}

type seedOption struct {
	id, label, color string
}

var e2eCustomFields = []seedCustomField{
	{
		key: "severidad", name: "Severidad", description: "Impacto del problema para el negocio", fieldType: "select",
		options: []seedOption{{id: "alta", label: "Alta", color: "red"}, {id: "media", label: "Media", color: "amber"}, {id: "baja", label: "Baja", color: "blue"}},
	},
	{key: "fecha-limite-cliente", name: "Fecha límite cliente", description: "Fecha comprometida con el cliente", fieldType: "date"},
	{key: "enlace-referencia", name: "Enlace de referencia", description: "Documentación o ticket externo relacionado", fieldType: "url"},
}

// addE2ECustomFields adds the e2e custom fields with their contexts and sets values on some issues. Only fields
// whose context covers the project of an issue get a value, as the application would accept it. Severity and
// the client date apply to the first project, the reference link to both.
func addE2ECustomFields(ds *dataset, org orgModel, recs []issueRec) {
	org0, org1 := org.projects[0].id, org.projects[1].id
	fields := ds.ignoreDuplicates("custom_fields", "id", "organization_id", "name", "description", "field_type",
		"options", "settings", "position", "is_system", "created_at", "updated_at")
	contexts := ds.ignoreDuplicates("custom_field_contexts", "id", "field_id", "project_id", "issue_type_id",
		"required", "default_value", "position", "created_at", "updated_at")

	ids := map[string]uuid.UUID{}
	for position, field := range e2eCustomFields {
		id := idFor("custom_field", org.orgID.String(), field.key)
		ids[field.key] = id
		options := make([]map[string]any, 0, len(field.options))
		for index, option := range field.options {
			options = append(options, map[string]any{"id": option.id, "label": option.label, "color": option.color, "position": index, "archived": false})
		}
		raw, err := json.Marshal(options)
		if err != nil {
			panic(err)
		}
		fields.add(id, org.orgID, field.name, field.description, field.fieldType, json.RawMessage(raw),
			json.RawMessage("{}"), position, false, orgCreated, orgCreated)
	}

	contextRow := func(field string, project uuid.UUID, position int) {
		contexts.add(idFor("custom_field_context", org.orgID.String(), field, project.String()), ids[field], project, nil,
			false, nil, position, orgCreated, orgCreated)
	}
	contextRow("severidad", org0, 0)
	contextRow("fecha-limite-cliente", org0, 1)
	contextRow("enlace-referencia", org0, 2)
	contextRow("enlace-referencia", org1, 2)

	severities := []string{"alta", "media", "baja"}
	for i := range recs {
		rec := &recs[i]
		if rec.projectID == org0 {
			if i%3 == 0 {
				rec.custom = setValue(rec.custom, ids["severidad"].String(), severities[(i/3)%len(severities)])
			}
			if i%4 == 0 {
				due := rec.created.AddDate(0, 0, 14).Format(time.DateOnly)
				rec.custom = setValue(rec.custom, ids["fecha-limite-cliente"].String(), due)
			}
		}
		if i%5 == 0 {
			rec.custom = setValue(rec.custom, ids["enlace-referencia"].String(), "https://example.com/referencias/"+rec.id.String())
		}
	}
}

func setValue(values map[string]any, key string, value any) map[string]any {
	if values == nil {
		values = map[string]any{}
	}
	values[key] = value
	return values
}
