package domain

import (
	"bytes"
	"encoding/json"
	"io"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// Gadget types.
const (
	GadgetAssignedToMe      = "assigned_to_me"
	GadgetCreatedByMe       = "created_by_me"
	GadgetFilterResults     = "filter_results"
	GadgetIssueStatistics   = "issue_statistics"
	GadgetPie               = "pie"
	GadgetTwoDimensional    = "two_dimensional"
	GadgetCreatedVsResolved = "created_vs_resolved"
	GadgetSprintBurndown    = "sprint_burndown"
	GadgetActivityStream    = "activity_stream"
	GadgetRecentProjects    = "recent_projects"
	GadgetFavorites         = "favorites"
	GadgetText              = "text"
	GadgetCalendarUpcoming  = "calendar_upcoming"
)

// Gadget types in catalog order.
var GadgetTypes = []string{
	GadgetAssignedToMe, GadgetCreatedByMe, GadgetFilterResults, GadgetIssueStatistics, GadgetPie, GadgetTwoDimensional,
	GadgetCreatedVsResolved, GadgetSprintBurndown, GadgetActivityStream, GadgetRecentProjects, GadgetFavorites,
	GadgetText, GadgetCalendarUpcoming,
}

// Config limits and the closed sets of values.
const (
	MaxRows         = 50
	DefaultRows     = 10
	MaxQueryLength  = 2000
	MaxTextLength   = 10000
	MinDays         = 7
	MaxDays         = 90
	DefaultDays     = 30
	MaxUpcomingDays = 60
	MaxStatGroups   = 20
)

// IssueColumns are the columns a result gadget can show.
var IssueColumns = []string{"key", "summary", "status", "priority", "type", "assignee", "due", "updated"}

// StatFields are the fields issue statistics can group by.
var StatFields = []string{"status", "assignee", "priority", "type", "label", "project"}

// FavoriteTypes are the entity types the favorites gadget can list.
var FavoriteTypes = []string{"project", "filter", "board", "dashboard", "issue"}

// ListConfig configures assigned_to_me and created_by_me.
type ListConfig struct {
	Limit       int  `json:"limit"`
	IncludeDone bool `json:"include_done"`
}

// FilterResultsConfig configures filter_results: a saved filter or a CoreQL text, never both.
type FilterResultsConfig struct {
	FilterID *uuid.UUID `json:"filter_id"`
	Query    string     `json:"query"`
	Columns  []string   `json:"columns"`
	Limit    int        `json:"limit"`
}

// StatsConfig configures issue_statistics and pie.
type StatsConfig struct {
	ProjectID   *uuid.UUID `json:"project_id"`
	GroupBy     string     `json:"group_by"`
	IncludeDone bool       `json:"include_done"`
	Limit       int        `json:"limit"`
}

// TwoDimensionalConfig configures two_dimensional.
type TwoDimensionalConfig struct {
	ProjectID   *uuid.UUID `json:"project_id"`
	XAxis       string     `json:"x_axis"`
	YAxis       string     `json:"y_axis"`
	IncludeDone bool       `json:"include_done"`
	Limit       int        `json:"limit"`
}

// CreatedVsResolvedConfig configures created_vs_resolved.
type CreatedVsResolvedConfig struct {
	ProjectID *uuid.UUID `json:"project_id"`
	Days      int        `json:"days"`
}

// BurndownConfig configures sprint_burndown. Without a sprint it follows the active sprint of the project, or
// the first active sprint of the viewer's projects.
type BurndownConfig struct {
	ProjectID *uuid.UUID `json:"project_id"`
	SprintID  *uuid.UUID `json:"sprint_id"`
}

// ActivityConfig configures activity_stream.
type ActivityConfig struct {
	ProjectID *uuid.UUID `json:"project_id"`
	Limit     int        `json:"limit"`
}

// RecentProjectsConfig configures recent_projects.
type RecentProjectsConfig struct {
	Limit int `json:"limit"`
}

// FavoritesConfig configures favorites. Empty Types means every type.
type FavoritesConfig struct {
	Types []string `json:"types"`
	Limit int      `json:"limit"`
}

// TextConfig configures text. The markdown is stored as written; the client sanitizes it when it renders it.
type TextConfig struct {
	Markdown string `json:"markdown"`
}

// CalendarUpcomingConfig configures calendar_upcoming.
type CalendarUpcomingConfig struct {
	Days  int `json:"days"`
	Limit int `json:"limit"`
}

// DefaultTitle is the title a gadget gets when the client sends none.
func DefaultTitle(gadgetType string) string {
	switch gadgetType {
	case GadgetAssignedToMe:
		return "Asignadas a mí"
	case GadgetCreatedByMe:
		return "Creadas por mí"
	case GadgetFilterResults:
		return "Resultados del filtro"
	case GadgetIssueStatistics:
		return "Estadísticas de incidencias"
	case GadgetPie:
		return "Gráfico circular"
	case GadgetTwoDimensional:
		return "Estadísticas en dos dimensiones"
	case GadgetCreatedVsResolved:
		return "Creadas frente a resueltas"
	case GadgetSprintBurndown:
		return "Burndown del sprint"
	case GadgetActivityStream:
		return "Actividad reciente"
	case GadgetRecentProjects:
		return "Proyectos recientes"
	case GadgetFavorites:
		return "Favoritos"
	case GadgetText:
		return "Texto"
	case GadgetCalendarUpcoming:
		return "Próximos eventos"
	}
	return ""
}

// KnownGadget reports whether t is in the catalog.
func KnownGadget(t string) bool {
	for _, k := range GadgetTypes {
		if k == t {
			return true
		}
	}
	return false
}

func configError(field, message string) *apperror.Error {
	return apperror.WithCode(apperror.KindValidation, CodeInvalidGadget, message).WithDetails(apperror.FieldError{Field: field, Message: message})
}

// decodeConfig decodes raw into dst rejecting unknown fields and trailing data. Empty or null raw is an empty
// object.
func decodeConfig(raw json.RawMessage, dst any) error {
	if len(raw) > MaxConfigBytes {
		return configError("", "la configuración es demasiado grande")
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		trimmed = []byte("{}")
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return configError("", "la configuración no es válida para este gadget: "+err.Error())
	}
	if _, err := dec.Token(); err != io.EOF {
		return configError("", "la configuración no es válida para este gadget")
	}
	return nil
}

func rows(field string, v *int) error {
	if *v == 0 {
		*v = DefaultRows
	}
	if *v < 1 || *v > MaxRows {
		return configError(field, "debe estar entre 1 y 50")
	}
	return nil
}

func oneOf(field, value string, allowed []string) error {
	for _, a := range allowed {
		if a == value {
			return nil
		}
	}
	return configError(field, "debe ser uno de: "+strings.Join(allowed, ", "))
}

// NormalizeConfig validates the configuration of a gadget type, fills the defaults and returns it re-encoded in
// a canonical form.
func NormalizeConfig(gadgetType string, raw json.RawMessage) (json.RawMessage, error) {
	var cfg any
	switch gadgetType {
	case GadgetAssignedToMe, GadgetCreatedByMe:
		c := ListConfig{}
		if err := decodeConfig(raw, &c); err != nil {
			return nil, err
		}
		if err := rows("limit", &c.Limit); err != nil {
			return nil, err
		}
		cfg = c
	case GadgetFilterResults:
		c := FilterResultsConfig{}
		if err := decodeConfig(raw, &c); err != nil {
			return nil, err
		}
		c.Query = strings.TrimSpace(c.Query)
		if (c.FilterID == nil) == (c.Query == "") {
			return nil, configError("filter_id", "indica un filtro guardado (filter_id) o una consulta (query), pero no ambos")
		}
		if utf8.RuneCountInString(c.Query) > MaxQueryLength {
			return nil, configError("query", "la consulta admite como máximo 2000 caracteres")
		}
		if c.Columns == nil {
			c.Columns = []string{"key", "summary", "status", "assignee", "updated"}
		}
		if len(c.Columns) == 0 || len(c.Columns) > len(IssueColumns) {
			return nil, configError("columns", "indica entre 1 y 8 columnas")
		}
		seen := map[string]bool{}
		for _, col := range c.Columns {
			if err := oneOf("columns", col, IssueColumns); err != nil {
				return nil, err
			}
			if seen[col] {
				return nil, configError("columns", "las columnas no pueden repetirse")
			}
			seen[col] = true
		}
		if err := rows("limit", &c.Limit); err != nil {
			return nil, err
		}
		cfg = c
	case GadgetIssueStatistics, GadgetPie:
		c := StatsConfig{}
		if err := decodeConfig(raw, &c); err != nil {
			return nil, err
		}
		if c.GroupBy == "" {
			c.GroupBy = "status"
		}
		if err := oneOf("group_by", c.GroupBy, StatFields); err != nil {
			return nil, err
		}
		if c.Limit == 0 {
			c.Limit = DefaultRows
		}
		if c.Limit < 1 || c.Limit > MaxStatGroups {
			return nil, configError("limit", "debe estar entre 1 y 20")
		}
		cfg = c
	case GadgetTwoDimensional:
		c := TwoDimensionalConfig{}
		if err := decodeConfig(raw, &c); err != nil {
			return nil, err
		}
		if c.XAxis == "" {
			c.XAxis = "status"
		}
		if c.YAxis == "" {
			c.YAxis = "assignee"
		}
		for field, v := range map[string]string{"x_axis": c.XAxis, "y_axis": c.YAxis} {
			if err := oneOf(field, v, StatFields); err != nil {
				return nil, err
			}
		}
		if c.XAxis == c.YAxis {
			return nil, configError("y_axis", "los dos ejes deben ser distintos")
		}
		if c.Limit == 0 {
			c.Limit = DefaultRows
		}
		if c.Limit < 1 || c.Limit > MaxStatGroups {
			return nil, configError("limit", "debe estar entre 1 y 20")
		}
		cfg = c
	case GadgetCreatedVsResolved:
		c := CreatedVsResolvedConfig{}
		if err := decodeConfig(raw, &c); err != nil {
			return nil, err
		}
		if c.Days == 0 {
			c.Days = DefaultDays
		}
		if c.Days < MinDays || c.Days > MaxDays {
			return nil, configError("days", "debe estar entre 7 y 90")
		}
		cfg = c
	case GadgetSprintBurndown:
		c := BurndownConfig{}
		if err := decodeConfig(raw, &c); err != nil {
			return nil, err
		}
		cfg = c
	case GadgetActivityStream:
		c := ActivityConfig{}
		if err := decodeConfig(raw, &c); err != nil {
			return nil, err
		}
		if err := rows("limit", &c.Limit); err != nil {
			return nil, err
		}
		cfg = c
	case GadgetRecentProjects:
		c := RecentProjectsConfig{}
		if err := decodeConfig(raw, &c); err != nil {
			return nil, err
		}
		if err := rows("limit", &c.Limit); err != nil {
			return nil, err
		}
		cfg = c
	case GadgetFavorites:
		c := FavoritesConfig{}
		if err := decodeConfig(raw, &c); err != nil {
			return nil, err
		}
		for _, t := range c.Types {
			if err := oneOf("types", t, FavoriteTypes); err != nil {
				return nil, err
			}
		}
		c.Types = uniqueSorted(c.Types)
		if err := rows("limit", &c.Limit); err != nil {
			return nil, err
		}
		cfg = c
	case GadgetText:
		c := TextConfig{}
		if err := decodeConfig(raw, &c); err != nil {
			return nil, err
		}
		if utf8.RuneCountInString(c.Markdown) > MaxTextLength {
			return nil, configError("markdown", "el texto admite como máximo 10000 caracteres")
		}
		cfg = c
	case GadgetCalendarUpcoming:
		c := CalendarUpcomingConfig{}
		if err := decodeConfig(raw, &c); err != nil {
			return nil, err
		}
		if c.Days == 0 {
			c.Days = 14
		}
		if c.Days < 1 || c.Days > MaxUpcomingDays {
			return nil, configError("days", "debe estar entre 1 y 60")
		}
		if err := rows("limit", &c.Limit); err != nil {
			return nil, err
		}
		cfg = c
	default:
		return nil, apperror.WithCode(apperror.KindValidation, CodeInvalidGadget, "tipo de gadget desconocido: "+gadgetType).
			WithDetails(apperror.FieldError{Field: "type", Message: "tipo de gadget desconocido"})
	}
	out, err := json.Marshal(cfg)
	if err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to encode the gadget configuration", err)
	}
	return out, nil
}

func uniqueSorted(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	set := map[string]bool{}
	for _, s := range in {
		set[s] = true
	}
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
