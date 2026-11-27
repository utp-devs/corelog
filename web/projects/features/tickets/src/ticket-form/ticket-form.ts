import { ChangeDetectionStrategy, Component, input, output, signal } from '@angular/core';
import { FormControl, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';
import { Select } from 'primeng/select';
import {
  ISSUE_PRIORITY_LABELS,
  ISSUE_TYPE_LABELS,
  IssuePriority,
  IssueType,
  MentionUser,
} from '@corelog/core';
import { FieldControl, FieldRow } from '@corelog/ui/field-row';
import { RichTextEditor } from '@corelog/ui/rich-text-editor';
import { Section } from '@corelog/ui/section';

const TITLE_MAX_LENGTH = 200;
const TITLE_MIN_LENGTH = 3;
const DESCRIPTION_MIN_LENGTH = 3;
const TITLE_ERROR = 'El título es demasiado corto';
const DESCRIPTION_ERROR = 'La descripción es demasiado corta';

const ISSUE_TYPE_OPTIONS = (Object.keys(ISSUE_TYPE_LABELS) as IssueType[]).map((value) => ({
  value,
  label: ISSUE_TYPE_LABELS[value],
}));
const PRIORITY_OPTIONS = (Object.keys(ISSUE_PRIORITY_LABELS) as IssuePriority[]).map((value) => ({
  value,
  label: ISSUE_PRIORITY_LABELS[value],
}));

/** Reactive form of the new-issue fields, with the defaults used when the form is opened or reset. */
export function createTicketForm() {
  return new FormGroup({
    title: new FormControl('', {
      nonNullable: true,
      validators: [Validators.required, Validators.minLength(TITLE_MIN_LENGTH)],
    }),
    description: new FormControl('', {
      nonNullable: true,
      validators: [Validators.required, Validators.minLength(DESCRIPTION_MIN_LENGTH)],
    }),
    priority: new FormControl<IssuePriority>('medium', {
      nonNullable: true,
      validators: [Validators.required],
    }),
    issueType: new FormControl<IssueType>('task', {
      nonNullable: true,
      validators: [Validators.required],
    }),
  });
}

export type TicketFormGroup = ReturnType<typeof createTicketForm>;

/**
 * New-issue form. From 992 px the main fields sit in the left column and the classification in the
 * right column; below, one column. Errors appear after the first submit attempt.
 */
@Component({
  selector: 'app-ticket-form',
  imports: [
    Button,
    FieldControl,
    FieldRow,
    InputText,
    ReactiveFormsModule,
    RichTextEditor,
    Section,
    Select,
  ],
  templateUrl: './ticket-form.html',
  styleUrl: './ticket-form.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TicketForm {
  /** Form owned by the page, so the page can read its dirty state for the unsaved-changes guard. */
  readonly form = input.required<TicketFormGroup>();
  readonly isSubmitting = input(false);
  readonly searchUsers = input<(query: string) => Promise<MentionUser[]>>();

  /** The user asked to submit a valid form. The page creates the issue. */
  readonly submitted = output<void>();
  readonly cancelled = output<void>();

  protected readonly titleMaxLength = TITLE_MAX_LENGTH;
  protected readonly issueTypeOptions = ISSUE_TYPE_OPTIONS;
  protected readonly priorityOptions = PRIORITY_OPTIONS;
  protected readonly showErrors = signal(false);

  protected titleError(): string | undefined {
    return this.showErrors() && this.form().controls.title.invalid ? TITLE_ERROR : undefined;
  }

  protected descriptionError(): string | undefined {
    return this.showErrors() && this.form().controls.description.invalid
      ? DESCRIPTION_ERROR
      : undefined;
  }

  protected submit(): void {
    this.showErrors.set(true);
    const form = this.form();
    if (form.invalid) {
      form.markAllAsTouched();
      return;
    }
    this.submitted.emit();
  }
}
