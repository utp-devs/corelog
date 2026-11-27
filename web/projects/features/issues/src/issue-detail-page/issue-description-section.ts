import { ChangeDetectionStrategy, Component, input, signal } from '@angular/core';
import { FormControl, FormGroup, ReactiveFormsModule } from '@angular/forms';
import { Button } from 'primeng/button';
import { InputNumber } from 'primeng/inputnumber';
import { InputText } from 'primeng/inputtext';
import {
  ISSUE_TYPE_LABELS,
  IssueType,
  MentionUser,
  Issue,
  UpdateIssueDetailsInput,
} from '@corelog/core';
import { DateField } from '@corelog/ui/date-field';
import { FieldControl, FieldRow } from '@corelog/ui/field-row';
import { RichTextEditor } from '@corelog/ui/rich-text-editor';
import { RichTextView } from '@corelog/ui/rich-text-view';
import { Section, SectionActions } from '@corelog/ui/section';
import { SelectField } from '@corelog/ui/select-field';

import { payloadFrom } from '../shared/issue-payload';

const ISSUE_TYPE_OPTIONS = (Object.keys(ISSUE_TYPE_LABELS) as IssueType[]).map((value) => ({
  value,
  label: ISSUE_TYPE_LABELS[value],
}));

/**
 * Description of an issue: rendered text by default, and an edit form with the detail fields
 * (description, type, due date, story points, parent issue). The save callback reports success.
 */
@Component({
  selector: 'app-issue-description-section',
  imports: [
    Button,
    DateField,
    FieldControl,
    FieldRow,
    InputNumber,
    InputText,
    ReactiveFormsModule,
    RichTextEditor,
    RichTextView,
    Section,
    SectionActions,
    SelectField,
  ],
  templateUrl: './issue-description-section.html',
  styleUrl: './issue-description-section.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class IssueDescriptionSection {
  readonly issue = input.required<Issue>();
  /** Saves the detail fields. Resolves true when the issue was saved and the form may close. */
  readonly saveDetails = input.required<(input: UpdateIssueDetailsInput) => Promise<boolean>>();
  /** The issue cannot be changed (archived project): the edit action is not offered. */
  readonly readOnly = input(false);
  readonly searchUsers = input<(query: string) => Promise<MentionUser[]>>();

  protected readonly issueTypeOptions = ISSUE_TYPE_OPTIONS;
  protected readonly editing = signal(false);
  protected readonly saving = signal(false);

  protected readonly form = new FormGroup({
    description: new FormControl('', { nonNullable: true }),
    issueType: new FormControl<IssueType>('task', { nonNullable: true }),
    parentId: new FormControl('', { nonNullable: true }),
    dueDate: new FormControl<string | null>(null),
    storyPoints: new FormControl<number | null>(null),
  });

  protected startEdit(): void {
    const issue = this.issue();
    this.form.reset({
      description: issue.description,
      issueType: issue.issueType,
      parentId: issue.parentId ?? '',
      dueDate: issue.dueDate ? issue.dueDate.slice(0, 10) : null,
      storyPoints: issue.storyPoints,
    });
    this.editing.set(true);
  }

  /** The stored value when the day did not change; otherwise the chosen day at midnight UTC. */
  private dueDatePayload(day: string | null): string | undefined {
    if (!day) return undefined;
    const stored = this.issue().dueDate;
    return stored && stored.slice(0, 10) === day ? stored : `${day}T00:00:00Z`;
  }

  protected cancelEdit(): void {
    this.editing.set(false);
  }

  protected async save(): Promise<void> {
    const values = this.form.getRawValue();
    this.saving.set(true);
    this.form.controls.description.disable({ emitEvent: false });
    let saved: boolean;
    try {
      saved = await this.saveDetails()(
        payloadFrom(this.issue(), {
          description: values.description,
          issueType: values.issueType,
          parentId: values.parentId || undefined,
          dueDate: this.dueDatePayload(values.dueDate),
          storyPoints: values.storyPoints ?? undefined,
        }),
      );
    } finally {
      this.form.controls.description.enable({ emitEvent: false });
      this.saving.set(false);
    }
    if (saved) this.editing.set(false);
  }
}
