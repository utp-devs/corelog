import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  input,
  output,
  signal,
  untracked,
} from '@angular/core';
import {
  FormControl,
  FormGroup,
  FormRecord,
  FormsModule,
  ReactiveFormsModule,
} from '@angular/forms';
import {
  ApiErrorDetail,
  CatalogStore,
  IssueTransition,
  OrganizationStore,
  fieldLabel,
  useOrgMembersById,
} from '@corelog/core';
import { DateField } from '@corelog/ui/date-field';
import { Dialog } from '@corelog/ui/dialog';
import { FieldControl, FieldRow } from '@corelog/ui/field-row';
import { SelectField, SelectFieldOption } from '@corelog/ui/select-field';
import { PickableUser, UserPicker } from '@corelog/ui/user-picker';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';
import { Message } from 'primeng/message';
import { Textarea } from 'primeng/textarea';

/** What the dialog hands back when the user confirms. Only what the transition asked for is filled. */
export interface TransitionDialogValue {
  readonly fields: Readonly<Record<string, string>>;
  readonly comment?: string;
  readonly resolutionId?: string;
  readonly assigneeId?: string;
}

type FieldKind = 'number' | 'date' | 'flag' | 'text';

interface FieldSpec {
  readonly name: string;
  readonly label: string;
  readonly kind: FieldKind;
}

const FIELD_KINDS: Readonly<Record<string, FieldKind>> = {
  story_points: 'number',
  due_date: 'date',
  start_date: 'date',
  flagged: 'flag',
};

/** Form control key of a field of the transition. */
function controlKey(name: string): string {
  return `f_${name}`;
}

/**
 * Asks for what a workflow transition needs before it runs: the required fields (rendered from their
 * type), a comment, a resolution and an assignee. The server's `workflow.validation_failed` details
 * are shown next to the fields they name. The dialog only emits the values; the owner runs the transition.
 */
