import { Issue, UpdateIssueDetailsInput } from '@corelog/core';

/** Copy of the issue detail fields, overriding only the given ones. */
export function payloadFrom(
  issue: Issue,
  overrides: Partial<UpdateIssueDetailsInput> = {},
): UpdateIssueDetailsInput {
  return {
    summary: issue.summary,
    description: issue.description,
    priorityId: issue.priority.id,
    issueTypeId: issue.issueType.id,
    parentId: issue.parentId ?? undefined,
    dueDate: issue.dueDate ?? undefined,
    storyPoints: issue.storyPoints ?? undefined,
    ...overrides,
  };
}
