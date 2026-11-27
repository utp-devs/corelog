import { FormControl, FormGroup, Validators } from '@angular/forms';

export const ISSUE_SUMMARY_MAX_LENGTH = 200;
export const ISSUE_SUMMARY_MIN_LENGTH = 3;
export const ISSUE_DESCRIPTION_MIN_LENGTH = 3;

/**
 * Reactive form of a new issue: where it goes (project), what it is (type, summary, description) and who
 * does it (priority, assignee). The type and priority hold catalog item ids; they start empty and
 * `injectIssueCatalogOptions` fills the defaults once the catalogs are loaded. An empty value lets the
 * server apply its default.
 */
export function createIssueDraftForm() {
  return new FormGroup({
    projectId: new FormControl('', { nonNullable: true, validators: [Validators.required] }),
    issueTypeId: new FormControl('', { nonNullable: true }),
    summary: new FormControl('', {
      nonNullable: true,
      validators: [
        Validators.required,
        Validators.minLength(ISSUE_SUMMARY_MIN_LENGTH),
        Validators.maxLength(ISSUE_SUMMARY_MAX_LENGTH),
      ],
    }),
    description: new FormControl('', {
      nonNullable: true,
      validators: [Validators.required, Validators.minLength(ISSUE_DESCRIPTION_MIN_LENGTH)],
    }),
    priorityId: new FormControl('', { nonNullable: true }),
    /** Empty leaves the issue unassigned. */
    assigneeId: new FormControl<string | undefined>(undefined, { nonNullable: true }),
  });
}

export type IssueDraftFormGroup = ReturnType<typeof createIssueDraftForm>;
