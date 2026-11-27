package usecase_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/workflow/application/port"
	"github.com/jhonsferg/corelog/internal/workflow/application/usecase"
	"github.com/jhonsferg/corelog/internal/workflow/domain"
	"github.com/jhonsferg/corelog/internal/workflow/workflowtest"
)

var fixedNow = time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)

type fixture struct {
	repo        *workflowtest.Repo
	authz       workflowtest.Authz
	orgs        workflowtest.OrgRoles
	svc         *usecase.Management
	org1, org2  uuid.UUID
	lead        uuid.UUID
	contributor uuid.UUID
	viewer      uuid.UUID
	orgAdmin    uuid.UUID
	orgMember   uuid.UUID
	outsider    uuid.UUID
	project     uuid.UUID // scrum project of org1
	otherOrg    uuid.UUID // a project of org2
	orgTwoAdmin uuid.UUID
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{
		repo: workflowtest.NewRepo(), org1: uuid.New(), org2: uuid.New(),
		lead: uuid.New(), contributor: uuid.New(), viewer: uuid.New(), orgAdmin: uuid.New(),
		orgMember: uuid.New(), outsider: uuid.New(), orgTwoAdmin: uuid.New(),
	}
	f.project = f.repo.SeedProject(f.org1, "scrum")
	f.otherOrg = f.repo.SeedProject(f.org2, "task_tracking")
	f.authz = workflowtest.Authz{
		Roles:    map[uuid.UUID]string{f.lead: "lead", f.contributor: "contributor", f.viewer: "viewer", f.orgAdmin: "org_admin", f.orgTwoAdmin: "org_admin"},
		Archived: map[uuid.UUID]bool{},
	}
	f.orgs = workflowtest.OrgRoles{Roles: map[workflowtest.OrgUser]string{
		{OrgID: f.org1, UserID: f.orgAdmin}:    "admin",
		{OrgID: f.org1, UserID: f.orgMember}:   "member",
		{OrgID: f.org2, UserID: f.orgTwoAdmin}: "admin",
	}}
	f.svc = usecase.NewManagement(f.repo, f.authz, f.orgs, workflowtest.Tx{}, &clock.FakeClock{T: fixedNow})
	return f
}

func (f *fixture) workflow() domain.Workflow { return f.repo.Workflows[f.project].Workflow }

func (f *fixture) version() int { return f.repo.Workflows[f.project].Version }

func statusIDByName(t *testing.T, w domain.Workflow, name string) string {
	t.Helper()
	for _, s := range w.Statuses {
		if strings.EqualFold(s.Name, name) {
			return s.ID
		}
	}
	t.Fatalf("status %q not in workflow", name)
	return ""
}

// definitionOf turns a stored workflow into the request format: every status is referenced by its id.
func definitionOf(w domain.Workflow, version int) port.Definition {
	def := port.Definition{ExpectedVersion: version, InitialRef: w.Initial, Mapping: map[uuid.UUID]uuid.UUID{}}
	for _, s := range w.Statuses {
		def.Statuses = append(def.Statuses, port.StatusInput{Ref: s.ID, ID: uuid.MustParse(s.ID)})
	}
	def.Transitions = append(def.Transitions, w.Transitions...)
	return def
}

// dropStatus removes a status and every transition that starts or ends on it.
func dropStatus(def port.Definition, ref string) port.Definition {
	out := port.Definition{ExpectedVersion: def.ExpectedVersion, InitialRef: def.InitialRef, Mapping: def.Mapping}
	for _, s := range def.Statuses {
		if s.Ref != ref {
			out.Statuses = append(out.Statuses, s)
		}
	}
	for _, t := range def.Transitions {
		if t.To == ref {
			continue
		}
		drop := false
		for _, from := range t.From {
			if from == ref {
				drop = true
			}
		}
		if !drop {
			out.Transitions = append(out.Transitions, t)
		}
	}
	return out
}

func codeOf(err error) string { return apperror.CodeOf(err) }

func failureOf(t *testing.T, err error) *port.ValidationFailure {
	t.Helper()
	var vf *port.ValidationFailure
	if !errors.As(err, &vf) {
		t.Fatalf("error = %v, want a ValidationFailure", err)
	}
	return vf
}

