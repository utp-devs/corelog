package coreql

import "strings"

// FieldType is the value type of a field. It decides allowed operators and
// value kinds.
type FieldType int

const (
	TypeText FieldType = iota + 1
	TypeNumber
	TypeDate
	TypeDateTime
	TypeEnum
	TypeUser
	TypeBool
	TypeLabels
	TypeProject
	TypeIssueType
	TypeStatus
	TypePriority
	TypeResolution
	TypeSprint
	TypeVersion
	TypeComponent
	TypeIssue
)

var fieldTypeNames = map[FieldType]string{
	TypeText:       "text",
	TypeNumber:     "number",
	TypeDate:       "date",
	TypeDateTime:   "datetime",
	TypeEnum:       "enum",
	TypeUser:       "user",
	TypeBool:       "bool",
	TypeLabels:     "labels",
	TypeProject:    "project",
	TypeIssueType:  "issuetype",
	TypeStatus:     "status",
	TypePriority:   "priority",
	TypeResolution: "resolution",
	TypeSprint:     "sprint",
	TypeVersion:    "version",
	TypeComponent:  "component",
	TypeIssue:      "issue",
}

// String returns the type name used in messages and suggestions.
func (t FieldType) String() string {
	if s, ok := fieldTypeNames[t]; ok {
		return s
	}
	return "unknown"
}

// FieldDef describes a field available to queries.
type FieldDef struct {
	Name      string
	Type      FieldType
	Operators []Op // nil means the defaults of Type
	Multi     bool // multi-valued (for example watcher or labels)
	Sortable  bool // allowed in ORDER BY
}

// allowedOps returns the operators the field accepts.
func (d FieldDef) allowedOps() []Op {
	if d.Operators != nil {
		return d.Operators
	}
	return defaultOps(d.Type)
}

func defaultOps(t FieldType) []Op {
	switch t {
	case TypeText:
		return []Op{OpEq, OpNe, OpContains, OpNotContains}
	case TypeNumber, TypeDate, TypeDateTime:
		return []Op{OpEq, OpNe, OpGt, OpGe, OpLt, OpLe}
	}
	return []Op{OpEq, OpNe}
}

// FieldCatalog resolves field names. Lookup is case-insensitive.
type FieldCatalog interface {
	Lookup(name string) (FieldDef, bool)
	Names() []string
}

// MapCatalog is a FieldCatalog backed by a fixed list of definitions.
type MapCatalog struct {
	defs  map[string]FieldDef
	names []string
}

// NewMapCatalog builds a catalog from definitions. Later duplicates are ignored.
func NewMapCatalog(defs ...FieldDef) *MapCatalog {
	c := &MapCatalog{defs: make(map[string]FieldDef, len(defs))}
	for _, d := range defs {
		key := strings.ToLower(d.Name)
		if _, dup := c.defs[key]; dup {
			continue
		}
		c.defs[key] = d
		c.names = append(c.names, d.Name)
	}
	return c
}

// Lookup returns the definition for name, ignoring case.
func (c *MapCatalog) Lookup(name string) (FieldDef, bool) {
	d, ok := c.defs[strings.ToLower(name)]
	return d, ok
}

// Names returns the canonical field names in declaration order.
func (c *MapCatalog) Names() []string {
	return append([]string(nil), c.names...)
}

// CompositeCatalog layers Top (for example custom fields cf[...]) over Base.
// Top wins when both define the same name.
type CompositeCatalog struct {
	Top  FieldCatalog
	Base FieldCatalog
}

// Lookup checks Top first, then Base.
func (c CompositeCatalog) Lookup(name string) (FieldDef, bool) {
	if c.Top != nil {
		if d, ok := c.Top.Lookup(name); ok {
			return d, true
		}
	}
	if c.Base != nil {
		return c.Base.Lookup(name)
	}
	return FieldDef{}, false
}

// Names returns the union of both catalogs, Top first, without duplicates.
func (c CompositeCatalog) Names() []string {
	seen := map[string]bool{}
	var out []string
	for _, cat := range []FieldCatalog{c.Top, c.Base} {
		if cat == nil {
			continue
		}
		for _, n := range cat.Names() {
			key := strings.ToLower(n)
			if !seen[key] {
				seen[key] = true
				out = append(out, n)
			}
		}
	}
	return out
}

var systemCatalog = NewMapCatalog(systemFields()...)

// SystemCatalog returns the catalog of built-in CoreQL system fields.
func SystemCatalog() FieldCatalog { return systemCatalog }

func systemFields() []FieldDef {
	return []FieldDef{
		{Name: "project", Type: TypeProject, Sortable: true},
		{Name: "key", Type: TypeText, Sortable: true},
		{Name: "issuetype", Type: TypeIssueType, Sortable: true},
		{Name: "type", Type: TypeIssueType, Sortable: true},
		{Name: "status", Type: TypeStatus, Sortable: true},
		{Name: "statusCategory", Type: TypeEnum, Sortable: true},
		{Name: "resolution", Type: TypeResolution, Sortable: true},
		{Name: "priority", Type: TypePriority, Sortable: true},
		{Name: "assignee", Type: TypeUser, Sortable: true},
		{Name: "reporter", Type: TypeUser, Sortable: true},
		{Name: "creator", Type: TypeUser, Sortable: true},
		{Name: "labels", Type: TypeLabels, Multi: true},
		{Name: "component", Type: TypeComponent, Multi: true},
		{Name: "fixVersion", Type: TypeVersion, Multi: true},
		{Name: "affectedVersion", Type: TypeVersion, Multi: true},
		{Name: "sprint", Type: TypeSprint, Multi: true},
		{Name: "epic", Type: TypeIssue},
		{Name: "parent", Type: TypeIssue},
		{Name: "summary", Type: TypeText, Sortable: true},
		{Name: "description", Type: TypeText},
		{Name: "comment", Type: TypeText},
		{Name: "text", Type: TypeText},
		{Name: "created", Type: TypeDateTime, Sortable: true},
		{Name: "updated", Type: TypeDateTime, Sortable: true},
		{Name: "resolved", Type: TypeDateTime, Sortable: true},
		{Name: "due", Type: TypeDate, Sortable: true},
		{Name: "start", Type: TypeDate, Sortable: true},
		{Name: "votes", Type: TypeNumber, Sortable: true},
		{Name: "watcher", Type: TypeUser, Multi: true},
		{Name: "flagged", Type: TypeBool},
		{Name: "storyPoints", Type: TypeNumber, Sortable: true},
		{Name: "originalEstimate", Type: TypeNumber, Sortable: true},
		{Name: "remainingEstimate", Type: TypeNumber, Sortable: true},
		{Name: "timeSpent", Type: TypeNumber, Sortable: true},
	}
}
