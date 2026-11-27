package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

func TestValidateKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
		valid bool
	}{
		{input: "CORE", want: "CORE", valid: true},
		{input: " core ", want: "CORE", valid: true},
		{input: "AB", want: "AB", valid: true},
		{input: "A1B2C3D4E5", want: "A1B2C3D4E5", valid: true},
		{input: "A", valid: false},
		{input: "A12345678901", valid: false},
		{input: "1CORE", valid: false},
		{input: "CO-RE", valid: false},
		{input: "CO RE", valid: false},
		{input: "", valid: false},
		{input: "ÁBC", valid: false},
	}
	for _, tt := range tests {
		got, err := ValidateKey(tt.input)
		if tt.valid {
			if err != nil || got != tt.want {
				t.Errorf("ValidateKey(%q) = %q, %v; want %q", tt.input, got, err, tt.want)
			}
			continue
		}
		if apperror.CodeOf(err) != "project.invalid_key" {
			t.Errorf("ValidateKey(%q) code = %q, want project.invalid_key", tt.input, apperror.CodeOf(err))
		}
	}
}

func TestTemplateCatalogMatchesDesign(t *testing.T) {
	templates := Templates()
	if len(templates) != 4 {
		t.Fatalf("templates = %d, want 4", len(templates))
	}
	byName := map[Template]TemplateDefinition{}
	for _, definition := range templates {
		byName[definition.Template] = definition
	}

	if !byName[TemplateScrum].SprintsEnabled || byName[TemplateScrum].ProjectType != ProjectTypeScrum {
		t.Fatal("scrum must enable sprints and use the scrum type")
	}
	for _, template := range []Template{TemplateKanban, TemplateTaskTracking, TemplateBugTracking} {
		if byName[template].SprintsEnabled || byName[template].ProjectType != ProjectTypeKanban {
			t.Fatalf("%s must disable sprints and use the kanban type", template)
		}
	}
	if got := len(byName[TemplateTaskTracking].Statuses); got != 3 {
		t.Fatalf("task tracking statuses = %d, want 3", got)
	}
	bug := byName[TemplateBugTracking]
	if len(bug.Statuses) != 5 || bug.Statuses[0].Name != "Nuevo" || bug.Statuses[4].Name != "Cerrado" {
		t.Fatalf("bug tracking statuses = %+v", bug.Statuses)
	}
	for _, definition := range templates {
		if len(definition.BoardColumns) == 0 || len(definition.IssueTypes) == 0 {
			t.Fatalf("%s has empty columns or issue types", definition.Template)
		}
	}
}

func TestTemplateDefinitionIsACopy(t *testing.T) {
	definition, err := TemplateDefinitionOf(TemplateScrum)
	if err != nil {
		t.Fatal(err)
	}
	definition.Statuses[0].Name = "changed"
	again, _ := TemplateDefinitionOf(TemplateScrum)
	if again.Statuses[0].Name == "changed" {
		t.Fatal("catalog must not change through a returned definition")
	}
}

func TestParseTemplate(t *testing.T) {
	if got, err := ParseTemplate(" Bug_Tracking "); err != nil || got != TemplateBugTracking {
		t.Fatalf("ParseTemplate = %q, %v", got, err)
	}
	if _, err := ParseTemplate("wiki"); apperror.CodeOf(err) != "project.invalid_template" {
		t.Fatalf("unknown template code = %q", apperror.CodeOf(err))
	}
}