func TestGetWorkflowIsVisibleToMembersOnly(t *testing.T) {
	f := newFixture(t)
	f.repo.AddIssues(f.project, uuidOf(t, statusIDByName(t, f.workflow(), "En curso")), 2)

	view, err := f.svc.GetWorkflow(context.Background(), f.contributor, "user", f.project)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Statuses) != 4 || view.Version != 1 || !view.IsTemplateDefault {
		t.Fatalf("view = %d statuses, version %d, default %v", len(view.Statuses), view.Version, view.IsTemplateDefault)
	}
	counts := map[string]int{}
	for _, s := range view.Statuses {
		counts[s.Name] = s.IssueCount
	}
	if counts["En curso"] != 2 || counts["Hecho"] != 0 {
		t.Fatalf("issue counts = %v", counts)
	}

	if _, err := f.svc.GetWorkflow(context.Background(), f.outsider, "user", f.project); apperror.KindOf(err) != apperror.KindNotFound {
		t.Fatalf("outsider: %v, want not found", err)
	}
	if _, err := f.svc.GetWorkflow(context.Background(), f.lead, "user", uuid.New()); apperror.KindOf(err) != apperror.KindNotFound {
		t.Fatalf("unknown project: %v, want not found", err)
	}
}

func uuidOf(t *testing.T, text string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestValidateIsADryRunWithPathsAndMapping(t *testing.T) {
	f := newFixture(t)
	reviewID := statusIDByName(t, f.workflow(), "En revisión")
	f.repo.AddIssues(f.project, uuidOf(t, reviewID), 3)
	before := f.repo.Writes

	def := dropStatus(definitionOf(f.workflow(), f.version()), reviewID)
	report, err := f.svc.ValidateWorkflow(context.Background(), f.lead, "user", f.project, def)
	if err != nil {
		t.Fatal(err)
	}
	if report.Valid || len(report.MappingRequired) != 1 || report.MappingRequired[0].IssueCount != 3 {
		t.Fatalf("report = valid %v, mapping %+v", report.Valid, report.MappingRequired)
	}
	if f.repo.Writes != before || f.version() != 1 {
		t.Fatalf("dry run wrote data")
	}

	broken := definitionOf(f.workflow(), f.version())
	broken.Transitions[0].To = "nowhere"
	report, err = f.svc.ValidateWorkflow(context.Background(), f.lead, "user", f.project, broken)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, issue := range report.Errors {
		if issue.Path == "transitions[0].to" && issue.Code == domain.IssueUnknownStatus {
			found = true
		}
	}
	if report.Valid || !found {
		t.Fatalf("errors = %+v, want transitions[0].to unknown status", report.Errors)
	}
}

func TestReplaceNeedsMappingForRemovedStatusWithIssues(t *testing.T) {
	f := newFixture(t)
	reviewID := statusIDByName(t, f.workflow(), "En revisión")
	f.repo.AddIssues(f.project, uuidOf(t, reviewID), 2)

	def := dropStatus(definitionOf(f.workflow(), f.version()), reviewID)
	_, err := f.svc.ReplaceWorkflow(context.Background(), f.lead, "user", f.project, def)
	if codeOf(err) != "workflow.status_mapping_required" {
		t.Fatalf("code = %q, err = %v", codeOf(err), err)
	}
	if vf := failureOf(t, err); len(vf.Mapping) != 1 || vf.Mapping[0].StatusID.String() != reviewID {
		t.Fatalf("mapping = %+v", vf.Mapping)
	}
	if f.version() != 1 {
		t.Fatalf("refused replace bumped the version to %d", f.version())
	}
}

func TestReplaceMovesIssuesWithHistoryAndActivity(t *testing.T) {
	f := newFixture(t)
	reviewID := statusIDByName(t, f.workflow(), "En revisión")
	progressID := statusIDByName(t, f.workflow(), "En curso")
	f.repo.AddIssues(f.project, uuidOf(t, reviewID), 3)

	def := dropStatus(definitionOf(f.workflow(), f.version()), reviewID)
	def.Mapping[uuidOf(t, reviewID)] = uuidOf(t, progressID)
	result, err := f.svc.ReplaceWorkflow(context.Background(), f.lead, "user", f.project, def)
	if err != nil {
		t.Fatal(err)
	}
	if f.version() != 2 || result.Workflow.Version != 2 {
		t.Fatalf("version = %d, want 2", f.version())
	}
	if _, ok := f.workflow().Status(reviewID); ok {
		t.Fatal("removed status is still in the workflow")
	}
	counts := f.repo.IssueStatus(f.project)
	if counts[uuidOf(t, progressID)] != 3 || counts[uuidOf(t, reviewID)] != 0 {
		t.Fatalf("issues per status = %v", counts)
	}
	if f.repo.Activities != 3 || f.repo.HistoryRefs[uuidOf(t, progressID)] != 3 {
		t.Fatalf("activities = %d, history = %v; want one per moved issue", f.repo.Activities, f.repo.HistoryRefs)
	}
	if !warningsContain(result.Warnings, "workflow.issues_migrated") {
		t.Fatalf("warnings = %+v, want workflow.issues_migrated", result.Warnings)
	}
}

func warningsContain(issues []domain.Issue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func TestReplaceCreatesNewStatusByName(t *testing.T) {
	f := newFixture(t)
	todo := statusIDByName(t, f.workflow(), "Por hacer")
	def := definitionOf(f.workflow(), f.version())
	def.Statuses = append(def.Statuses, port.StatusInput{Ref: "blocked", Name: "Bloqueado", Category: domain.CategoryInProgress})
	def.Transitions = append(def.Transitions, domain.Transition{ID: "block", Name: "Bloquear", From: []string{todo}, To: "blocked"})

	if _, err := f.svc.ReplaceWorkflow(context.Background(), f.lead, "user", f.project, def); err != nil {
		t.Fatal(err)
	}
	created, ok := f.repo.StatusByName(f.org1, "Bloqueado")
	if !ok {
		t.Fatal("status Bloqueado was not created in the organization")
	}
	if _, ok := f.workflow().Status(created.ID.String()); !ok {
		t.Fatal("workflow does not use the created status")
	}
}

func TestReplaceVersionConflict(t *testing.T) {
	f := newFixture(t)
	def := definitionOf(f.workflow(), 7)
	_, err := f.svc.ReplaceWorkflow(context.Background(), f.lead, "user", f.project, def)
	if codeOf(err) != "workflow.version_conflict" {
		t.Fatalf("code = %q, err = %v", codeOf(err), err)
	}
	if apperror.KindOf(err) != apperror.KindConflict {
		t.Fatalf("kind = %v, want conflict", apperror.KindOf(err))
	}
}

func TestReplaceRequiresVersionAndPermission(t *testing.T) {
	f := newFixture(t)
	def := definitionOf(f.workflow(), 0)
	if _, err := f.svc.ReplaceWorkflow(context.Background(), f.lead, "user", f.project, def); codeOf(err) != "workflow.version_required" {
		t.Fatalf("missing version: %v", err)
	}

	def = definitionOf(f.workflow(), 1)
	for name, actor := range map[string]uuid.UUID{"contributor": f.contributor, "viewer": f.viewer, "outsider": f.outsider} {
		_, err := f.svc.ReplaceWorkflow(context.Background(), actor, "user", f.project, def)
		want := apperror.KindForbidden
		if name == "outsider" {
			want = apperror.KindNotFound
		}
		if apperror.KindOf(err) != want {
			t.Fatalf("%s: kind = %v, want %v", name, apperror.KindOf(err), want)
		}
	}
	if f.repo.Writes != 0 {
		t.Fatalf("refused requests wrote %d times", f.repo.Writes)
	}
}

func TestReplaceAllowsOrgAdminAndRefusesArchivedProject(t *testing.T) {
	f := newFixture(t)
	def := definitionOf(f.workflow(), 1)
	if _, err := f.svc.ReplaceWorkflow(context.Background(), f.orgAdmin, "user", f.project, def); err != nil {
		t.Fatalf("org admin: %v", err)
	}
	f.authz.Archived[f.project] = true
	_, err := f.svc.ReplaceWorkflow(context.Background(), f.lead, "user", f.project, definitionOf(f.workflow(), f.version()))
	if codeOf(err) != "project.archived" {
		t.Fatalf("archived project: %v", err)
	}
	if _, err := f.svc.GetWorkflow(context.Background(), f.lead, "user", f.project); err != nil {
		t.Fatalf("archived project must stay readable: %v", err)
	}
}

func TestResetAppliesTemplateWithMapping(t *testing.T) {
	f := newFixture(t)
	reviewID := statusIDByName(t, f.workflow(), "En revisión")
	progressID := statusIDByName(t, f.workflow(), "En curso")
	f.repo.AddIssues(f.project, uuidOf(t, reviewID), 1)

	_, err := f.svc.ResetWorkflow(context.Background(), f.lead, "user", f.project, "task_tracking", 1, nil)
	if codeOf(err) != "workflow.status_mapping_required" {
		t.Fatalf("reset without mapping: %v", err)
	}
	mapping := map[uuid.UUID]uuid.UUID{uuidOf(t, reviewID): uuidOf(t, progressID)}
	result, err := f.svc.ResetWorkflow(context.Background(), f.lead, "user", f.project, "task_tracking", 1, mapping)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Workflow.Statuses) != 3 || f.version() != 2 {
		t.Fatalf("reset = %d statuses, version %d", len(result.Workflow.Statuses), f.version())
	}
	if _, err := f.svc.ResetWorkflow(context.Background(), f.lead, "user", f.project, "nope", 2, nil); codeOf(err) != "workflow.unknown_template" {
		t.Fatalf("unknown template: %v", err)
	}
}

func TestTemplatesAreListedWithSpanishLabels(t *testing.T) {
	f := newFixture(t)
	templates := f.svc.ListTemplates(context.Background())
	if len(templates) != 4 {
		t.Fatalf("templates = %d, want 4", len(templates))
	}
	names := map[string]string{}
	for _, tpl := range templates {
		names[tpl.ID] = tpl.Name
		if tpl.Description == "" || len(tpl.Definition.Statuses) == 0 {
			t.Fatalf("template %s has no description or statuses", tpl.ID)
		}
	}
	if names["task_tracking"] != "Seguimiento de tareas" {
		t.Fatalf("names = %v", names)
	}
}

func TestMigrateIssuesMovesExplicitlyAndChecksTargets(t *testing.T) {
	f := newFixture(t)
	reviewID := statusIDByName(t, f.workflow(), "En revisión")
	progressID := statusIDByName(t, f.workflow(), "En curso")
	f.repo.AddIssues(f.project, uuidOf(t, reviewID), 4)

	result, err := f.svc.MigrateIssues(context.Background(), f.lead, "user", f.project, map[uuid.UUID]uuid.UUID{uuidOf(t, reviewID): uuidOf(t, progressID)})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 4 || len(result.Moves) != 1 {
		t.Fatalf("result = %+v", result)
	}

	outside, err := f.svc.CreateStatus(context.Background(), f.orgAdmin, f.org1, "Archivado", domain.CategoryDone)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.MigrateIssues(context.Background(), f.lead, "user", f.project, map[uuid.UUID]uuid.UUID{uuidOf(t, progressID): outside.ID})
	if codeOf(err) != "workflow.invalid_status_mapping" {
		t.Fatalf("target outside the workflow: %v", err)
	}
	_, err = f.svc.MigrateIssues(context.Background(), f.lead, "user", f.project, map[uuid.UUID]uuid.UUID{uuidOf(t, progressID): f.otherStatus(t)})
	if codeOf(err) != "workflow.invalid_status_mapping" {
		t.Fatalf("status of another organization: %v", err)
	}
	if _, err := f.svc.MigrateIssues(context.Background(), f.contributor, "user", f.project, nil); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("contributor: %v", err)
	}
}

