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
import type { DashboardLayout } from '@corelog/core';
import { Dialog } from '@corelog/ui/dialog';
import { FieldControl, FieldRow } from '@corelog/ui/field-row';
import { SelectField, SelectFieldOption } from '@corelog/ui/select-field';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';
import { Message } from 'primeng/message';
import { Textarea } from 'primeng/textarea';

export interface DashboardFormValue {
  readonly name: string;
  readonly description: string;
  readonly layout: DashboardLayout;
}

export const LAYOUT_OPTIONS: readonly SelectFieldOption[] = [
  { value: '1', label: 'Una columna' },
  { value: '2', label: 'Dos columnas iguales' },
  { value: '3', label: 'Tres columnas' },
  { value: '2-1', label: 'Dos columnas, la primera más ancha' },
  { value: '1-2', label: 'Dos columnas, la segunda más ancha' },
];

const MAX_NAME = 120;
const MAX_DESCRIPTION = 500;

/** Dialog to create a dashboard or to rename it and change its description. */
@Component({
  selector: 'app-dashboard-form-dialog',
  imports: [
    Button,
    Dialog,
    FieldControl,
    FieldRow,
    InputText,
    Message,
    ReactiveFormsModule,
    SelectField,
    Textarea,
  ],
  templateUrl: './dashboard-form-dialog.html',
  styleUrl: './dashboard-form-dialog.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class DashboardFormDialog {
  readonly open = input(false);
  readonly mode = input<'create' | 'edit'>('create');
  readonly name = input('');
  readonly description = input('');
  readonly busy = input(false);
  readonly error = input<string | null>(null);

  readonly submitted = output<DashboardFormValue>();
  readonly closeRequested = output<void>();

  protected readonly layoutOptions = LAYOUT_OPTIONS;
  protected readonly nameMax = MAX_NAME;
  protected readonly descriptionMax = MAX_DESCRIPTION;
  protected readonly heading = computed(() =>
    this.mode() === 'create' ? 'Crear un panel' : 'Editar el panel',
  );

  protected readonly form = new FormGroup({
    name: new FormControl('', {
      nonNullable: true,
      validators: [Validators.required, Validators.maxLength(MAX_NAME)],
    }),
    description: new FormControl('', {
      nonNullable: true,
      validators: [Validators.maxLength(MAX_DESCRIPTION)],
    }),
    layout: new FormControl<string>('2', { nonNullable: true }),
  });

  protected attempted = false;

  constructor() {
    effect(() => {
      if (!this.open()) return;
      const name = this.name();
      const description = this.description();
      untracked(() => {
        this.attempted = false;
        this.form.reset({ name, description, layout: '2' });
      });
    });
  }

  protected nameError(): string | undefined {
    if (!this.attempted) return undefined;
    const control = this.form.controls.name;
    if (control.hasError('required')) return 'Escribe un nombre para el panel.';
    return control.hasError('maxlength') ? `Máximo ${MAX_NAME} caracteres.` : undefined;
  }

  protected submit(): void {
    this.attempted = true;
    if (this.form.invalid || this.busy()) return;
    const value = this.form.getRawValue();
    this.submitted.emit({
      name: value.name.trim(),
      description: value.description.trim(),
      layout: value.layout as DashboardLayout,
    });
  }
}