@Component({
  selector: 'app-transition-dialog',
  imports: [
    Button,
    DateField,
    Dialog,
    FieldControl,
    FieldRow,
    FormsModule,
    InputText,
    Message,
    ReactiveFormsModule,
    SelectField,
    Textarea,
    UserPicker,
  ],
  templateUrl: './transition-dialog.html',
  styleUrl: './transition-dialog.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TransitionDialog {
  private readonly catalog = inject(CatalogStore);
  private readonly orgs = inject(OrganizationStore);
  private readonly members = useOrgMembersById(this.orgs.currentOrgId);

  readonly open = input(false);
  /** Transition being run; the dialog shows nothing without it. */
  readonly transition = input<IssueTransition | null>(null);
  /** Key and summary of the issue (or a count for a bulk run), as context under the title. */
  readonly context = input<string>();
  /** Id of the current assignee, kept as the default of the assignee picker. */
  readonly currentAssigneeId = input<string | null>(null);
  readonly busy = input(false);
  /** Failure of the last attempt that has no field to sit next to. */
  readonly error = input<string | null>(null);
  /** Server details (`comment`, `fields.<name>`, `resolution_id`, `assignee_id`). */
  readonly serverErrors = input<readonly ApiErrorDetail[]>([]);

  readonly confirmed = output<TransitionDialogValue>();
  readonly closeRequested = output<void>();

  protected readonly submitted = signal(false);
  protected readonly assigneeId = signal<string | undefined>(undefined);
  protected readonly resolutionId = signal<string | null>(null);

  protected readonly form = new FormGroup({
    comment: new FormControl('', { nonNullable: true }),
    fields: new FormRecord<FormControl<string>>({}),
  });

  protected readonly fieldSpecs = computed<FieldSpec[]>(() =>
    (this.transition()?.requiredFields ?? []).map((name) => ({
      name,
      label: fieldLabel(name),
      kind: FIELD_KINDS[name] ?? 'text',
    })),
  );
  protected readonly resolutionOptions = computed<SelectFieldOption[]>(() =>
    this.catalog.resolutions().map((item) => ({ value: item.id, label: item.name })),
  );
  protected readonly users = computed<PickableUser[]>(() =>
    Array.from(this.members.members(), ([id, member]) => ({ id, name: member.name })).sort((a, b) =>
      a.name.localeCompare(b.name, 'es'),
    ),
  );

  protected readonly commentRequired = computed(() => this.transition()?.requiresComment === true);
  protected readonly resolutionRequired = computed(
    () => this.transition()?.requiresResolution === true,
  );
  protected readonly assigneeRequired = computed(
    () => this.transition()?.requiresAssignee === true,
  );
  /** A comment is always welcome in the dialog, required or not. */
  protected readonly heading = computed(() => this.transition()?.name ?? 'Cambiar estado');
  protected readonly subtitle = computed(() => {
    const target = this.transition()?.to.name;
    const context = this.context();
    const move = target ? `Pasa a «${target}»` : '';
    return [context, move].filter((part) => part).join(' · ') || undefined;
  });

  /** Message of the server for a key (`comment`, `fields.due_date`, ...), once the user has not edited it. */
  protected readonly remote = computed(() => {
    const map = new Map<string, string>();
    for (const detail of this.serverErrors()) map.set(detail.field, detail.message);
    return map;
  });

  constructor() {
    effect(() => {
      const isOpen = this.open();
      const transition = this.transition();
      untracked(() => {
        if (isOpen && transition) this.prepare(transition);
      });
    });
  }

  /** Text of a field; a number input hands back a number, so the value is converted. */
  private textOf(name: string): string {
    return String(this.fieldControl(name).value ?? '').trim();
  }

  protected fieldControl(name: string): FormControl<string> {
    return this.form.controls.fields.controls[controlKey(name)];
  }

  protected fieldError(name: string): string | undefined {
    if (this.submitted() && !this.textOf(name)) {
      return name === 'flagged' ? undefined : 'Este campo es obligatorio.';
    }
    return this.remote().get(`fields.${name}`);
  }

  protected commentError(): string | undefined {
    if (this.submitted() && this.commentRequired() && !this.form.controls.comment.value.trim()) {
      return 'Escribe un comentario.';
    }
    return this.remote().get('comment');
  }

  protected resolutionError(): string | undefined {
    if (this.submitted() && this.resolutionRequired() && !this.resolutionId()) {
      return 'Elige una resolución.';
    }
    return this.remote().get('resolution_id');
  }

  protected assigneeError(): string | undefined {
    if (this.submitted() && this.assigneeRequired() && !this.assigneeId()) {
      return 'Elige una persona.';
    }
    return this.remote().get('assignee_id');
  }

  /** Details the dialog does not place next to a field (for example the subtasks rule). */
  protected readonly otherErrors = computed(() => {
    const known = new Set([
      'comment',
      'resolution_id',
      'assignee_id',
      ...this.fieldSpecs().map((spec) => `fields.${spec.name}`),
    ]);
    return this.serverErrors().filter((detail) => !known.has(detail.field));
  });

  protected toggleFlag(name: string, checked: boolean): void {
    this.fieldControl(name).setValue(checked ? 'true' : '');
  }

  protected submit(): void {
    if (this.busy()) return;
    this.submitted.set(true);
    const missingField = this.fieldSpecs().some(
      (spec) => spec.kind !== 'flag' && !this.textOf(spec.name),
    );
    const missingComment = this.commentRequired() && !this.form.controls.comment.value.trim();
    const missingResolution = this.resolutionRequired() && !this.resolutionId();
    const missingAssignee = this.assigneeRequired() && !this.assigneeId();
    if (missingField || missingComment || missingResolution || missingAssignee) return;

    const fields: Record<string, string> = {};
    for (const spec of this.fieldSpecs()) {
      const value = this.textOf(spec.name);
      if (value) fields[spec.name] = value;
    }
    const comment = this.form.controls.comment.value.trim();
    this.confirmed.emit({
      fields,
      ...(comment ? { comment } : {}),
      ...(this.resolutionId() ? { resolutionId: this.resolutionId() as string } : {}),
      ...(this.assigneeId() ? { assigneeId: this.assigneeId() as string } : {}),
    });
  }

  private prepare(transition: IssueTransition): void {
    this.submitted.set(false);
    this.form.controls.comment.setValue('');
    const group = this.form.controls.fields;
    for (const key of Object.keys(group.controls)) group.removeControl(key);
    for (const name of transition.requiredFields) {
      group.addControl(controlKey(name), new FormControl('', { nonNullable: true }));
    }
    this.resolutionId.set(null);
    this.assigneeId.set(this.currentAssigneeId() ?? undefined);
    if (transition.requiresResolution) this.catalog.ensureLoaded();
  }
}
