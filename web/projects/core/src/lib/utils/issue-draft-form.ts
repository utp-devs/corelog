import { FormControl, FormGroup, Validators } from '@angular/forms';

import { IssuePriority, IssueType } from '../models';
import { ISSUE_PRIORITY_LABELS, ISSUE_TYPE_LABELS } from './ticket-labels';

export const ISSUE_TITLE_MAX_LENGTH = 200;
export const ISSUE_TITLE_MIN_LENGTH = 3;
export const ISSUE_DESCRIPTION_MIN_LENGTH = 3;

/** Types offered when an issue is created on its own: a subtask needs a parent, so it is not listed. */
export const CREATABLE_ISSUE_TYPES: readonly IssueType[] = ['task', 'bug', 'story', 'epic'];

export interface IssueOption<T extends string> {
  readonly value: T;
  readonly label: string;
}

export const CREATABLE_ISSUE_TYPE_OPTIONS: IssueOption<IssueType>[] = CREATABLE_ISSUE_TYPES.map(
  (value) => ({ value, label: ISSUE_TYPE_LABELS[value] }),
);

export const ISSUE_PRIORITY_OPTIONS: IssueOption<IssuePriority>[] = (
  Object.keys(ISSUE_PRIORITY_LABELS) as IssuePriority[]
).map((value) => ({ value, label: ISSUE_PRIORITY_LABELS[value] }));

/**
 * Reactive form of a new issue: where it goes (project), what it is (type, summary, description) and who
 * does it (priority, assignee). Defaults match the full new-issue page.
 */
export function createIssueDraftForm() {
  return new FormGroup({
    projectId: new FormControl('', { nonNullable: true, validators: [Validators.required] }),
    issueType: new FormControl<IssueType>('task', {
      nonNullable: true,
      validators: [Validators.required],
    }),
    title: new FormControl('', {
      nonNullable: true,
      validators: [
        Validators.required,
        Validators.minLength(ISSUE_TITLE_MIN_LENGTH),
        Validators.maxLength(ISSUE_TITLE_MAX_LENGTH),
      ],
    }),
    description: new FormControl('', {
      nonNullable: true,
      validators: [Validators.required, Validators.minLength(ISSUE_DESCRIPTION_MIN_LENGTH)],
    }),
    priority: new FormControl<IssuePriority>('medium', {
      nonNullable: true,
      validators: [Validators.required],
    }),
    /** Empty leaves the issue unassigned. */
    assigneeId: new FormControl<string | undefined>(undefined, { nonNullable: true }),
  });
}

export type IssueDraftFormGroup = ReturnType<typeof createIssueDraftForm>;
