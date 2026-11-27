package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewLabelValidatesPaletteColor(t *testing.T) {
	projectID := uuid.New()
	label, err := NewLabel(projectID, "bug", "")
	if err != nil || label.Color != "blue" {
		t.Fatalf("default color: %v, %+v", err, label)
	}
	if _, err := NewLabel(projectID, "bug", "#ff0000"); err != ErrInvalidLabelColor {
		t.Errorf("color outside palette: err = %v, want ErrInvalidLabelColor", err)
	}
	if _, err := NewLabel(projectID, "  ", "red"); err != ErrInvalidCatalogItem {
		t.Errorf("blank name: err = %v", err)
	}
}

func TestLabelUpdateKeepsNilArguments(t *testing.T) {
	label, err := NewLabel(uuid.New(), "bug", "red")
	if err != nil {
		t.Fatal(err)
	}
	color := "pink"
	if err := label.Update(nil, &color); err != nil {
		t.Fatal(err)
	}
	if label.Name != "bug" || label.Color != "pink" {
		t.Errorf("label = %+v, want name kept and color pink", label)
	}
	bad := "neon"
	if err := label.Update(nil, &bad); err != ErrInvalidLabelColor {
		t.Errorf("bad color: err = %v", err)
	}
	empty := "   "
	if err := label.Update(&empty, nil); err != ErrInvalidCatalogItem {
		t.Errorf("blank name: err = %v", err)
	}
}