// otherStatus returns a status of the second organization.
func (f *fixture) otherStatus(t *testing.T) uuid.UUID {
	t.Helper()
	list, err := f.repo.ListStatuses(context.Background(), f.org2)
	if err != nil || len(list) == 0 {
		t.Fatalf("no status in the second organization: %v", err)
	}
	return list[0].ID
}

func TestOrganizationStatusesReadAndWritePermissions(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.svc.ListStatuses(ctx, f.orgMember, f.org1); err != nil {
		t.Fatalf("member read: %v", err)
	}
	if _, err := f.svc.ListStatuses(ctx, f.outsider, f.org1); apperror.KindOf(err) != apperror.KindNotFound {
		t.Fatalf("outsider read: %v", err)
	}
	if _, err := f.svc.CreateStatus(ctx, f.orgMember, f.org1, "Nuevo", domain.CategoryTodo); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("member write: %v", err)
	}
	if _, err := f.svc.CreateStatus(ctx, f.orgAdmin, f.org1, "hecho", domain.CategoryDone); codeOf(err) != "workflow.status_name_taken" {
		t.Fatalf("duplicate name: %v", err)
	}
	if _, err := f.svc.CreateStatus(ctx, f.orgAdmin, f.org1, "  ", domain.CategoryDone); codeOf(err) != "workflow.status_name_invalid" {
		t.Fatalf("blank name: %v", err)
	}
	created, err := f.svc.CreateStatus(ctx, f.orgAdmin, f.org1, "Bloqueado", domain.CategoryInProgress)
	if err != nil || created.Name != "Bloqueado" {
		t.Fatalf("create: %v", err)
	}
}

