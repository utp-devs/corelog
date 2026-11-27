package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/events"
)

// ListIssueLinks returns the links of an issue whose other end the caller can browse. Links to issues in projects
// the caller cannot see are left out.
func (s *IssueService) ListIssueLinks(ctx context.Context, issueID, actorID uuid.UUID, globalRole string) ([]port.LinkedIssue, error) {
	issue, err := s.repo.FindByID(ctx, issueID)
	if err != nil {
		return nil, err
	}
	if err := s.requireVisible(ctx, issue.ProjectID, actorID, globalRole); err != nil {
		return nil, err
	}
	out := []port.LinkedIssue{}
	if s.issueLinks == nil {
		return out, nil
	}
	links, err := s.issueLinks.ListLinks(ctx, issueID)
	if err != nil {
		return nil, err
	}
	visible := map[uuid.UUID]bool{}
	for _, link := range links {
		ok, seen := visible[link.Other.ProjectID]
		if !seen {
			err := s.requireIssueView(ctx, link.Other.ProjectID, actorID, globalRole)
			switch {
			case err == nil:
				ok = true
			case apperror.KindOf(err) == apperror.KindForbidden:
				ok = false
			default:
				return nil, err
			}
			visible[link.Other.ProjectID] = ok
		}
		if ok {
			out = append(out, link)
		}
	}
	return out, nil
}

// CreateIssueLink links the source issue to the target. The caller needs edit on the source and browse on the
// target. A target outside the organization, or one the caller cannot see, is reported as not found.
func (s *IssueService) CreateIssueLink(ctx context.Context, sourceID, actorID uuid.UUID, globalRole, targetRef string, kind domain.IssueLinkKind) (port.LinkedIssue, error) {
	if s.issueLinks == nil {
		return port.LinkedIssue{}, domain.ErrInvalidIssueLink
	}
	source, err := s.issueLinks.FindLinkable(ctx, sourceID)
	if err != nil {
		return port.LinkedIssue{}, err
	}
	if err := s.requireVisible(ctx, source.ProjectID, actorID, globalRole); err != nil {
		return port.LinkedIssue{}, err
	}
	if err := s.requireIssueEdit(ctx, source.ProjectID, actorID, globalRole); err != nil {
		return port.LinkedIssue{}, err
	}
	targetID, err := s.resolveLinkTarget(ctx, source, actorID, globalRole, targetRef)
	if err != nil {
		return port.LinkedIssue{}, err
	}
	if targetID == sourceID {
		return port.LinkedIssue{}, domain.ErrSelfLink
	}
	target, err := s.issueLinks.FindLinkable(ctx, targetID)
	if err != nil {
		return port.LinkedIssue{}, err
	}
	if target.OrganizationID != source.OrganizationID {
		return port.LinkedIssue{}, domain.ErrIssueNotFound
	}
	if err := s.requireVisible(ctx, target.ProjectID, actorID, globalRole); err != nil {
		return port.LinkedIssue{}, err
	}
	link, err := domain.NewIssueLink(sourceID, targetID, actorID, kind, s.now())
	if err != nil {
		return port.LinkedIssue{}, err
	}
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		if s.lifecycle != nil {
			if err := s.lifecycle.LockForUpdate(ctx, sourceID); err != nil {
				return err
			}
		}
		exists, err := s.issueLinks.PairExists(ctx, sourceID, targetID, kind)
		if err != nil {
			return err
		}
		if exists {
			return domain.ErrDuplicateLink
		}
		if kind.ForbidsCycles() {
			reaches, err := s.issueLinks.BlocksReach(ctx, targetID, sourceID)
			if err != nil {
				return err
			}
			if reaches {
				return domain.ErrLinkCycle
			}
		}
		if err := s.issueLinks.Save(ctx, link); err != nil {
			return err
		}
		return s.recordLinkChange(ctx, source.ProjectID, link, actorID, "link_added", source.Key, target.Key, "issue.link_added")
	})
	if err != nil {
		return port.LinkedIssue{}, err
	}
	return port.LinkedIssue{LinkID: link.ID, Kind: kind, Outward: true, Other: target}, nil
}

// DeleteIssueLink removes a link of the issue in the path. The caller needs edit on that issue.
func (s *IssueService) DeleteIssueLink(ctx context.Context, issueID, linkID, actorID uuid.UUID, globalRole string) error {
	if s.issueLinks == nil {
		return domain.ErrIssueLinkNotFound
	}
	issue, err := s.issueLinks.FindLinkable(ctx, issueID)
	if err != nil {
		return err
	}
	if err := s.requireVisible(ctx, issue.ProjectID, actorID, globalRole); err != nil {
		return err
	}
	if err := s.requireIssueEdit(ctx, issue.ProjectID, actorID, globalRole); err != nil {
		return err
	}
	return s.tx.InTx(ctx, func(ctx context.Context) error {
		if s.lifecycle != nil {
			if err := s.lifecycle.LockForUpdate(ctx, issueID); err != nil {
				return err
			}
		}
		link, err := s.issueLinks.FindLinkForIssue(ctx, issueID, linkID)
		if err != nil {
			return err
		}
		if err := s.issueLinks.Delete(ctx, linkID); err != nil {
			return err
		}
		sourceKey, targetKey := issue.Key, s.linkEndKey(ctx, link.TargetIssueID)
		if link.TargetIssueID != issueID {
			sourceKey, targetKey = s.linkEndKey(ctx, link.SourceIssueID), issue.Key
		}
		return s.recordLinkChange(ctx, issue.ProjectID, link, actorID, "link_removed", sourceKey, targetKey, "issue.link_removed")
	})
}

// linkEndKey returns the key of a linked issue for history text. A missing issue falls back to its id.
func (s *IssueService) linkEndKey(ctx context.Context, id uuid.UUID) string {
	summary, err := s.issueLinks.FindLinkable(ctx, id)
	if err != nil {
		return id.String()
	}
	return summary.Key
}

// resolveLinkTarget reads the target as an issue UUID or as a key inside the organization of the source.
func (s *IssueService) resolveLinkTarget(ctx context.Context, source port.IssueSummary, actorID uuid.UUID, globalRole, ref string) (uuid.UUID, error) {
	if id, err := uuid.Parse(ref); err == nil {
		return id, nil
	}
	return s.ResolveKey(ctx, actorID, globalRole, source.OrganizationID, ref)
}

// recordLinkChange writes one activity on each end of the link and publishes the event on the source issue.
// It runs inside the caller's transaction.
func (s *IssueService) recordLinkChange(ctx context.Context, projectID uuid.UUID, link *domain.IssueLink, actorID uuid.UUID, action string, sourceKey, targetKey, eventType string) error {
	at := s.now()
	if s.collaboration != nil {
		if err := s.collaboration.SaveActivity(ctx, domain.NewLinkActivity(link.SourceIssueID, actorID, action, link.Kind, targetKey, at)); err != nil {
			return err
		}
		if err := s.collaboration.SaveActivity(ctx, domain.NewLinkActivity(link.TargetIssueID, actorID, action, link.Kind, sourceKey, at)); err != nil {
			return err
		}
	}
	if s.events == nil {
		return nil
	}
	sourceID, targetID, actor := link.SourceIssueID, link.TargetIssueID, actorID
	return s.events.Publish(ctx, events.Event{
		ProjectID: &projectID, EntityID: &sourceID, ActorID: &actor, Type: eventType, EntityType: "issue",
		Payload: map[string]any{
			"link_id": link.ID.String(), "kind": string(link.Kind),
			"source_issue_id": sourceID.String(), "target_issue_id": targetID.String(),
		},
		CreatedAt: at,
	})
}
