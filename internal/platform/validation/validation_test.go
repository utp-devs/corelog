package validation

import (
	"testing"
)

type sampleStruct struct {
	Name  string `validate:"required"`
	Email string `validate:"required,email"`
}

func TestValidation(t *testing.T) {
	t.Run("valid struct passes validation", func(t *testing.T) {
		valid := sampleStruct{Name: "Alice", Email: "alice@example.com"}
		if err := Struct(valid); err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("invalid struct fails validation", func(t *testing.T) {
		invalid := sampleStruct{Name: "", Email: "not-an-email"}
		if err := Struct(invalid); err == nil {
			t.Errorf("expected validation error, got nil")
		}
	})
}