func TestRenameUpdatesWorkflowsAndCategoryChangeIsRefusedWhenInvalid(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	doneID := uuidOf(t, statusIDByName(t, f.workflow(), "Hecho"))

	renamed := "Completado"
	if _, err := f.svc.UpdateStatus(ctx, f.orgAdmin, f.org1, doneID, port.StatusPatch{Name: &renamed}); err != nil {
		t.Fatal(err)
	}
	if statusIDByName(t, f.workflow(), "Completado") == "" || f.version() != 2 {
		t.Fatalf("rename did not reach the workflow, version %d", f.version())
	}

	// Scrum has a single done status: moving it to in progress would leave the workflow without a done status.
	inProgress := domain.CategoryInProgress
	_, err := f.svc.UpdateStatus(ctx, f.orgAdmin, f.org1, doneID, port.StatusPatch{Category: &inProgress})
	if codeOf(err) != "workflow.category_in_use" {
		t.Fatalf("invalidating category change: %v", err)
	}
	stored, _ := f.repo.ListStatuses(ctx, f.org1)
	for _, s := range stored {
		if s.ID == doneID && s.Category != domain.CategoryDone {
			t.Fatal("refused category change was stored")
		}
	}
}

func TestDeleteStatusNeedsNoUseOrReassignment(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	progressID := uuidOf(t, statusIDByName(t, f.workflow(), "En curso"))
	reviewID := uuidOf(t, statusIDByName(t, f.workflow(), "En revisión"))
	f.repo.AddIssues(f.project, progressID, 2)

	if _, err := f.svc.DeleteStatus(ctx, f.orgAdmin, f.org1, progressID, uuid.Nil); codeOf(err) != "workflow.status_in_use" {
		t.Fatalf("delete in use: %v", err)
	}
	// Reassigning onto a status of the flow would turn transitions into self-transitions: refused.
	if _, err := f.svc.DeleteStatus(ctx, f.orgAdmin, f.org1, reviewID, progressID); codeOf(err) != "workflow.reassign_invalid" {
		t.Fatalf("reassign onto a flow status that merges transitions: %v", err)
	}
	// Without issues or history, a status outside every workflow is deleted without reassignment.
	orphan, err := f.svc.CreateStatus(ctx, f.orgAdmin, f.org1, "Huérfano", domain.CategoryTodo)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.DeleteStatus(ctx, f.orgAdmin, f.org1, orphan.ID, uuid.Nil); err != nil {
		t.Fatalf("delete unused status: %v", err)
	}
}

