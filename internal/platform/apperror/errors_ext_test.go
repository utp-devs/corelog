package apperror

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func TestWithCodeAndDetails(t *testing.T) {
	t.Run("WithCode sets kind, code and message", func(t *testing.T) {
		err := WithCode(KindConflict, "team_name_taken", "team name already exists")
		if err.Kind != KindConflict || err.Code != "team_name_taken" || err.Message != "team name already exists" {
			t.Fatalf("unexpected error: %+v", err)
		}
		if err.Error() != "team name already exists" {
			t.Errorf("got %q", err.Error())
		}
	})

	t.Run("WithDetails returns the same error and appends details", func(t *testing.T) {
		err := New(KindValidation, "invalid input")
		got := err.WithDetails(FieldError{Field: "title", Message: "is required"})
		if got != err {
			t.Fatal("WithDetails must return the same error pointer")
		}
		got.WithDetails(FieldError{Field: "email", Message: "is invalid"})
		want := []FieldError{{Field: "title", Message: "is required"}, {Field: "email", Message: "is invalid"}}
		if !reflect.DeepEqual(err.Details, want) {
			t.Errorf("details = %+v, want %+v", err.Details, want)
		}
	})

	t.Run("KindRateLimited is a distinct kind", func(t *testing.T) {
		if KindOf(New(KindRateLimited, "slow down")) != KindRateLimited {
			t.Error("KindOf did not return KindRateLimited")
		}
		if KindRateLimited == KindInternal {
			t.Error("KindRateLimited must not equal KindInternal")
		}
	})
}

func TestCodeAndDetailsHelpers(t *testing.T) {
	details := []FieldError{{Field: "page_size", Message: "must be at most 100"}}
	plain := errors.New("raw error")
	wrapped := fmt.Errorf("handler: %w", WithCode(KindValidation, "bad_page", "bad page").WithDetails(details...))

	tests := []struct {
		name        string
		err         error
		wantCode    string
		wantDetails []FieldError
	}{
		{"plain apperror without code", New(KindNotFound, "missing"), "", nil},
		{"apperror with code", WithCode(KindNotFound, "issue_not_found", "missing"), "issue_not_found", nil},
		{"apperror with details", New(KindValidation, "x").WithDetails(details...), "", details},
		{"non apperror", plain, "", nil},
		{"wrapped apperror is found through the chain", wrapped, "bad_page", details},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CodeOf(tt.err); got != tt.wantCode {
				t.Errorf("CodeOf = %q, want %q", got, tt.wantCode)
			}
			if got := DetailsOf(tt.err); !reflect.DeepEqual(got, tt.wantDetails) {
				t.Errorf("DetailsOf = %+v, want %+v", got, tt.wantDetails)
			}
		})
	}
}
