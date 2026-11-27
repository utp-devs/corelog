package usecase

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// CloneIssue copies an issue into the project named in the input (the original project by default). The copy gets
// a new key, the caller as reporter and a link of kind clones back to the original. Subtasks are copied one level
// deep. The whole copy is one transaction; attachment files are removed again when it fails.
func (s *IssueService) CloneIssue(ctx context.Context, sourceID, actorID uuid.UUID, globalRole string, in port.CloneInput) (*domain.Issue, error) {
	if s.ops == nil || s.ops.clones == nil || s.issueLinks == nil {
		return nil, errOperationsUnavailable()
	}
	include, err := domain.ParseCloneIncludes(in.Include)
	if err != nil {
		return nil, err
	}
	var summary string
	if strings.TrimSpace(in.Summary) != "" {
		if summary, err = domain.NormalizeSummary(in.Summary); err != nil {
			return nil, err
		}
	}
	var clone *domain.Issue
	var copiedFiles []string
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		source, err := s.repo.FindByID(ctx, sourceID)
		if err != nil {
			return err
		}
		if err := s.requireVisible(ctx, source.ProjectID, actorID, globalRole); err != nil {
			return err
		}
		target := source.ProjectID
		if in.ProjectID != uuid.Nil {
			target = in.ProjectID
		}
		if err := s.requireIssueCreate(ctx, target, actorID, globalRole); err != nil {
			return err
		}
		title := summary
		if title == "" {
			title = domain.DefaultCloneSummary(source.Summary)
		}
		clone, err = s.copyIssue(ctx, source, target, actorID, title)
		if err != nil {
			return err
		}
		if err := s.saveCloneLink(ctx, clone.ID, source.ID, actorID); err != nil {
			return err
		}
		if include[domain.CloneIncludeLinks] {
			if err := s.ops.clones.CopyLinks(ctx, source.ID, clone.ID, actorID); err != nil {
				return err
			}
		}
		if include[domain.CloneIncludeWatchers] {
			if err := s.ops.clones.CopyWatchers(ctx, source.ID, clone.ID); err != nil {
				return err
			}
		}
		if target == source.ProjectID {
			if err := s.ops.clones.CopyProjectScoped(ctx, source.ID, clone.ID); err != nil {
				return err
			}
		}
		if include[domain.CloneIncludeAttachments] {
			copiedFiles, err = s.copyAttachments(ctx, source.ID, clone.ID, actorID)
			if err != nil {
				return err
			}
		}
		if include[domain.CloneIncludeSubtasks] {
			if err := s.cloneSubtasks(ctx, source, clone, actorID, target); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		s.removeFiles(ctx, copiedFiles)
		return nil, err
	}
	return clone, nil
}

// copyIssue saves the copy of the fields of source in the target project. Values the target does not accept
// are replaced by its defaults or dropped.
func (s *IssueService) copyIssue(ctx context.Context, source *domain.Issue, target, actorID uuid.UUID, summary string) (*domain.Issue, error) {
	draft := domain.CloneDraft(source, target, actorID, summary, s.now())
	sameProject := target == source.ProjectID
	if sameProject {
		if source.ParentID != nil {
			parent, err := s.repo.FindByID(ctx, *source.ParentID)
			if err != nil {
				return nil, err
			}
			if err := domain.ParentRule(draft.HierarchyLevel(), true, parent.HierarchyLevel(), true); err != nil {
				return nil, err
			}
		}
	} else {
		draft.ParentID = nil
		if err := domain.ParentRule(draft.HierarchyLevel(), false, 0, true); err != nil {
			return nil, err
		}
	}
	if s.catalog != nil {
		if err := s.keepCatalogValues(ctx, draft, target, sameProject); err != nil {
			return nil, err
		}
	}
	if s.customScope != nil {
		scope, err := s.customScope.ScopeFor(ctx, target, draft.IssueTypeID)
		if err != nil {
			return nil, err
		}
		draft.Custom = domain.PruneCustomValues(source.Custom, scope)
	} else {
		for key, value := range source.Custom {
			draft.Custom[key] = value
		}
	}
	if s.lifecycle != nil {
		rank, err := s.lifecycle.NextRank(ctx, target)
		if err != nil {
			return nil, err
		}
		draft.Rank = rank
	}
	if err := s.repo.Save(ctx, draft); err != nil {
		return nil, err
	}
	if s.watchers != nil {
		if err := s.watchers.Add(ctx, draft.ID, actorID); err != nil {
			return nil, err
		}
	}
	if err := s.publishLifecycle(ctx, draft, actorID, "issue.created"); err != nil {
		return nil, err
	}
	return draft, nil
}

