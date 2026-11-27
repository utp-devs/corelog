package http

import (
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

func TestParsePatchReadsVersionAndComponentFields(t *testing.T) {
	fix := uuid.New()
	component := uuid.New()
	in, err := parsePatch([]byte(`{"fix_versions":["` + fix.String() + `"],"affects_versions":null,"components":["` + component.String() + `"],"override_closed_versions":true}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !in.FixVersions.Set || in.FixVersions.Value == nil || (*in.FixVersions.Value)[0] != fix {
		t.Fatalf("fix_versions = %+v", in.FixVersions)
	}
	if !in.AffectsVersions.Set || in.AffectsVersions.Value != nil {
		t.Fatalf("null affects_versions must clear the set: %+v", in.AffectsVersions)
	}
	if !in.Components.Set || (*in.Components.Value)[0] != component {
		t.Fatalf("components = %+v", in.Components)
	}
	if !in.OverrideClosedVersions {
		t.Fatal("override flag not read")
	}
}

func TestParsePatchRejectsMistypedLinkFields(t *testing.T) {
	_, err := parsePatch([]byte(`{"components":"not-a-list"}`))
	if apperror.KindOf(err) != apperror.KindValidation {
		t.Fatalf("mistyped components accepted: %v", err)
	}
	_, err = parsePatch([]byte(`{"fix_version":["` + uuid.New().String() + `"]}`))
	if apperror.KindOf(err) != apperror.KindValidation {
		t.Fatalf("unknown field accepted: %v", err)
	}
}
