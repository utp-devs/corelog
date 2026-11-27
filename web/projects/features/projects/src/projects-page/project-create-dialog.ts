import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  effect,
  inject,
  model,
  signal,
  untracked,
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
  ORGANIZATION_ID_PROVIDER,
  OrganizationApiService,
  Project,
  ProjectStore,
  ProjectType,
  ProjectVisibility,
  SERVER_ERROR_KEY,
  applyServerErrors,
  asApiError,
} from '@corelog/core';
import { Dialog } from '@corelog/ui/dialog';
import { FieldControl, FieldRow } from '@corelog/ui/field-row';
import { PickableUser, UserPicker } from '@corelog/ui/user-picker';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';
import { Textarea } from 'primeng/textarea';
import { Observable, catchError, of, switchMap } from 'rxjs';

import { PROJECT_KEY_MAX_LENGTH, suggestProjectKey } from './project-create-dialog-key';

interface ChoiceCard<T extends string> {
  readonly value: T;
  readonly label: string;
  readonly description: string;
}

const TYPE_OPTIONS: readonly ChoiceCard<ProjectType>[] = [
  {
    value: 'kanban',
    label: 'Kanban',
    description: 'Flujo continuo: las incidencias avanzan por columnas sin iteraciones.',
  },
  {
    value: 'scrum',
    label: 'Scrum',
    description: 'Trabajo en sprints con backlog, planificación y seguimiento.',
  },
];

const VISIBILITY_OPTIONS: readonly ChoiceCard<ProjectVisibility>[] = [
  {
    value: 'open',
    label: 'Abierto',
    description: 'Cualquier persona de la organización puede verlo.',
  },
  {
    value: 'private',
    label: 'Privado',
    description: 'Solo lo ven las personas que agregues como miembros.',
  },
];

const KEY_MESSAGES: Record<string, string> = {
  required: 'Escribe una clave',
  maxlength: `La clave no puede tener más de ${PROJECT_KEY_MAX_LENGTH} caracteres`,
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

/**
 * Dialog that creates a project: name, key (suggested from the name until the user types one),
 * description, type and visibility as cards, and an optional lead (the creator by default). Field details
 * from the server are shown under their fields. The page decides who may open it; the server stays the
 * authority and its refusal reaches the user as a translated toast.
 */
@Component({
  selector: 'app-project-create-dialog',
  imports: [
    ReactiveFormsModule,
    Button,
    Dialog,
    InputText,
    Textarea,
    FieldRow,
    FieldControl,
    UserPicker,
  ],
  templateUrl: './project-create-dialog.html',
  styleUrl: './project-create-dialog.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProjectCreateDialog {
  private readonly store = inject(ProjectStore);
  private readonly notify = inject(NotificationService);
  private readonly destroyRef = inject(DestroyRef);
  private readonly organizationId = inject(ORGANIZATION_ID_PROVIDER);
  private readonly organizations = inject(OrganizationApiService);

  /** Two-way binding: true while the dialog is open. */
  readonly open = model(false);

  protected readonly form = new FormGroup({
    key: new FormControl('', {
      nonNullable: true,
      validators: [Validators.required, Validators.maxLength(PROJECT_KEY_MAX_LENGTH)],
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
    /** Person who leads the project. Empty means the creator, who is the lead by default. */
    leadId: new FormControl<string | undefined>(undefined, { nonNullable: true }),
  });

  protected readonly submitting = signal(false);
  protected readonly typeOptions = TYPE_OPTIONS;
  protected readonly visibilityOptions = VISIBILITY_OPTIONS;
  protected readonly keyMaxLength = PROJECT_KEY_MAX_LENGTH;
  protected readonly leadCandidates = signal<PickableUser[]>([]);

  /** True once the user typed a key: from then on the name no longer rewrites it. */
  private keyEdited = false;

  constructor() {
    this.form.controls.name.valueChanges
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe((name) => {
        if (!this.keyEdited) {
          this.form.controls.key.setValue(suggestProjectKey(name), { emitEvent: false });
        }
      });
    effect(() => {
      if (this.open()) untracked(() => this.onOpened());
    });
  }

  protected keyError(): string | undefined {
    return controlMessage(this.form.controls.key, KEY_MESSAGES);
  }

  protected nameError(): string | undefined {
    return controlMessage(this.form.controls.name, NAME_MESSAGES);
  }

  protected serverError(control: AbstractControl): string | undefined {
    return serverMessage(control);
  }

  protected onKeyInput(value: string): void {
    this.keyEdited = value.trim() !== '';
    const key = this.form.controls.key;
    if (value !== value.toUpperCase()) key.setValue(value.toUpperCase());
    if (!this.keyEdited) key.setValue(suggestProjectKey(this.form.controls.name.value));
  }

  protected radioId(group: string, value: string): string {
    return `project-create-${group}-${value}`;
  }

  protected submit(): void {
    this.form.markAllAsTouched();
    // Drops server errors from the previous attempt so they do not block the next submit.
    this.form.updateValueAndValidity();
    if (this.form.invalid || this.submitting()) return;

    this.submitting.set(true);
    const { leadId, ...input } = this.form.getRawValue();
    this.store
      .createProject(input)
      .pipe(
        switchMap((project) =>
          leadId
            ? this.store
                .updateProject(project.id, {
                  name: project.name,
                  description: project.description,
                  type: project.type,
                  visibility: project.visibility,
                  leadId,
                })
                .pipe(catchError(() => this.leadNotAssigned(project)))
            : of(project),
        ),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe({
        next: () => {
          this.submitting.set(false);
          this.notify.success('Proyecto creado');
          this.resetForm();
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

  /** The project exists but the chosen lead could not be set: say so and keep the creator as lead. */
  private leadNotAssigned(project: Project): Observable<Project> {
    this.notify.info('El proyecto se creó, pero no se pudo asignar a la persona responsable.');
    return of(project);
  }

  private onOpened(): void {
    this.resetForm();
    const organizationId = this.organizationId();
    if (!organizationId) return;
    this.organizations
      .listMembers(organizationId)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (members) =>
          this.leadCandidates.set(
            members.map((member) => ({
              id: member.userId,
              name: member.displayName || member.name,
              email: member.email,
            })),
          ),
        error: () => this.leadCandidates.set([]),
      });
  }

  private resetForm(): void {
    this.keyEdited = false;
    this.form.reset({
      key: '',
      name: '',
      description: '',
      type: 'kanban',
      visibility: 'open',
      leadId: undefined,
    });
  }
}
