package http

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/workflow/application/port"
	"github.com/jhonsferg/corelog/internal/workflow/domain"
)

// definitionBody is the workflow sent by the editor. Statuses are named by a local ref; transitions refer to
// those refs in from and to, and initial_ref names the initial one. An existing organization status is sent
// with its id; a new one with name and category and is created when the organization has no status of that name.
type definitionBody struct {
	ExpectedVersion int               `json:"expected_version"`
	InitialRef      string            `json:"initial_ref"`
	Statuses        []statusBody      `json:"statuses"`
	Transitions     []json.RawMessage `json:"transitions"`
	StatusMapping   map[string]string `json:"status_mapping"`
}

type statusBody struct {
	Ref      string `json:"ref"`
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
}

type resetBody struct {
	Template        string            `json:"template"`
	ExpectedVersion int               `json:"expected_version"`
	StatusMapping   map[string]string `json:"status_mapping"`
}

type migrateBody struct {
	StatusMapping map[string]string `json:"status_mapping"`
}

type statusWriteBody struct {
	Name     *string `json:"name"`
	Category *string `json:"category"`
}

type createStatusBody struct {
	Name     string `json:"name"`
	Category string `json:"category"`
}

type reorderBody struct {
	StatusIDs []string `json:"status_ids"`
}

// toDefinition converts the body and reports the shape problems it finds. Problems stop the request before a
// use case runs.
func (b definitionBody) toDefinition() (port.Definition, []domain.Issue) {
	var issues []domain.Issue
	def := port.Definition{ExpectedVersion: b.ExpectedVersion, InitialRef: b.InitialRef}
	for i, s := range b.Statuses {
		in := port.StatusInput{Ref: s.Ref, Name: s.Name, Category: domain.Category(s.Category)}
		if s.ID != "" {
			id, err := uuid.Parse(s.ID)
			if err != nil {
				issues = append(issues, domain.Issue{Path: fmt.Sprintf("statuses[%d].id", i), Code: "workflow.invalid_id", Message: "el identificador del estado no es un UUID válido"})
			}
			in.ID = id
		}
		def.Statuses = append(def.Statuses, in)
	}
	for i, raw := range b.Transitions {
		t, err := domain.DecodeTransition(raw)
		if err != nil {
			issues = append(issues, domain.Issue{Path: fmt.Sprintf("transitions[%d]", i), Code: errorCode(err), Message: err.Error()})
			continue
		}
		def.Transitions = append(def.Transitions, t)
	}
	def.Mapping, issues = parseMapping(b.StatusMapping, issues)
	return def, issues
}

// parseMapping converts the JSON mapping into ids and records the entries that are not UUIDs.
func parseMapping(raw map[string]string, issues []domain.Issue) (map[uuid.UUID]uuid.UUID, []domain.Issue) {
	out := make(map[uuid.UUID]uuid.UUID, len(raw))
	for key, target := range raw {
		from, errFrom := uuid.Parse(key)
		to, errTo := uuid.Parse(target)
		if errFrom != nil || errTo != nil {
			issues = append(issues, domain.Issue{Path: "status_mapping." + key, Code: "workflow.invalid_status_mapping", Message: "el mapeo de estados debe usar identificadores UUID"})
			continue
		}
		out[from] = to
	}
	return out, issues
}

func errorCode(err error) string {
	if code := apperror.CodeOf(err); code != "" {
		return code
	}
	return domain.CodeMalformed
}

// statusResponse is one status of a stored workflow.
type statusResponse struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Category   string `json:"category"`
	Position   int    `json:"position"`
	IssueCount int    `json:"issue_count"`
}

// workflowResponse is a stored workflow. Transitions use the domain encoding.
type workflowResponse struct {
	Statuses          []statusResponse  `json:"statuses"`
	InitialStatusID   string            `json:"initial_status_id"`
	Transitions       []json.RawMessage `json:"transitions"`
	Version           int               `json:"version"`
	UpdatedAt         time.Time         `json:"updated_at"`
	IsTemplateDefault bool              `json:"is_template_default"`
}

// saveResponse is the stored workflow of a replace, reset or migration with the warnings of the change.
type saveResponse struct {
	workflowResponse
	Warnings []detailResponse `json:"warnings"`
}

type validationResponse struct {
	Valid           bool              `json:"valid"`
	Errors          []detailResponse  `json:"errors"`
	Warnings        []detailResponse  `json:"warnings"`
	MappingRequired []mappingResponse `json:"mapping_required"`
}

