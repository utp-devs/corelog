import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  Signal,
  computed,
  effect,
  inject,
  signal,
  untracked,
} from '@angular/core';
import { takeUntilDestroyed, toSignal } from '@angular/core/rxjs-interop';
import { FormControl, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import {
  CurrentProjectStore,
  DefaultAssigneePolicy,
  NotificationService,
  Project,
  UnsavedChangesAware,
  asApiError,
  warnBeforeUnload,
} from '@corelog/core';
import { ColorPicker } from '@corelog/ui/color-picker';
import { FieldControl, FieldRow } from '@corelog/ui/field-row';
import { Section } from '@corelog/ui/section';
import { SelectField, SelectFieldOption } from '@corelog/ui/select-field';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';
import { Message } from 'primeng/message';
import { Textarea } from 'primeng/textarea';
import { map } from 'rxjs';

import { ProjectSettingsContext } from '../project-settings-context';
import { settingsErrorMessage } from '../settings-copy';
import { ProjectIconPicker } from './project-icon-picker';

export const NAME_MAX = 120;
export const DESCRIPTION_MAX = 2000;

export const ASSIGNEE_OPTIONS: readonly SelectFieldOption[] = [
  { value: 'unassigned', label: 'Sin asignar' },
  { value: 'lead', label: 'Asignar al líder del proyecto' },
];

interface DetailsValues {
  name: string;
  description: string;
  color: string | null;
  icon: string;
  defaultAssigneePolicy: DefaultAssigneePolicy;
  leadId: string;
}

function valuesOf(project: Project): DetailsValues {
  return {
    name: project.name,
    description: project.description,
    color: project.color || null,
    icon: project.icon ?? '',
    defaultAssigneePolicy: project.defaultAssigneePolicy ?? 'unassigned',
    leadId: project.leadId,
  };
}

/** Name, description, appearance, default assignee and lead of the project. */
@Component({
  selector: 'app-project-details-page',
  imports: [
    ReactiveFormsModule,
    Button,
    InputText,
    Textarea,
    Message,
    Section,
    FieldRow,
    FieldControl,
    SelectField,
    ColorPicker,
    ProjectIconPicker,
  ],
  templateUrl: './project-details-page.html',
  styleUrl: './project-details-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProjectDetailsPage implements UnsavedChangesAware {
  private readonly context = inject(ProjectSettingsContext);
  private readonly current = inject(CurrentProjectStore);
  private readonly notify = inject(NotificationService);
  private readonly destroyRef = inject(DestroyRef);

  protected readonly project = this.context.project;
  protected readonly canEdit = this.context.canEditNow;
  protected readonly nameMax = NAME_MAX;
  protected readonly descriptionMax = DESCRIPTION_MAX;
  protected readonly assigneeOptions = ASSIGNEE_OPTIONS;

  protected readonly form = new FormGroup({
    name: new FormControl('', {
      nonNullable: true,
      validators: [Validators.required, Validators.maxLength(NAME_MAX)],
    }),
    description: new FormControl('', {
      nonNullable: true,
      validators: [Validators.maxLength(DESCRIPTION_MAX)],
    }),
    color: new FormControl<string | null>(null),
    icon: new FormControl('', { nonNullable: true }),
    defaultAssigneePolicy: new FormControl<DefaultAssigneePolicy>('unassigned', {
      nonNullable: true,
    }),
    leadId: new FormControl('', { nonNullable: true, validators: [Validators.required] }),
  });

  protected readonly saving = signal(false);
  protected readonly submitted = signal(false);
  protected readonly failure = signal<string | null>(null);

  private readonly formDirty = toSignal(this.form.events.pipe(map(() => this.form.dirty)), {
    initialValue: false,
  });
  /** True while there are edits not saved yet (read by the unsaved-changes guard). */
  readonly dirty: Signal<boolean> = this.formDirty;

  /** People who can lead: the members of the project, and the current lead even if the list lacks them. */
  protected readonly leadOptions = computed<SelectFieldOption[]>(() => {
    const leadId = this.project()?.leadId;
    const ids = new Set(this.context.members().map((member) => member.userId));
    if (leadId) ids.add(leadId);
    return [...ids]
      .map((id) => ({ value: id, label: this.context.personOf(id).name }))
      .sort((a, b) => a.label.localeCompare(b.label, 'es'));
  });

  private readonly versionKey = computed(() => {
    const project = this.project();
    return project ? `${project.id}|${project.updatedAt}` : null;
  });

  constructor() {
    warnBeforeUnload(this.dirty);

    // A new project or a fresher `updatedAt` resets the form to the stored values.
    effect(() => {
      this.versionKey();
      untracked(() => {
        const project = this.project();
        if (project) {
          this.form.reset(valuesOf(project));
          this.failure.set(null);
          this.submitted.set(false);
        }
      });
    });

    effect(() => {
      const editable = this.canEdit();
      untracked(() => {
        if (editable) this.form.enable({ emitEvent: false });
        else this.form.disable({ emitEvent: false });
      });
    });
  }

  protected save(): void {
    const project = this.project();
    if (!project || !this.canEdit()) return;
    this.submitted.set(true);
    this.form.markAllAsTouched();
    if (this.form.invalid) return;

    const value = this.form.getRawValue();
    this.saving.set(true);
    this.failure.set(null);
    this.context.projects
      .updateProject(project.id, {
        name: value.name.trim(),
        description: value.description.trim(),
        type: project.type,
        leadId: value.leadId,
        visibility: project.visibility,
        color: value.color ?? undefined,
        icon: value.icon,
        defaultAssigneePolicy: value.defaultAssigneePolicy,
      })
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (updated) => {
          this.saving.set(false);
          this.current.setProject(updated);
          // A lead transfer changes who can do what, for the caller included.
          this.context.reloadProjectData(updated.id);
          this.current.reloadPermissions();
          this.notify.success('Detalles del proyecto guardados');
        },
        error: (error: unknown) => {
          this.saving.set(false);
          this.showFailure(error);
        },
      });
  }

  protected discard(): void {
    const project = this.project();
    if (project) {
      this.form.reset(valuesOf(project));
      this.failure.set(null);
      this.submitted.set(false);
    }
  }

  private showFailure(error: unknown): void {
    const apiError = asApiError(error);
    let mapped = false;
    for (const detail of apiError.details) {
      const control = this.form.get(detail.field);
      if (control) {
        control.setErrors({ server: detail.message });
        mapped = true;
      }
    }
    if (!mapped)
      this.failure.set(settingsErrorMessage(error, 'No se pudieron guardar los cambios.'));
  }

  protected errorOf(name: 'name' | 'description' | 'leadId'): string | undefined {
    const control = this.form.controls[name];
    if (!(control.touched || this.submitted()) || !control.errors) return undefined;
    if (control.errors['server']) return String(control.errors['server']);
    if (control.errors['required']) {
      return name === 'name' ? 'Escribe el nombre del proyecto.' : 'Elige a la persona que lidera.';
    }
    if (control.errors['maxlength']) return 'El texto es demasiado largo.';
    return undefined;
  }
}