func TestNewProjectFromInputDefaultsAndTemplateType(t *testing.T) {
	orgID, leadID := uuid.New(), uuid.New()
	p, err := NewProjectFromInput(NewProjectInput{
		OrganizationID: orgID, Key: "bug", Name: "Bugs", Template: TemplateBugTracking,
		Visibility: VisibilityTeam, LeadID: leadID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.Key != "BUG" || p.Visibility != VisibilityOpen || p.Type != ProjectTypeKanban {
		t.Fatalf("project = %+v", p)
	}
	if p.Color != DefaultColor || p.DefaultAssigneePolicy != AssigneePolicyUnassigned || p.NextIssueNumber != 1 {
		t.Fatalf("defaults = color %q policy %q next %d", p.Color, p.DefaultAssigneePolicy, p.NextIssueNumber)
	}
}

func TestNewProjectLegacyTypeMapsToTemplate(t *testing.T) {
	orgID, leadID := uuid.New(), uuid.New()
	p, err := NewProject(orgID, "SCR", "Scrum", "", ProjectTypeScrum, VisibilityOpen, leadID)
	if err != nil || p.Template != TemplateScrum || p.Type != ProjectTypeScrum {
		t.Fatalf("scrum legacy = %+v, %v", p, err)
	}
	if _, err := NewProject(orgID, "KAN", "Kanban", "", ProjectType("other"), VisibilityOpen, leadID); err == nil {
		t.Fatal("unknown legacy type must fail")
	}
}

func TestNewProjectFromInputRejectsBadValues(t *testing.T) {
	valid := NewProjectInput{OrganizationID: uuid.New(), Key: "OK", Name: "Ok", Template: TemplateKanban, Visibility: VisibilityOpen, LeadID: uuid.New()}
	cases := map[string]func(*NewProjectInput){
		"key":        func(in *NewProjectInput) { in.Key = "x" },
		"template":   func(in *NewProjectInput) { in.Template = "wiki" },
		"visibility": func(in *NewProjectInput) { in.Visibility = "secret" },
		"policy":     func(in *NewProjectInput) { in.DefaultAssigneePolicy = "anyone" },
		"color":      func(in *NewProjectInput) { in.Color = "a-very-long-color-name-that-goes-over-the-limit-here" },
		"lead":       func(in *NewProjectInput) { in.LeadID = uuid.Nil },
		"org":        func(in *NewProjectInput) { in.OrganizationID = uuid.Nil },
	}
	for name, mutate := range cases {
		in := valid
		mutate(&in)
		if _, err := NewProjectFromInput(in); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func newTestProject(t *testing.T, template Template) *Project {
	t.Helper()
	p, err := NewProjectFromInput(NewProjectInput{
		OrganizationID: uuid.New(), Key: "LIFE", Name: "Lifecycle", Template: template,
		Visibility: VisibilityOpen, LeadID: uuid.New(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestArchiveAndRestoreRules(t *testing.T) {
	p := newTestProject(t, TemplateKanban)
	at := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)

	if err := p.Restore(at); apperror.CodeOf(err) != "project.not_archived" {
		t.Fatalf("restore active = %q", apperror.CodeOf(err))
	}
	if err := p.Archive(at); err != nil || !p.IsArchived() {
		t.Fatalf("archive = %v", err)
	}
	if err := p.Archive(at); apperror.CodeOf(err) != "project.already_archived" {
		t.Fatalf("archive twice = %q", apperror.CodeOf(err))
	}
	if err := p.Restore(at); err != nil || p.IsArchived() {
		t.Fatalf("restore = %v", err)
	}
}

func TestArchivedProjectRejectsUpdate(t *testing.T) {
	p := newTestProject(t, TemplateKanban)
	if err := p.Archive(time.Now()); err != nil {
		t.Fatal(err)
	}
	err := p.Update(UpdateInput{Name: "New", Type: ProjectTypeKanban, Visibility: VisibilityOpen, LeadID: p.LeadID})
	if apperror.CodeOf(err) != "project.archived" {
		t.Fatalf("update archived = %q", apperror.CodeOf(err))
	}
}

func TestSoftDeleteAndConfirmation(t *testing.T) {
	p := newTestProject(t, TemplateKanban)
	if err := p.ConfirmDeletion("LIFE"); apperror.CodeOf(err) != "project.confirmation_mismatch" {
		t.Fatalf("mismatch = %q", apperror.CodeOf(err))
	}
	if err := p.ConfirmDeletion("  Lifecycle "); err != nil {
		t.Fatalf("exact name with spaces = %v", err)
	}
	at := time.Now()
	if err := p.SoftDelete(at); err != nil || !p.IsDeleted() {
		t.Fatalf("SoftDelete = %v", err)
	}
	if err := p.SoftDelete(at); apperror.CodeOf(err) != "project.not_found" {
		t.Fatalf("delete twice = %q", apperror.CodeOf(err))
	}
}

func TestUpdateTemplateTypeRules(t *testing.T) {
	update := func(p *Project, typ ProjectType) error {
		return p.Update(UpdateInput{Name: "Lifecycle", Type: typ, Visibility: VisibilityOpen, LeadID: p.LeadID})
	}

	kanban := newTestProject(t, TemplateKanban)
	if err := update(kanban, ProjectTypeScrum); err != nil || kanban.Template != TemplateScrum || kanban.Type != ProjectTypeScrum {
		t.Fatalf("kanban to scrum: %v template %q", err, kanban.Template)
	}
	if err := update(kanban, ProjectTypeKanban); err != nil || kanban.Template != TemplateKanban {
		t.Fatalf("scrum to kanban: %v template %q", err, kanban.Template)
	}

	task := newTestProject(t, TemplateTaskTracking)
	if err := update(task, ProjectTypeScrum); apperror.CodeOf(err) != "project.invalid" {
		t.Fatalf("task tracking to scrum = %q", apperror.CodeOf(err))
	}
	if err := update(task, ProjectTypeKanban); err != nil || task.Template != TemplateTaskTracking {
		t.Fatalf("task tracking keeps template: %v %q", err, task.Template)
	}
}

func TestUpdateKeepsAppearanceWhenEmpty(t *testing.T) {
	p := newTestProject(t, TemplateKanban)
	p.Color, p.Icon = "green", "rocket"
	if err := p.Update(UpdateInput{Name: "Lifecycle", Type: ProjectTypeKanban, Visibility: VisibilityOpen, LeadID: p.LeadID}); err != nil {
		t.Fatal(err)
	}
	if p.Color != "green" || p.Icon != "rocket" {
		t.Fatalf("appearance changed to %q %q", p.Color, p.Icon)
	}
}

func TestParseAssigneePolicy(t *testing.T) {
	if got, err := ParseAssigneePolicy(""); err != nil || got != AssigneePolicyUnassigned {
		t.Fatalf("empty = %q, %v", got, err)
	}
	if got, err := ParseAssigneePolicy("lead"); err != nil || got != AssigneePolicyLead {
		t.Fatalf("lead = %q, %v", got, err)
	}
	if _, err := ParseAssigneePolicy("everyone"); apperror.CodeOf(err) != "project.invalid_assignee_policy" {
		t.Fatalf("everyone = %q", apperror.CodeOf(err))
	}
}
