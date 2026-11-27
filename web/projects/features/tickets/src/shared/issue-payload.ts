import { Ticket, UpdateIssueDetailsInput } from '@corelog/core';

/** Copy of the issue detail fields, overriding only the given ones. */
export function payloadFrom(
  ticket: Ticket,
  overrides: Partial<UpdateIssueDetailsInput> = {},
): UpdateIssueDetailsInput {
  return {
    title: ticket.title,
    description: ticket.description,
    priority: ticket.priority,
    issueType: ticket.issueType,
    parentId: ticket.parentId ?? undefined,
    dueDate: ticket.dueDate ?? undefined,
    storyPoints: ticket.storyPoints ?? undefined,
    ...overrides,
  };
}
