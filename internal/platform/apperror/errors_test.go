package apperror

import (
	"errors"
	"testing"
)

func TestAppError(t *testing.T) {
	t.Run("New creates error with kind and message", func(t *testing.T) {
		err := New(KindNotFound, "resource not found")
		if err.Kind != KindNotFound {
			t.Errorf("got kind %v, want %v", err.Kind, KindNotFound)
		}
		if err.Error() != "resource not found" {
			t.Errorf("got message %q, want 'resource not found'", err.Error())
		}
	})

	t.Run("Wrap creates error wrapping underlying error", func(t *testing.T) {
		orig := errors.New("db connection lost")
		wrapped := Wrap(KindInternal, "operation failed", orig)
		if wrapped.Kind != KindInternal {
			t.Errorf("got kind %v, want %v", wrapped.Kind, KindInternal)
		}
		if wrapped.Error() != "operation failed: db connection lost" {
			t.Errorf("got error string %q", wrapped.Error())
		}
		if !errors.Is(wrapped, orig) {
			t.Errorf("expected errors.Is to match underlying error")
		}
	})

	t.Run("KindOf returns correct kind or KindInternal for standard error", func(t *testing.T) {
		appErr := New(KindValidation, "invalid input")
		if KindOf(appErr) != KindValidation {
			t.Errorf("got kind %v, want %v", KindOf(appErr), KindValidation)
		}

		stdErr := errors.New("generic error")
		if KindOf(stdErr) != KindInternal {
			t.Errorf("got kind %v, want %v", KindOf(stdErr), KindInternal)
		}
	})
}
