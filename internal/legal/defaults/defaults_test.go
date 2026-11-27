package defaults

import (
	"strings"
	"testing"

	"github.com/jhonsferg/corelog/internal/legal/domain"
)

func TestDefaultDocuments(t *testing.T) {
	docs := Documents()
	if len(docs) != len(domain.AllSlugs) {
		t.Fatalf("documents = %d, want %d", len(docs), len(domain.AllSlugs))
	}
	for _, d := range docs {
		if !d.Slug.Valid() || d.Title == "" || d.Summary == "" {
			t.Errorf("incomplete document %+v", d.Slug)
		}
		if !strings.HasPrefix(d.Body, "> **Borrador de plantilla: debe ser revisado por un asesor legal antes de su publicación.**") {
			t.Errorf("%s does not start with the review notice", d.Slug)
		}
		if !strings.Contains(d.Body, "[") || !strings.Contains(d.Body, "[Razón social de la empresa]") {
			t.Errorf("%s has no placeholders", d.Slug)
		}
		if d.Required != d.Slug.DefaultRequired() {
			t.Errorf("%s required = %v, want %v", d.Slug, d.Required, d.Slug.DefaultRequired())
		}
		if len([]rune(d.Body)) > domain.MaxBodyLength {
			t.Errorf("%s is too long", d.Slug)
		}
	}
	for _, d := range docs {
		if d.Slug == domain.SlugDataProcessing {
			for _, want := range []string{"29733", "Acceso", "Rectificación", "Cancelación", "Oposición", "teléfono"} {
				if !strings.Contains(d.Body, want) {
					t.Errorf("data processing policy lacks %q", want)
				}
			}
		}
	}
}
