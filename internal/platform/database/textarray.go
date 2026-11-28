package database

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// TextList scans and stores a list of strings kept in a TEXT[] column. Queries read the column as
// to_jsonb(column) and write it with ARRAY(SELECT jsonb_array_elements_text($n::jsonb)), so it does not depend on
// the driver array support. A nil list is stored as an empty array.
type TextList []string

// Scan implements sql.Scanner for the JSON text produced by to_jsonb.
func (l *TextList) Scan(src any) error {
	var raw []byte
	switch v := src.(type) {
	case nil:
		*l = nil
		return nil
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("database: cannot scan %T into TextList", src)
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("database: decode text list: %w", err)
	}
	*l = out
	return nil
}

// Value implements driver.Valuer: the list is sent as JSON text.
func (l TextList) Value() (driver.Value, error) {
	if l == nil {
		return "[]", nil
	}
	raw, err := json.Marshal([]string(l))
	if err != nil {
		return nil, err
	}
	return string(raw), nil
}
