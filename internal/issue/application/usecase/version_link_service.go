package usecase

import (
	"context"
	"sort"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// platformAdminRole is the global role that may override the closed-version rules.
const platformAdminRole = "admin"

// WithVersionLinks fixes the repository of the version and component links of issues.
func (s *IssueService) WithVersionLinks(r port.VersionLinkRepository) *IssueService {
	s.versionLinks = r
	return s
}

// attachLinks loads the versions and components of the issues with one batch query per kind.
func (s *IssueService) attachLinks(ctx context.Context, issues ...*domain.Issue) error {
	if s.versionLinks == nil || len(issues) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, 0, len(issues))
	for _, issue := range issues {
		ids = append(ids, issue.ID)
	}
	links, err := s.versionLinks.LoadLinks(ctx, ids)
	if err != nil {
		return err
	}
	for _, issue := range issues {
		l := links[issue.ID]
		issue.FixVersions = l.FixVersions
		issue.AffectsVersions = l.AffectsVersions
		issue.Components = l.Components
	}
	return nil
}

// applyLinkFields applies the fix versions, affects versions and components of a patch. Nil values clear the set.
// Only versions that are newly linked are checked against the closed-version rules.
func (s *IssueService) applyLinkFields(ctx context.Context, issue *domain.Issue, in port.PatchInput, add func(field, from, to string)) error {
	if !in.FixVersions.Set && !in.AffectsVersions.Set && !in.Components.Set {
		return nil
	}
	if s.versionLinks == nil {
		return apperror.New(apperror.KindInternal, "version links are not available")
	}
	current, err := s.versionLinks.LoadLinks(ctx, []uuid.UUID{issue.ID})
	if err != nil {
		return err
	}
	before := current[issue.ID]
	override := in.OverrideClosedVersions && in.GlobalRole == platformAdminRole

	if in.FixVersions.Set {
		refs, err := s.replaceVersionKind(ctx, issue, domain.VersionLinkFix, in.FixVersions.Value, before.FixVersions, override)
		if err != nil {
			return err
		}
		add("fix_versions", domain.NamesText(before.FixVersions), domain.NamesText(refs))
		issue.FixVersions = refs
	}
	if in.AffectsVersions.Set {
		refs, err := s.replaceVersionKind(ctx, issue, domain.VersionLinkAffects, in.AffectsVersions.Value, before.AffectsVersions, override)
		if err != nil {
			return err
		}
		add("affects_versions", domain.NamesText(before.AffectsVersions), domain.NamesText(refs))
		issue.AffectsVersions = refs
	}
	if in.Components.Set {
		refs, err := s.replaceComponents(ctx, issue, in.Components.Value)
		if err != nil {
			return err
		}
		add("components", domain.NamesText(before.Components), domain.NamesText(refs))
		issue.Components = refs
	}
	return nil
}

// replaceVersionKind validates the requested versions of one kind and stores them. current are the versions
// already linked; they are kept even when their status would refuse a new link.
func (s *IssueService) replaceVersionKind(ctx context.Context, issue *domain.Issue, kind string, requested *[]uuid.UUID, current []domain.LinkedRef, override bool) ([]domain.LinkedRef, error) {
	ids, err := distinctIDs(requested, domain.MaxIssueVersionLinks, domain.ErrIssueVersionInvalid)
	if err != nil {
		return nil, err
	}
	versions, err := s.versionLinks.ResolveVersions(ctx, issue.ProjectID, ids)
	if err != nil {
		return nil, err
	}
	if len(versions) != len(ids) {
		return nil, domain.ErrIssueVersionInvalid
	}
	linked := make(map[uuid.UUID]bool, len(current))
	for _, ref := range current {
		linked[ref.ID] = true
	}
	refs := make([]domain.LinkedRef, 0, len(versions))
	for _, v := range versions {
		if !linked[v.ID] {
			check := domain.CheckAffectsAccepted
			if kind == domain.VersionLinkFix {
				check = domain.CheckFixAccepted
			}
			if err := check(v.Status, override); err != nil {
				return nil, err
			}
		}
		refs = append(refs, domain.LinkedRef{ID: v.ID, Name: v.Name})
	}
	sortRefs(refs)
	if err := s.versionLinks.ReplaceIssueVersions(ctx, issue.ID, kind, ids); err != nil {
		return nil, err
	}
	return refs, nil
}

// replaceComponents validates and stores the components of an issue.
func (s *IssueService) replaceComponents(ctx context.Context, issue *domain.Issue, requested *[]uuid.UUID) ([]domain.LinkedRef, error) {
	ids, err := distinctIDs(requested, domain.MaxIssueComponents, domain.ErrIssueComponentInvalid)
	if err != nil {
		return nil, err
	}
	refs, err := s.versionLinks.ResolveComponents(ctx, issue.ProjectID, ids)
	if err != nil {
		return nil, err
	}
	if len(refs) != len(ids) {
		return nil, domain.ErrIssueComponentInvalid
	}
	sortRefs(refs)
	if err := s.versionLinks.ReplaceIssueComponents(ctx, issue.ID, ids); err != nil {
		return nil, err
	}
	return refs, nil
}

// attachCreateComponents stores the components of a new issue. Invalid components reject the whole create.
func (s *IssueService) attachCreateComponents(ctx context.Context, issue *domain.Issue, requested []uuid.UUID) error {
	if len(requested) == 0 {
		return nil
	}
	if s.versionLinks == nil {
		return apperror.New(apperror.KindInternal, "version links are not available")
	}
	refs, err := s.replaceComponents(ctx, issue, &requested)
	if err != nil {
		return err
	}
	issue.Components = refs
	return nil
}

// componentAssignee resolves the default assignee a new issue gets from its components. Components are read in
// name order; the first one whose policy names someone wins. found is false when no component has an opinion.
func (s *IssueService) componentAssignee(ctx context.Context, projectID uuid.UUID, componentIDs []uuid.UUID) (assignee *uuid.UUID, found bool, err error) {
	if s.versionLinks == nil || len(componentIDs) == 0 {
		return nil, false, nil
	}
	defaults, err := s.versionLinks.ComponentDefaults(ctx, projectID, componentIDs)
	if err != nil {
		return nil, false, err
	}
	for _, component := range defaults {
		switch component.Policy {
		case domain.ComponentPolicyComponentLead:
			if component.LeadID != nil {
				return component.LeadID, true, nil
			}
		case domain.ComponentPolicyProjectLead:
			lead, err := s.projectLead(ctx, projectID)
			if err != nil {
				return nil, false, err
			}
			if lead != nil {
				return lead, true, nil
			}
		}
	}
	return nil, false, nil
}

// projectLead returns the lead of the project, read through the project defaults port. Nil when unknown.
func (s *IssueService) projectLead(ctx context.Context, projectID uuid.UUID) (*uuid.UUID, error) {
	if s.defaults == nil {
		return nil, nil
	}
	_, lead, err := s.defaults.DefaultAssigneePolicy(ctx, projectID)
	return lead, err
}

// distinctIDs turns an optional requested list into distinct ids within the limit. Nil means none.
func distinctIDs(requested *[]uuid.UUID, limit int, invalid error) ([]uuid.UUID, error) {
	var raw []uuid.UUID
	if requested != nil {
		raw = *requested
	}
	ids, ok := domain.DedupeIDs(raw, limit)
	if !ok {
		return nil, invalid
	}
	return ids, nil
}

func sortRefs(refs []domain.LinkedRef) {
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].Name != refs[j].Name {
			return refs[i].Name < refs[j].Name
		}
		return refs[i].ID.String() < refs[j].ID.String()
	})
}