func TestDeleteStatusReassignsIssuesAndWorkflows(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	progressID := uuidOf(t, statusIDByName(t, f.workflow(), "En curso"))
	f.repo.AddIssues(f.project, progressID, 2)
	blocked, err := f.svc.CreateStatus(ctx, f.orgAdmin, f.org1, "Bloqueado", domain.CategoryInProgress)
	if err != nil {
		t.Fatal(err)
	}
	result, err := f.svc.DeleteStatus(ctx, f.orgAdmin, f.org1, progressID, blocked.ID)
	if err != nil {
		t.Fatalf("reassign: %v", err)
	}
	if result.ReassignedIssues != 2 {
		t.Fatalf("reassigned = %d, want 2", result.ReassignedIssues)
	}
	if _, ok := f.workflow().Status(progressID.String()); ok {
		t.Fatal("deleted status is still in the workflow")
	}
	if _, ok := f.workflow().Status(blocked.ID.String()); !ok {
		t.Fatal("workflow does not use the reassignment target")
	}
	if f.repo.IssueStatus(f.project)[blocked.ID] != 2 {
		t.Fatalf("issues per status = %v", f.repo.IssueStatus(f.project))
	}
	if err := f.workflow().Check(); err != nil {
		t.Fatalf("workflow after reassignment is invalid: %v", err)
	}
}

