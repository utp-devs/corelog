package usecase

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/search/application/port"
	"github.com/jhonsferg/corelog/internal/search/domain/coreql"
)

// customCatalog exposes the custom fields a viewer can query as a CoreQL field catalog. A field is reachable
// as cf[<name>], as cf[<id>] and, when the name has no whitespace, by its bare name.
type customCatalog struct {
	defs  map[string]coreql.FieldDef
	names []string
	byRef map[string]port.CustomFieldDef
}

func newCustomCatalog(fields []port.CustomFieldDef) *customCatalog {
	c := &customCatalog{defs: map[string]coreql.FieldDef{}, byRef: map[string]port.CustomFieldDef{}}
	merged := map[string]*port.CustomFieldDef{}
	var order []string
	for _, f := range fields {
		key := strings.ToLower(strings.TrimSpace(f.Name))
		if key == "" || len(f.IDs) == 0 {
			continue
		}
		if existing, ok := merged[key]; ok {
			// The same name in another organization contributes its ids when the type agrees.
			if existing.Kind == f.Kind {
				existing.IDs = append(existing.IDs, f.IDs...)
			}
			continue
		}
		cp := f
		cp.IDs = append([]uuid.UUID(nil), f.IDs...)
		merged[key] = &cp
		order = append(order, key)
	}
	for _, key := range order {
		def := *merged[key]
		fd := customFieldDef(def)
		refs := []string{"cf[" + key + "]"}
		if !strings.ContainsAny(key, " \t") && !strings.ContainsAny(key, "()[],\"'") {
			refs = append(refs, key)
		}
		for _, id := range def.IDs {
			refs = append(refs, "cf["+strings.ToLower(id.String())+"]")
		}
		for _, ref := range refs {
			if _, dup := c.defs[ref]; dup {
				continue
			}
			c.defs[ref] = fd
			c.byRef[ref] = def
		}
		c.names = append(c.names, fmt.Sprintf("cf[%s]", def.Name))
	}
	return c
}

// customFieldDef maps a custom field type to a CoreQL field definition.
func customFieldDef(def port.CustomFieldDef) coreql.FieldDef {
	fd := coreql.FieldDef{Name: "cf[" + def.Name + "]"}
	switch def.Kind {
	case "number":
		fd.Type, fd.Sortable = coreql.TypeNumber, true
	case "date":
		fd.Type, fd.Sortable = coreql.TypeDate, true
	case "datetime":
		fd.Type, fd.Sortable = coreql.TypeDateTime, true
	case "select":
		fd.Type, fd.Sortable = coreql.TypeEnum, true
	case "multiselect":
		fd.Type, fd.Multi = coreql.TypeEnum, true
	case "checkbox":
		fd.Type, fd.Sortable = coreql.TypeBool, true
	case "user":
		fd.Type = coreql.TypeUser
	case "labels":
		fd.Type, fd.Multi = coreql.TypeLabels, true
	default:
		fd.Type, fd.Sortable = coreql.TypeText, true
	}
	return fd
}

// Lookup resolves a custom field reference.
func (c *customCatalog) Lookup(name string) (coreql.FieldDef, bool) {
	d, ok := c.defs[port.CustomRef(name)]
	return d, ok
}

// Names lists the cf[<name>] spelling of every field.
func (c *customCatalog) Names() []string { return append([]string(nil), c.names...) }

// refs returns the reference map handed to the SQL builder.
func (c *customCatalog) refs() map[string]port.CustomFieldDef { return c.byRef }
