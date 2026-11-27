package domain

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// Feed kinds: a comment is one entry, and the changes of one patch form one history entry.
const (
	FeedKindComment = "comment"
	FeedKindHistory = "history"
)

// Feed filters accepted by the activity endpoint.
const (
	ActivityFilterAll      = "all"
	ActivityFilterComments = "comments"
	ActivityFilterHistory  = "history"
)

// Page size bounds of the activity feed.
const (
	DefaultActivityLimit = 25
	MaxActivityLimit     = 100
)

var (
	ErrInvalidActivityFilter = apperror.WithCode(apperror.KindValidation, "issue.invalid_activity_filter", "el filtro de actividad debe ser all, comments o history")
	ErrInvalidActivityLimit  = apperror.WithCode(apperror.KindValidation, "issue.invalid_activity_limit", "el limite debe estar entre 1 y 100")
	ErrInvalidActivityCursor = apperror.WithCode(apperror.KindValidation, "issue.invalid_activity_cursor", "el cursor de actividad no es valido")
)

// fieldLabels maps the internal field names of changes to the Spanish names shown in the feed.
var fieldLabels = map[string]string{
	"fix_versions":               "Versiones de corrección",
	"affects_versions":           "Versiones afectadas",
	"components":                 "Componentes",
	"summary":                    "Resumen",
	"description":                "Descripción",
	"status":                     "Estado",
	"priority":                   "Prioridad",
	"assignee":                   "Responsable",
	"assignee_id":                "Responsable",
	"reporter":                   "Informador",
	"issue_type":                 "Tipo",
	"labels":                     "Etiquetas",
	"due_date":                   "Fecha límite",
	"start_date":                 "Fecha de inicio",
	"story_points":               "Puntos de historia",
	"original_estimate":          "Estimación original",
	"remaining_estimate":         "Estimación restante",
	"flagged":                    "Marcada",
	"parent":                     "Padre",
	"links":                      "Enlaces",
	"resolution":                 "Resolución",
	"project":                    "Proyecto",
	"sprint":                     "Sprint",
	"original_estimate_seconds":  "Estimación original",
	"remaining_estimate_seconds": "Estimación restante",
}

// FieldLabel returns the Spanish name of a changed field. Unknown fields keep their internal name.
func FieldLabel(field string) string {
	if label, ok := fieldLabels[field]; ok {
		return label
	}
	return field
}

// ParseActivityFilter validates the filter query value. An empty value means all.
func ParseActivityFilter(value string) (string, error) {
	switch value {
	case "":
		return ActivityFilterAll, nil
	case ActivityFilterAll, ActivityFilterComments, ActivityFilterHistory:
		return value, nil
	}
	return "", ErrInvalidActivityFilter
}

// NormalizeActivityLimit applies the default page size and rejects sizes outside 1..MaxActivityLimit.
func NormalizeActivityLimit(limit int, given bool) (int, error) {
	if !given {
		return DefaultActivityLimit, nil
	}
	if limit < 1 || limit > MaxActivityLimit {
		return 0, ErrInvalidActivityLimit
	}
	return limit, nil
}

// ActivityCursor is the position after the last entry of a page. Entries are ordered by time, kind and id,
// all descending, so the triple is unique and the order is stable when timestamps are equal.
type ActivityCursor struct {
	At   time.Time `json:"t"`
	Kind string    `json:"k"`
	ID   uuid.UUID `json:"i"`
}

// EncodeActivityCursor returns the opaque form of the cursor.
func EncodeActivityCursor(c ActivityCursor) string {
	raw, _ := json.Marshal(c) // the struct has only plain fields, marshaling cannot fail
	return base64.RawURLEncoding.EncodeToString(raw)
}

// DecodeActivityCursor parses an opaque cursor. An empty value means the first page.
func DecodeActivityCursor(value string) (*ActivityCursor, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, ErrInvalidActivityCursor
	}
	var c ActivityCursor
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, ErrInvalidActivityCursor
	}
	if c.At.IsZero() || (c.Kind != FeedKindComment && c.Kind != FeedKindHistory) || c.ID == uuid.Nil {
		return nil, ErrInvalidActivityCursor
	}
	c.At = c.At.UTC()
	return &c, nil
}