func TestReorderNeedsEveryStatusExactlyOnce(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	list, _ := f.repo.ListStatuses(ctx, f.org1)
	ids := make([]uuid.UUID, 0, len(list))
	for i := len(list) - 1; i >= 0; i-- {
		ids = append(ids, list[i].ID)
	}
	if _, err := f.svc.ReorderStatuses(ctx, f.orgAdmin, f.org1, ids[:2]); codeOf(err) != "workflow.status_order_invalid" {
		t.Fatalf("short list: %v", err)
	}
	reordered, err := f.svc.ReorderStatuses(ctx, f.orgAdmin, f.org1, ids)
	if err != nil {
		t.Fatal(err)
	}
	if reordered[0].ID != ids[0] {
		t.Fatalf("first position = %s, want %s", reordered[0].ID, ids[0])
	}
}

func TestCrossOrganizationIsolation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	foreign := f.otherStatus(t)

	// An admin of the second organization cannot touch the first organization's catalog.
	if _, err := f.svc.UpdateStatus(ctx, f.orgTwoAdmin, f.org1, foreign, port.StatusPatch{}); apperror.KindOf(err) != apperror.KindNotFound {
		t.Fatalf("foreign admin update: %v", err)
	}
	if _, err := f.svc.DeleteStatus(ctx, f.orgTwoAdmin, f.org1, uuidOf(t, statusIDByName(t, f.workflow(), "Hecho")), uuid.Nil); apperror.KindOf(err) != apperror.KindNotFound {
		t.Fatalf("foreign admin delete: %v", err)
	}

	// A workflow cannot use a status of another organization.
	def := definitionOf(f.workflow(), 1)
	def.Statuses[0] = port.StatusInput{Ref: def.Statuses[0].Ref, ID: foreign}
	report, err := f.svc.ValidateWorkflow(ctx, f.lead, "user", f.project, def)
	if err != nil {
		t.Fatal(err)
	}
	if report.Valid {
		t.Fatal("definition with a foreign status was reported valid")
	}

	// Reassignment cannot move issues into another organization.
	if _, err := f.svc.DeleteStatus(ctx, f.orgAdmin, f.org1, uuidOf(t, statusIDByName(t, f.workflow(), "Hecho")), foreign); codeOf(err) != "workflow.reassign_invalid" {
		t.Fatalf("reassign to foreign status: %v", err)
	}
	list, _ := f.svc.ListStatuses(ctx, f.orgAdmin, f.org1)
	for _, s := range list {
		if s.OrganizationID != f.org1 {
			t.Fatal("catalog listing leaks statuses of another organization")
		}
	}
}

func TestReplaceRefusesInvalidDefinitionWithPaths(t *testing.T) {
	f := newFixture(t)
	def := definitionOf(f.workflow(), f.version())
	def.Transitions[0].To = "nowhere"
	_, err := f.svc.ReplaceWorkflow(context.Background(), f.lead, "user", f.project, def)
	if codeOf(err) != "workflow.invalid" {
		t.Fatalf("code = %q, err = %v", codeOf(err), err)
	}
	vf := failureOf(t, err)
	if len(vf.Issues) == 0 || vf.Issues[0].Path != "transitions[0].to" {
		t.Fatalf("issues = %+v, want transitions[0].to first", vf.Issues)
	}
	if f.repo.Writes != 0 || f.version() != 1 {
		t.Fatalf("refused definition was written (writes %d, version %d)", f.repo.Writes, f.version())
	}
}

func TestReplaceFailsWhenTheProjectHasNoWorkflowRow(t *testing.T) {
	f := newFixture(t)
	delete(f.repo.Workflows, f.project)
	_, err := f.svc.ReplaceWorkflow(context.Background(), f.lead, "user", f.project, definitionOf(port.Record{}.Workflow, 1))
	if codeOf(err) != "workflow.not_found" {
		t.Fatalf("code = %q, err = %v", codeOf(err), err)
	}
}
