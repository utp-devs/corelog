package pagination

import (
	"errors"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		page    string
		size    string
		want    Params
		wantErr bool
	}{
		{"empty values use defaults", "", "", Params{Page: 1, PageSize: 20}, false},
		{"valid values", "3", "50", Params{Page: 3, PageSize: 50}, false},
		{"max page size accepted", "1", "100", Params{Page: 1, PageSize: 100}, false},
		{"page zero rejected", "0", "", Params{}, true},
		{"page negative rejected", "-1", "", Params{}, true},
		{"page non numeric rejected", "abc", "", Params{}, true},
		{"size zero rejected", "", "0", Params{}, true},
		{"size negative rejected", "", "-1", Params{}, true},
		{"size non numeric rejected", "", "abc", Params{}, true},
		{"size above max rejected", "", "101", Params{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.page, tt.size)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if apperror.KindOf(err) != apperror.KindValidation {
					t.Errorf("kind = %v, want validation", apperror.KindOf(err))
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestOffsetAndMeta(t *testing.T) {
	tests := []struct {
		name       string
		p          Params
		wantOffset int
	}{
		{"first page", Params{Page: 1, PageSize: 20}, 0},
		{"second page", Params{Page: 2, PageSize: 20}, 20},
		{"third page of 10", Params{Page: 3, PageSize: 10}, 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.Offset(); got != tt.wantOffset {
				t.Errorf("Offset() = %d, want %d", got, tt.wantOffset)
			}
		})
	}

	t.Run("NewMeta copies params and total", func(t *testing.T) {
		got := NewMeta(Params{Page: 2, PageSize: 20}, 41)
		want := Meta{Page: 2, PageSize: 20, Total: 41}
		if got != want {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})
}

func TestParseRequest(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    Params
		wantErr bool
	}{
		{"no query uses defaults", "/api/v1/issues", Params{Page: 1, PageSize: 20}, false},
		{"reads page and page_size", "/api/v1/issues?page=2&page_size=5", Params{Page: 2, PageSize: 5}, false},
		{"invalid page_size", "/api/v1/issues?page_size=500", Params{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRequest(httptest.NewRequest("GET", tt.url, nil))
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

type cursorPayload struct {
	ID        string `json:"id"`
	CreatedAt string `json:"created_at"`
}

func TestCursorRoundTrip(t *testing.T) {
	in := cursorPayload{ID: "550e8400-e29b-41d4-a716-446655440000", CreatedAt: "2026-11-21T18:30:00Z"}
	enc, err := EncodeCursor(in)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var out cursorPayload
	if err := DecodeCursor(enc, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("round trip = %+v, want %+v", out, in)
	}
}

func TestDecodeCursor(t *testing.T) {
	t.Run("empty cursor yields zero value without error", func(t *testing.T) {
		out := cursorPayload{ID: "stale", CreatedAt: "stale"}
		if err := DecodeCursor("", &out); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out != (cursorPayload{}) {
			t.Errorf("out = %+v, want zero value", out)
		}
	})

	garbage := []struct {
		name   string
		cursor string
	}{
		{"not base64", "!!!not-base64!!!"},
		{"base64 but not json", "bm90IGpzb24"},
		{"json of wrong type", "MQ"},
	}
	for _, tt := range garbage {
		t.Run(tt.name, func(t *testing.T) {
			var out cursorPayload
			err := DecodeCursor(tt.cursor, &out)
			if err == nil {
				t.Fatal("expected error")
			}
			var appErr *apperror.Error
			if !errors.As(err, &appErr) || appErr.Kind != apperror.KindValidation {
				t.Errorf("want validation apperror, got %v", err)
			}
		})
	}
}
