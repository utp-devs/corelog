package validation

import (
	"testing"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

type failedAddress struct {
	City string `json:"city_name" validate:"required"`
}

type failedSample struct {
	Name    string        `json:"full_name" validate:"required"`
	Address failedAddress `json:"address"`
}

func TestFailedMapsCodeAndJSONFieldNames(t *testing.T) {
	in := failedSample{}
	err := Struct(in)
	if err == nil {
		t.Fatal("expected validation error")
	}
	appErr := Failed(in, err)
	if appErr.Kind != apperror.KindValidation {
		t.Fatalf("kind %q, want validation", appErr.Kind)
	}
	if appErr.Code != CodeFailed {
		t.Fatalf("code %q, want %q", appErr.Code, CodeFailed)
	}
	if apperror.CodeOf(appErr) != CodeFailed {
		t.Fatalf("CodeOf = %q", apperror.CodeOf(appErr))
	}
	got := map[string]string{}
	for _, d := range appErr.Details {
		got[d.Field] = d.Message
	}
	if got["full_name"] != "es obligatorio" {
		t.Errorf("full_name detail = %q", got["full_name"])
	}
	if got["address.city_name"] != "es obligatorio" {
		t.Errorf("address.city_name detail = %q; all %+v", got["address.city_name"], appErr.Details)
	}
	if appErr.Message != "invalid request body" {
		t.Errorf("message = %q", appErr.Message)
	}
}
