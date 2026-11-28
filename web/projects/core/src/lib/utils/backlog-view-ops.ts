import type {
  BacklogCategoryTotals,
  BacklogIssue,
  BacklogSection,
  BacklogTotals,
  BacklogView,
} from '../models/backlog-v2.model';
import type { StatusCategory } from '../models/board-v2.model';

const CATEGORIES: readonly StatusCategory[] = ['todo', 'in_progress', 'done'];

function sumOf(issues: readonly BacklogIssue[]): BacklogCategoryTotals {
  return {
    issues: issues.length,
    points: issues.reduce((total, issue) => total + (issue.storyPoints ?? 0), 0),
    originalEstimateSeconds: issues.reduce(
      (total, issue) => total + (issue.originalEstimateSeconds ?? 0),
      0,
    ),
  };
}

/** Totals of a list of issues: overall and by status category, as the server computes them. */
export function totalsOf(issues: readonly BacklogIssue[]): BacklogTotals {
  const byCategory: Partial<Record<StatusCategory, BacklogCategoryTotals>> = {};
  for (const category of CATEGORIES) {
    byCategory[category] = sumOf(issues.filter((issue) => issue.statusCategory === category));
  }
  return { ...sumOf(issues), byCategory };
}

/** Where the moved issues land in the target section: next to an anchor, or at the end. */
export interface BacklogAnchor {
  readonly beforeIssueId?: string;
  readonly afterIssueId?: string;
}

function sectionTargets(section: BacklogSection, sprintId: string | null): boolean {
  return sprintId === null ? section.kind === 'backlog' : section.sprint?.id === sprintId;
}

/**
 * The backlog after moving issues to a section, computed without the server. The issues keep the
 * order they are given in. Unknown ids are ignored; an unknown target returns the view unchanged.
 */
export function moveIssuesInBacklog(
  view: BacklogView,
  issueIds: readonly string[],
  sprintId: string | null,
  anchor: BacklogAnchor = {},
): BacklogView {
  if (!view.sections.some((section) => sectionTargets(section, sprintId))) return view;
  const byId = new Map<string, BacklogIssue>();
  for (const section of view.sections) {
    for (const issue of section.issues) byId.set(issue.id, issue);
  }
  const moving = issueIds
    .map((id) => byId.get(id))
    .filter((issue): issue is BacklogIssue => issue !== undefined);
  if (moving.length === 0) return view;
  const movingIds = new Set(moving.map((issue) => issue.id));

  const sections = view.sections.map((section) => {
    const remaining = section.issues.filter((issue) => !movingIds.has(issue.id));
    let issues = remaining;
    if (sectionTargets(section, sprintId)) {
      let index = remaining.length;
      if (anchor.beforeIssueId) {
        const found = remaining.findIndex((issue) => issue.id === anchor.beforeIssueId);
        if (found >= 0) index = found;
      } else if (anchor.afterIssueId) {
        const found = remaining.findIndex((issue) => issue.id === anchor.afterIssueId);
        if (found >= 0) index = found + 1;
      }
      issues = [...remaining.slice(0, index), ...moving, ...remaining.slice(index)];
    }
    if (
      issues === section.issues ||
      (issues.length === section.issues.length &&
        issues.every((issue, i) => issue === section.issues[i]))
    ) {
      return section;
    }
    return { ...section, issues, totals: totalsOf(issues) };
  });
  return { ...view, sections };
}

/** Same view with the story points of one issue replaced and the totals recomputed. */
export function setPointsInBacklog(
  view: BacklogView,
  issueId: string,
  points: number | null,
): BacklogView {
  const sections = view.sections.map((section) => {
    if (!section.issues.some((issue) => issue.id === issueId)) return section;
    const issues = section.issues.map((issue) =>
      issue.id === issueId ? { ...issue, storyPoints: points } : issue,
    );
    return { ...section, issues, totals: totalsOf(issues) };
  });
  return { ...view, sections };
}

/** Ids from `anchorId` to `targetId` inclusive, in list order: the range a shift-click selects. */
export function rangeBetween(ids: readonly string[], anchorId: string, targetId: string): string[] {
  const from = ids.indexOf(anchorId);
  const to = ids.indexOf(targetId);
  if (from < 0 || to < 0) return to >= 0 ? [targetId] : [];
  const [start, end] = from <= to ? [from, to] : [to, from];
  return ids.slice(start, end + 1);
}

/**
 * The anchor a drop leaves for the server: the first issue at or after `index` that is not itself moving
 * (the moved ones go before it), else the last one before `index` (they go after it), else none.
 */
export function anchorAt(
  ids: readonly string[],
  index: number,
  moving: ReadonlySet<string>,
): BacklogAnchor {
  for (let i = index; i < ids.length; i++) {
    if (!moving.has(ids[i])) return { beforeIssueId: ids[i] };
  }
  for (let i = Math.min(index, ids.length) - 1; i >= 0; i--) {
    if (!moving.has(ids[i])) return { afterIssueId: ids[i] };
  }
  return {};
}
