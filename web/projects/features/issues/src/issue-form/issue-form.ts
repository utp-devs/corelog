import { ChangeDetectionStrategy, Component, effect, input, output, signal } from '@angular/core';
import { ReactiveFormsModule } from '@angular/forms';
import {
  ISSUE_SUMMARY_MAX_LENGTH,
  ISSUE_SUMMARY_MIN_LENGTH,
  IssueDraftFormGroup,
  MentionUser,
} from '@corelog/core';
import { FieldControl, FieldRow } from '@corelog/ui/field-row';
import { RichTextEditor } from '@corelog/ui/rich-text-editor';
import { Section } from '@corelog/ui/section';
import { SelectField, SelectFieldOption } from '@corelog/ui/select-field';
import { PickableUser, UserPicker } from '@corelog/ui/user-picker';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';

/**
 * New-issue form. From 992 px the main fields sit in the left column and the classification (project,
 * type, priority, assignee) in the right column; below, one column with the actions pinned to the bottom.
 * A field shows its error only after it was touched or the form was submitted, never while it is empty and
 * untouched.
 */
@Component({
  selector: 'app-issue-form',
  imports: [
    Button,
    FieldControl,
    FieldRow,
    InputText,
    ReactiveFormsModule,
    RichTextEditor,
    Section,
    SelectField,
    UserPicker,
  ],
  templateUrl: './issue-form.html',
  styleUrl: './issue-form.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class IssueForm {
  /** Form owned by the page, so the page can read its dirty state for the unsaved-changes guard. */
  readonly form = input.required<IssueDraftFormGroup>();
  readonly isSubmitting = input(false);
  readonly projectOptions = input<readonly SelectFieldOption[]>([]);
  readonly projectsLoading = input(false);
  /** Types of the chosen project and the organization priorities, with icon and color. */
  readonly typeOptions = input<readonly SelectFieldOption[]>([]);
  readonly priorityOptions = input<readonly SelectFieldOption[]>([]);
  readonly assigneeUsers = input<PickableUser[]>([]);
  readonly searchUsers = input<(query: string) => Promise<MentionUser[]>>();

  /** The user asked to submit a valid form. The page creates the issue. */
  readonly submitted = output<void>();
  readonly cancelled = output<void>();

  protected readonly summaryMaxLength = ISSUE_SUMMARY_MAX_LENGTH;
  protected readonly summaryMessages = {
    required: 'Escribe un resumen.',
    minlength: 'El resumen es demasiado corto',
    maxlength: `El resumen no puede superar ${ISSUE_SUMMARY_MAX_LENGTH} caracteres.`,
  };
  protected readonly descriptionMessages = {
    required: 'Describe la incidencia.',
    minlength: 'La descripción es demasiado corta',
  };
  protected readonly projectMessages = { required: 'Elige un proyecto.' };
  protected readonly summaryHelper = `Describe el problema en una frase (mínimo ${ISSUE_SUMMARY_MIN_LENGTH} caracteres).`;
  /** True after the first submit attempt: errors of untouched fields become visible. */
  protected readonly attempted = signal(false);

  constructor() {
    // The description editor follows its control: disabled while the page submits the issue.
    effect(() => {
      const control = this.form().controls.description;
      if (this.isSubmitting()) {
        control.disable({ emitEvent: false });
      } else {
        control.enable({ emitEvent: false });
      }
    });
  }

  protected setAssignee(userId: string | undefined): void {
    const control = this.form().controls.assigneeId;
    control.setValue(userId);
    control.markAsDirty();
  }

  protected submit(): void {
    this.attempted.set(true);
    const form = this.form();
    if (form.invalid) {
      form.markAllAsTouched();
      return;
    }
    this.submitted.emit();
  }
}