// keepCatalogValues keeps the type and priority of the source when the target accepts them, and clears them
// otherwise so the defaults of the target apply.
func (s *IssueService) keepCatalogValues(ctx context.Context, draft *domain.Issue, target uuid.UUID, sameProject bool) error {
	if sameProject {
		return nil
	}
	enabled, err := s.catalog.IssueTypeEnabled(ctx, target, draft.IssueTypeID)
	if err != nil {
		return err
	}
	if !enabled {
		draft.IssueTypeID = uuid.Nil
		draft.IssueTypeRef = nil
		draft.IssueType = domain.IssueTypeTask
	}
	if draft.PriorityID != uuid.Nil {
		_, err := s.catalog.Select(ctx, target, domain.CatalogPriority, port.CatalogSelector{ID: draft.PriorityID})
		switch {
		case err == nil:
		case apperror.KindOf(err) == apperror.KindNotFound:
			draft.PriorityID = uuid.Nil
			draft.PriorityRef = nil
			draft.Priority = domain.PriorityMedium
		default:
			return err
		}
	}
	return nil
}

// cloneSubtasks copies the live children of source under clone. Children keep their fields; they are not cloned
// further.
func (s *IssueService) cloneSubtasks(ctx context.Context, source, clone *domain.Issue, actorID, target uuid.UUID) error {
	if s.children == nil {
		return nil
	}
	children, err := s.children.ListChildren(ctx, source.ID)
	if err != nil {
		return err
	}
	for _, child := range children {
		childClone, err := s.copyIssue(ctx, child, target, actorID, child.Summary)
		if err != nil {
			return err
		}
		childClone.ParentID = &clone.ID
		if err := domain.ParentRule(childClone.HierarchyLevel(), true, clone.HierarchyLevel(), true); err != nil {
			return err
		}
		if err := s.repo.Update(ctx, childClone); err != nil {
			return err
		}
		if err := s.saveCloneLink(ctx, childClone.ID, child.ID, actorID); err != nil {
			return err
		}
	}
	return nil
}

// copyAttachments duplicates the attachment records of from on to, with their own copies of the files. It returns
// the storage keys it wrote so the caller can remove them when the copy fails.
func (s *IssueService) copyAttachments(ctx context.Context, fromID, toID, actorID uuid.UUID) ([]string, error) {
	if s.ops.attachments == nil || s.ops.files == nil {
		return nil, nil
	}
	items, err := s.ops.attachments.ListAttachments(ctx, fromID)
	if err != nil {
		return nil, err
	}
	var written []string
	for _, item := range items {
		copied := &domain.Attachment{
			IssueID: toID, UploaderID: item.UploaderID, FileName: item.FileName,
			ContentType: item.ContentType, Size: item.Size, StorageKey: uuid.NewString(), CreatedAt: s.now(),
		}
		reader, err := s.ops.files.Open(ctx, item.StorageKey)
		if err != nil {
			return written, err
		}
		_, err = s.ops.files.Save(ctx, copied.StorageKey, reader)
		_ = reader.Close()
		if err != nil {
			return written, err
		}
		written = append(written, copied.StorageKey)
		if err := s.ops.attachments.SaveAttachment(ctx, copied); err != nil {
			return written, err
		}
	}
	return written, nil
}

// removeFiles deletes attachment files written by a failed copy. Failures are ignored: the rows were never saved.
func (s *IssueService) removeFiles(ctx context.Context, keys []string) {
	if s.ops == nil || s.ops.files == nil {
		return
	}
	for _, key := range keys {
		_ = s.ops.files.Delete(ctx, key)
	}
}

// saveCloneLink records the clones link from the copy back to the original.
func (s *IssueService) saveCloneLink(ctx context.Context, cloneID, originalID, actorID uuid.UUID) error {
	link, err := domain.NewIssueLink(cloneID, originalID, actorID, domain.LinkKindClones, s.now())
	if err != nil {
		return err
	}
	return s.issueLinks.Save(ctx, link)
}