type migrationResponse struct {
	Moved int            `json:"moved"`
	Moves []moveResponse `json:"moves"`
}

type moveResponse struct {
	FromStatusID string `json:"from_status_id"`
	ToStatusID   string `json:"to_status_id"`
	IssueCount   int    `json:"issue_count"`
}

// templateResponse is a template in the request format, so the editor can send it back as a definition.
type templateResponse struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Definition  definitionOutput `json:"definition"`
}

type definitionOutput struct {
	InitialRef  string            `json:"initial_ref"`
	Statuses    []refStatus       `json:"statuses"`
	Transitions []json.RawMessage `json:"transitions"`
}

type refStatus struct {
	Ref      string `json:"ref"`
	Name     string `json:"name"`
	Category string `json:"category"`
}

// orgStatusResponse is one entry of the organization status catalog.
type orgStatusResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Position int    `json:"position"`
}

type deleteStatusResponse struct {
	ID               string `json:"id"`
	ReassignedIssues int    `json:"reassigned_issues"`
}

func toWorkflowResponse(view port.WorkflowView) (workflowResponse, error) {
	out := workflowResponse{
		Statuses:          make([]statusResponse, 0, len(view.Statuses)),
		InitialStatusID:   idText(view.InitialStatusID),
		Transitions:       make([]json.RawMessage, 0, len(view.Transitions)),
		Version:           view.Version,
		UpdatedAt:         view.UpdatedAt,
		IsTemplateDefault: view.IsTemplateDefault,
	}
	for _, s := range view.Statuses {
		out.Statuses = append(out.Statuses, statusResponse{ID: idText(s.ID), Name: s.Name, Category: string(s.Category), Position: s.Position, IssueCount: s.IssueCount})
	}
	for _, t := range view.Transitions {
		raw, err := domain.EncodeTransition(t)
		if err != nil {
			return workflowResponse{}, err
		}
		out.Transitions = append(out.Transitions, raw)
	}
	return out, nil
}

func toSaveResponse(result port.SaveResult) (saveResponse, error) {
	wf, err := toWorkflowResponse(result.Workflow)
	if err != nil {
		return saveResponse{}, err
	}
	return saveResponse{workflowResponse: wf, Warnings: toDetails(result.Warnings)}, nil
}

func toDetails(issues []domain.Issue) []detailResponse {
	out := make([]detailResponse, 0, len(issues))
	for _, issue := range issues {
		out = append(out, detailResponse{Path: issue.Path, Code: issue.Code, Message: issue.Message})
	}
	return out
}

func toValidationResponse(report port.ValidationReport) validationResponse {
	out := validationResponse{
		Valid:           report.Valid,
		Errors:          toDetails(report.Errors),
		Warnings:        toDetails(report.Warnings),
		MappingRequired: make([]mappingResponse, 0, len(report.MappingRequired)),
	}
	for _, m := range report.MappingRequired {
		out.MappingRequired = append(out.MappingRequired, mappingResponse{StatusID: m.StatusID.String(), Name: m.Name, IssueCount: m.IssueCount})
	}
	return out
}

func toMigrationResponse(result port.MigrationResult) migrationResponse {
	out := migrationResponse{Moved: result.Total, Moves: make([]moveResponse, 0, len(result.Moves))}
	for _, m := range result.Moves {
		out.Moves = append(out.Moves, moveResponse{FromStatusID: m.From.String(), ToStatusID: m.To.String(), IssueCount: m.Count})
	}
	return out
}

func toTemplateResponse(t port.TemplateView) (templateResponse, error) {
	out := templateResponse{ID: t.ID, Name: t.Name, Description: t.Description, Definition: definitionOutput{
		InitialRef: t.Definition.InitialRef, Statuses: []refStatus{}, Transitions: []json.RawMessage{},
	}}
	for _, s := range t.Definition.Statuses {
		out.Definition.Statuses = append(out.Definition.Statuses, refStatus{Ref: s.Ref, Name: s.Name, Category: string(s.Category)})
	}
	for _, tr := range t.Definition.Transitions {
		raw, err := domain.EncodeTransition(tr)
		if err != nil {
			return templateResponse{}, err
		}
		out.Definition.Transitions = append(out.Definition.Transitions, raw)
	}
	return out, nil
}

func toOrgStatusResponse(s port.OrgStatus) orgStatusResponse {
	return orgStatusResponse{ID: s.ID.String(), Name: s.Name, Category: string(s.Category), Position: s.Position}
}

func idText(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}
	return id.String()
}
