import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  inject,
  model,
  signal,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import {
  FormControl,
  FormGroup,
  ReactiveFormsModule,
  Validators,
  type AbstractControl,
} from '@angular/forms';
import {
  NotificationService,
  ProjectStore,
  ProjectType,
  ProjectVisibility,
  SERVER_ERROR_KEY,
  applyServerErrors,
  asApiError,
} from '@corelog/core';
import { FieldControl, FieldRow } from '@corelog/ui/field-row';
import { PrimeTemplate } from 'primeng/api';
import { Button } from 'primeng/button';
import { Dialog } from 'primeng/dialog';
import { InputText } from 'primeng/inputtext';
import { Select } from 'primeng/select';
import { Textarea } from 'primeng/textarea';

const TYPE_OPTIONS: { value: ProjectType; label: string }[] = [
  { value: 'kanban', label: 'Kanban' },
  { value: 'scrum', label: 'Scrum' },
];

const VISIBILITY_OPTIONS: { value: ProjectVisibility; label: string }[] = [
  { value: 'open', label: 'Abierto' },
  { value: 'private', label: 'Privado' },
];

const KEY_MESSAGES: Record<string, string> = {
  required: 'Escribe una clave',
  maxlength: 'La clave no puede tener más de 20 caracteres',
};

const NAME_MESSAGES: Record<string, string> = {
  required: 'Escribe un nombre',
  minlength: 'El nombre debe tener al menos 2 caracteres',
  maxlength: 'El nombre no puede tener más de 120 caracteres',
};

/** Message for the first problem of a control: the server detail wins, then the client rules. */
function controlMessage(
  control: AbstractControl,
  messages: Record<string, string>,
): string | undefined {
  if (control.hasError(SERVER_ERROR_KEY)) return String(control.getError(SERVER_ERROR_KEY));
  if (!control.touched && !control.dirty) return undefined;
  for (const key of Object.keys(messages)) {
    const message = messages[key];
    if (message && control.hasError(key)) return message;
  }
  return undefined;
}

/** Server detail of a control, shown under its field. */
function serverMessage(control: AbstractControl): string | undefined {
  return control.hasError(SERVER_ERROR_KEY)
    ? String(control.getError(SERVER_ERROR_KEY))
    : undefined;
}

/** Dialog that creates a project. Field details from the server are shown under their fields. */
@Component({
  selector: 'app-project-create-dialog',
  imports: [
    ReactiveFormsModule,
    Button,
    Dialog,
    InputText,
    PrimeTemplate,
    Select,
    Textarea,
    FieldRow,
    FieldControl,
  ],
  templateUrl: './project-create-dialog.html',
  styleUrl: './project-create-dialog.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProjectCreateDialog {
  private readonly store = inject(ProjectStore);
  private readonly notify = inject(NotificationService);
  private readonly destroyRef = inject(DestroyRef);

  /** Two-way binding: true while the dialog is open. */
  readonly open = model(false);

  protected readonly form = new FormGroup({
    key: new FormControl('', {
      nonNullable: true,
      validators: [Validators.required, Validators.maxLength(20)],
    }),
    name: new FormControl('', {
      nonNullable: true,
      validators: [Validators.required, Validators.minLength(2), Validators.maxLength(120)],
    }),
    description: new FormControl('', { nonNullable: true }),
    type: new FormControl<ProjectType>('kanban', {
      nonNullable: true,
      validators: [Validators.required],
    }),
    visibility: new FormControl<ProjectVisibility>('open', {
      nonNullable: true,
      validators: [Validators.required],
    }),
  });

  protected readonly submitting = signal(false);
  protected readonly typeOptions = TYPE_OPTIONS;
  protected readonly visibilityOptions = VISIBILITY_OPTIONS;

  protected keyError(): string | undefined {
    return controlMessage(this.form.controls.key, KEY_MESSAGES);
  }

  protected nameError(): string | undefined {
    return controlMessage(this.form.controls.name, NAME_MESSAGES);
  }

  protected serverError(control: AbstractControl): string | undefined {
    return serverMessage(control);
  }

  protected submit(): void {
    this.form.markAllAsTouched();
    // Drops server errors from the previous attempt so they do not block the next submit.
    this.form.updateValueAndValidity();
    if (this.form.invalid || this.submitting()) return;

    this.submitting.set(true);
    this.store
      .createProject(this.form.getRawValue())
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: () => {
          this.submitting.set(false);
          this.notify.success('Proyecto creado');
          this.form.reset({
            key: '',
            name: '',
            description: '',
            type: 'kanban',
            visibility: 'open',
          });
          this.open.set(false);
        },
        error: (error: unknown) => {
          this.submitting.set(false);
          const apiError = asApiError(error);
          if (!applyServerErrors(this.form, apiError)) {
            this.notify.error(apiError.message || 'No se pudo crear el proyecto');
          }
        },
      });
  }

  protected cancel(): void {
    this.open.set(false);
  }
}
