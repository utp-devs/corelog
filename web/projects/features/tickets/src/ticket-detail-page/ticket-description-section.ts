import { ChangeDetectionStrategy, Component, input, signal } from '@angular/core';
import { FormControl, FormGroup, ReactiveFormsModule } from '@angular/forms';
import { Button } from 'primeng/button';
import { DatePicker } from 'primeng/datepicker';
import { InputNumber } from 'primeng/inputnumber';
import { InputText } from 'primeng/inputtext';
import { Select } from 'primeng/select';
import {
  ISSUE_TYPE_LABELS,
  IssueType,
  MentionUser,
  Ticket,
  UpdateIssueDetailsInput,
} from '@corelog/core';
import { EmptyState } from '@corelog/ui/empty-state';
import { FieldControl, FieldRow } from '@corelog/ui/field-row';
import { RichTextEditor } from '@corelog/ui/rich-text-editor';
import { RichTextView } from '@corelog/ui/rich-text-view';
import { Section, SectionActions } from '@corelog/ui/section';

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
  selector: 'app-ticket-description-section',
  imports: [
    Button,
    DatePicker,
    EmptyState,
    FieldControl,
    FieldRow,
    InputNumber,
    InputText,
    ReactiveFormsModule,
    RichTextEditor,
    RichTextView,
    Section,
    SectionActions,
    Select,
  ],
  templateUrl: './ticket-description-section.html',
  styleUrl: './ticket-description-section.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TicketDescriptionSection {
  readonly ticket = input.required<Ticket>();
  /** Saves the detail fields. Resolves true when the issue was saved and the form may close. */
  readonly saveDetails = input.required<(input: UpdateIssueDetailsInput) => Promise<boolean>>();
  readonly searchUsers = input<(query: string) => Promise<MentionUser[]>>();

  protected readonly issueTypeOptions = ISSUE_TYPE_OPTIONS;
  protected readonly editing = signal(false);
  protected readonly saving = signal(false);

  protected readonly form = new FormGroup({
    description: new FormControl('', { nonNullable: true }),
    issueType: new FormControl<IssueType>('task', { nonNullable: true }),
    parentId: new FormControl('', { nonNullable: true }),
    dueDate: new FormControl<Date | null>(null),
    storyPoints: new FormControl<number | null>(null),
  });

  protected startEdit(): void {
    const ticket = this.ticket();
    this.form.reset({
      description: ticket.description,
      issueType: ticket.issueType,
      parentId: ticket.parentId ?? '',
      dueDate: ticket.dueDate ? new Date(ticket.dueDate) : null,
      storyPoints: ticket.storyPoints,
    });
    this.editing.set(true);
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
        payloadFrom(this.ticket(), {
          description: values.description,
          issueType: values.issueType,
          parentId: values.parentId || undefined,
          dueDate: values.dueDate?.toISOString(),
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
