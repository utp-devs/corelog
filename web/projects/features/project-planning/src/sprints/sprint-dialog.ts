import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  input,
  output,
  untracked,
} from '@angular/core';
import { FormControl, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { AgileSprint } from '@corelog/core';
import { DateField } from '@corelog/ui/date-field';
import { Dialog } from '@corelog/ui/dialog';
import { FieldControl, FieldRow } from '@corelog/ui/field-row';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';
import { Message } from 'primeng/message';
import { Textarea } from 'primeng/textarea';

import { addDays, defaultSprintDates } from '../shared/planning-format';

/** What the person chose in the sprint dialog. Empty dates are null. */
export interface SprintDialogValue {
  readonly name: string;
  readonly goal: string;
  readonly startDate: string | null;
  readonly endDate: string | null;
  /** Start even when the sprint has no issues. */
  readonly force: boolean;
}

export type SprintDialogMode = 'create' | 'edit' | 'start';

const MAX_NAME = 80;
const MAX_GOAL = 500;

const HEADINGS: Readonly<Record<SprintDialogMode, string>> = {
  create: 'Crear sprint',
  edit: 'Editar sprint',
  start: 'Iniciar sprint',
};

/**
 * Dialog to create, edit or start a sprint: name, goal and dates. When it starts a sprint that has no
 * issues, the person must confirm that on purpose. The owner runs the action with the value it emits.
 */
@Component({
  selector: 'app-sprint-dialog',
  imports: [
    Button,
    DateField,
    Dialog,
    FieldControl,
    FieldRow,
    InputText,
    Message,
    ReactiveFormsModule,
    Textarea,
  ],
  templateUrl: './sprint-dialog.html',
  styleUrl: './sprint-dialog.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class SprintDialog {
  readonly open = input(false);
  readonly mode = input<SprintDialogMode>('create');
  /** Sprint being edited or started; its values prefill the form. */
  readonly sprint = input<AgileSprint | null>(null);
  /** Issues the sprint holds; a start with none asks for confirmation. */
  readonly issueCount = input(0);
  /** Name proposed for a new sprint. */
  readonly suggestedName = input('');
  readonly busy = input(false);
  readonly error = input<string | null>(null);

  readonly submitted = output<SprintDialogValue>();
  readonly closeRequested = output<void>();

  protected readonly form = new FormGroup({
    name: new FormControl('', {
      nonNullable: true,
      validators: [Validators.required, Validators.pattern(/\S/), Validators.maxLength(MAX_NAME)],
    }),
    goal: new FormControl('', { nonNullable: true, validators: [Validators.maxLength(MAX_GOAL)] }),
    startDate: new FormControl<string | null>(null),
    endDate: new FormControl<string | null>(null),
    force: new FormControl(false, { nonNullable: true }),
  });

  protected readonly heading = computed(() => HEADINGS[this.mode()]);
  protected readonly confirmLabel = computed(() => HEADINGS[this.mode()]);
  protected readonly needsForce = computed(
    () => this.mode() === 'start' && this.issueCount() === 0,
  );
  protected readonly subtitle = computed(() =>
    this.mode() === 'start'
      ? 'Confirma el nombre, las fechas y el objetivo antes de empezar.'
      : undefined,
  );
  protected submittedOnce = false;

  protected readonly nameMax = MAX_NAME;
  protected readonly goalMax = MAX_GOAL;

  constructor() {
    effect(() => {
      if (!this.open()) return;
      untracked(() => this.prefill());
    });
  }

  protected submit(): void {
    this.submittedOnce = true;
    if (
      this.form.invalid ||
      this.datesInvalid() ||
      (this.needsForce() && !this.form.controls.force.value)
    ) {
      this.form.markAllAsTouched();
      return;
    }
    const value = this.form.getRawValue();
    this.submitted.emit({
      name: value.name.trim(),
      goal: value.goal.trim(),
      startDate: value.startDate || null,
      endDate: value.endDate || null,
      force: value.force,
    });
  }

  protected nameError(): string | undefined {
    const control = this.form.controls.name;
    if (!(control.touched || this.submittedOnce) || control.valid) return undefined;
    return control.hasError('maxlength') ? `Máximo ${MAX_NAME} caracteres.` : 'Escribe un nombre.';
  }

  protected datesInvalid(): boolean {
    const { startDate, endDate } = this.form.getRawValue();
    return !!startDate && !!endDate && endDate < startDate;
  }

  protected dateError(): string | undefined {
    return this.submittedOnce && this.datesInvalid()
      ? 'El fin no puede ser anterior al inicio.'
      : undefined;
  }

  protected forceError(): boolean {
    return this.submittedOnce && this.needsForce() && !this.form.controls.force.value;
  }

  private prefill(): void {
    const sprint = this.sprint();
    const mode = this.mode();
    this.submittedOnce = false;
    const defaults = defaultSprintDates();
    const startDate = sprint?.startDate ?? (mode === 'edit' ? null : defaults.start);
    const endDate =
      sprint?.endDate ??
      (mode === 'edit' ? null : startDate ? addDays(startDate, 14) : defaults.end);
    this.form.reset({
      name: sprint?.name ?? this.suggestedName(),
      goal: sprint?.goal ?? '',
      startDate,
      endDate,
      force: false,
    });
  }
}
