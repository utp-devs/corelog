// Package pagination parses page/page_size query parameters and opaque cursors.
package pagination

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

const (
	DefaultPage     = 1
	DefaultPageSize = 20
	MaxPageSize     = 100
)

// Params is a validated page request.
type Params struct {
	Page     int
	PageSize int
}

// Meta describes a page of results in list responses.
type Meta struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	Total    int `json:"total"`
}

// Parse validates the raw page and page_size values. Empty values take the
// defaults; zero, negative, non-numeric or oversized values are validation errors.
func Parse(pageStr, sizeStr string) (Params, error) {
	page := DefaultPage
	if pageStr != "" {
		n, err := strconv.Atoi(pageStr)
		if err != nil || n < 1 {
			return Params{}, apperror.New(apperror.KindValidation, "page must be a positive integer")
		}
		page = n
	}

	size := DefaultPageSize
	if sizeStr != "" {
		n, err := strconv.Atoi(sizeStr)
		if err != nil || n < 1 {
			return Params{}, apperror.New(apperror.KindValidation, "page_size must be a positive integer")
		}
		if n > MaxPageSize {
			return Params{}, apperror.New(apperror.KindValidation, fmt.Sprintf("page_size must be at most %d", MaxPageSize))
		}
		size = n
	}

	return Params{Page: page, PageSize: size}, nil
}

// Offset returns the SQL offset for the page.
func (p Params) Offset() int { return (p.Page - 1) * p.PageSize }

// NewMeta builds the response metadata for p and the total row count.
func NewMeta(p Params, total int) Meta {
	return Meta{Page: p.Page, PageSize: p.PageSize, Total: total}
}

// ParseRequest reads and validates the page and page_size query parameters.
func ParseRequest(r *http.Request) (Params, error) {
	q := r.URL.Query()
	return Parse(q.Get("page"), q.Get("page_size"))
}

// EncodeCursor serializes v as an opaque URL-safe cursor.
func EncodeCursor(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("pagination: encode cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// DecodeCursor parses a cursor produced by EncodeCursor into out (a non-nil
// pointer). An empty cursor leaves out at its zero value and returns nil.
// Malformed cursors are validation errors.
func DecodeCursor(s string, out any) error {
	if s == "" {
		if rv := reflect.ValueOf(out); rv.Kind() == reflect.Pointer && !rv.IsNil() {
			rv.Elem().Set(reflect.Zero(rv.Elem().Type()))
		}
		return nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return apperror.New(apperror.KindValidation, "invalid cursor")
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return apperror.New(apperror.KindValidation, "invalid cursor")
	}
	return nil
}
