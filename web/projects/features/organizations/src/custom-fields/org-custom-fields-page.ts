import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  signal,
  untracked,
} from '@angular/core';
import { FormsModule } from '@angular/forms';
import {
  CUSTOM_FIELD_TYPES,
  CatalogStore,
  CustomField,
  CustomFieldApiService,
  CustomFieldContext,
  CustomFieldOptionDraft,
  CustomFieldType,
  NotificationService,
  OrganizationStore,
  ProjectStore,
  asApiError,
  canManageOrg,
} from '@corelog/core';
import { ConfirmModal } from '@corelog/ui/confirm-modal';
import { Dialog } from '@corelog/ui/dialog';
import { EmptyState } from '@corelog/ui/empty-state';
import { ErrorState } from '@corelog/ui/error-state';
import { FieldRow } from '@corelog/ui/field-row';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader, PageHeaderActions } from '@corelog/ui/page-header';
import { Section } from '@corelog/ui/section';
import { SelectField } from '@corelog/ui/select-field';
import { Button } from 'primeng/button';
import { Message } from 'primeng/message';
import { firstValueFrom } from 'rxjs';

export const FIELD_TYPE_LABELS: Readonly<Record<CustomFieldType, string>> = {
  text: 'Texto corto',
  paragraph: 'Texto largo',
  number: 'Número',
  date: 'Fecha',
  datetime: 'Fecha y hora',
  select: 'Lista de opciones',
  multiselect: 'Lista de opciones múltiple',
  checkbox: 'Casilla',
  user: 'Persona',
  url: 'Enlace',
  labels: 'Etiquetas',
};

/** Sentinel of the "all projects" and "all types" entries of the context selects. */
const ALL = '';

interface OptionRow {
  id?: string;
  label: string;
  archived: boolean;
}

interface ContextRow {
  projectId: string;
  issueTypeId: string;
  required: boolean;
}

/**
 * Custom fields of the organization: the list, the create and edit dialog (name, description, type,
 * options with archive, number limits) and the contexts dialog that says in which projects and issue
 * types a field applies and whether it is required there.
 */
@Component({
  selector: 'app-org-custom-fields-page',
  imports: [
    Button,
    ConfirmModal,
    Dialog,
    EmptyState,
    ErrorState,
    FieldRow,
    FormsModule,
    LoadingSkeleton,
    Message,
    PageContainer,
    PageHeader,
    PageHeaderActions,
    Section,
    SelectField,
  ],
  templateUrl: './org-custom-fields-page.html',
  styleUrl: './org-custom-fields-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class OrgCustomFieldsPage {
  private readonly api = inject(CustomFieldApiService);
  private readonly orgs = inject(OrganizationStore);
  private readonly catalog = inject(CatalogStore);
  private readonly projects = inject(ProjectStore);
  private readonly notify = inject(NotificationService);

  protected readonly fields = signal<readonly CustomField[]>([]);
  protected readonly loading = signal(true);
  protected readonly loadError = signal(false);
  protected readonly canManage = computed(() => canManageOrg(this.orgs.currentRole()));
  protected readonly typeLabels = FIELD_TYPE_LABELS;
  protected readonly typeOptions = CUSTOM_FIELD_TYPES.map((type) => ({
    value: type,
    label: FIELD_TYPE_LABELS[type],
  }));

  // Create and edit dialog.
  protected readonly formOpen = signal(false);
  protected readonly editing = signal<CustomField | null>(null);
  protected readonly name = signal('');
  protected readonly description = signal('');
  protected readonly type = signal<CustomFieldType>('text');
  protected readonly options = signal<OptionRow[]>([]);
  protected readonly min = signal('');
  protected readonly max = signal('');
  protected readonly saving = signal(false);
  protected readonly formError = signal<string | null>(null);

  // Contexts dialog.
  protected readonly contextField = signal<CustomField | null>(null);
  protected readonly contexts = signal<ContextRow[]>([]);
  protected readonly contextError = signal<string | null>(null);

  // Delete.
  protected readonly deleting = signal<CustomField | null>(null);

  protected readonly hasOptions = computed(
    () => this.type() === 'select' || this.type() === 'multiselect',
  );
  protected readonly projectOptions = computed(() => [
    { value: ALL, label: 'Todos los proyectos' },
    ...this.projects.projects().map((project) => ({
      value: project.id,
      label: `${project.name} (${project.key})`,
    })),
  ]);
  protected readonly issueTypeOptions = computed(() => [
    { value: ALL, label: 'Todos los tipos' },
    ...this.catalog.issueTypes().map((item) => ({ value: item.id, label: item.name })),
  ]);

  constructor() {
    this.catalog.ensureLoaded();
    this.projects.fetchProjects();
    effect(() => {
      const organizationId = this.orgs.currentOrgId();
      if (organizationId) untracked(() => void this.load(organizationId));
    });
  }

  protected reload(): void {
    const organizationId = this.orgs.currentOrgId();
    if (organizationId) void this.load(organizationId);
  }

  protected summaryOf(field: CustomField): string {
    if (field.contexts.length === 0) return 'Sin contexto: no se usa en ningún proyecto';
    return field.contexts
      .map((context) => {
        const project = context.projectId
          ? (this.projects.projects().find((item) => item.id === context.projectId)?.key ??
            'Proyecto')
          : 'Todos los proyectos';
        const type = context.issueTypeId
          ? (this.catalog.issueTypes().find((item) => item.id === context.issueTypeId)?.name ??
            'Tipo')
          : 'todos los tipos';
        return `${project} / ${type}${context.required ? ' (obligatorio)' : ''}`;
      })
      .join(', ');
  }

  // Create and edit.

  protected openCreate(): void {
    this.editing.set(null);
    this.name.set('');
    this.description.set('');
    this.type.set('text');
    this.options.set([]);
    this.min.set('');
    this.max.set('');
    this.formError.set(null);
    this.formOpen.set(true);
  }

  protected openEdit(field: CustomField): void {
    this.editing.set(field);
    this.name.set(field.name);
    this.description.set(field.description);
    this.type.set(field.type);
    this.options.set(
      field.options.map((option) => ({
        id: option.id,
        label: option.label,
        archived: option.archived,
      })),
    );
    this.min.set(field.min === null ? '' : String(field.min));
    this.max.set(field.max === null ? '' : String(field.max));
    this.formError.set(null);
    this.formOpen.set(true);
  }

  protected closeForm(): void {
    if (!this.saving()) this.formOpen.set(false);
  }

  protected addOption(): void {
    this.options.update((list) => [...list, { label: '', archived: false }]);
  }

  protected setOptionLabel(index: number, label: string): void {
    this.options.update((list) => list.map((row, i) => (i === index ? { ...row, label } : row)));
  }

  protected toggleArchived(index: number): void {
    this.options.update((list) =>
      list.map((row, i) => (i === index ? { ...row, archived: !row.archived } : row)),
    );
  }

  protected removeOption(index: number): void {
    this.options.update((list) => list.filter((_, i) => i !== index));
  }

  protected async submit(): Promise<void> {
    const organizationId = this.orgs.currentOrgId();
    const name = this.name().trim();
    if (!organizationId) return;
    if (!name) {
      this.formError.set('Escribe un nombre para el campo.');
      return;
    }
    const options: CustomFieldOptionDraft[] = this.options()
      .filter((row) => row.label.trim() !== '')
      .map((row) => ({ id: row.id, label: row.label.trim(), archived: row.archived }));
    if (this.hasOptions() && options.length === 0) {
      this.formError.set('Añade al menos una opción.');
      return;
    }
    const min = this.min().trim() === '' ? undefined : Number(this.min());
    const max = this.max().trim() === '' ? undefined : Number(this.max());
    if ((min !== undefined && Number.isNaN(min)) || (max !== undefined && Number.isNaN(max))) {
      this.formError.set('Los límites deben ser números.');
      return;
    }
    this.saving.set(true);
    this.formError.set(null);
    try {
      const editing = this.editing();
      if (editing) {
        await firstValueFrom(
          this.api.patch(organizationId, editing.id, {
            name,
            description: this.description().trim(),
            options: this.hasOptions() ? options : undefined,
            min: this.type() === 'number' ? (min ?? null) : undefined,
            max: this.type() === 'number' ? (max ?? null) : undefined,
          }),
        );
      } else {
        await firstValueFrom(
          this.api.create(organizationId, {
            name,
            description: this.description().trim() || undefined,
            type: this.type(),
            options: this.hasOptions()
              ? options.map((option) => ({ label: option.label }))
              : undefined,
            min: this.type() === 'number' ? min : undefined,
            max: this.type() === 'number' ? max : undefined,
          }),
        );
      }
      this.formOpen.set(false);
      this.notify.success(editing ? 'Campo actualizado' : 'Campo creado');
      await this.load(organizationId, false);
    } catch (error) {
      this.formError.set(asApiError(error).message);
    } finally {
      this.saving.set(false);
    }
  }

  // Contexts.

  protected openContexts(field: CustomField): void {
    this.contextField.set(field);
    this.contextError.set(null);
    this.contexts.set(
      field.contexts.map((context) => ({
        projectId: context.projectId ?? ALL,
        issueTypeId: context.issueTypeId ?? ALL,
        required: context.required,
      })),
    );
  }

  protected closeContexts(): void {
    if (!this.saving()) this.contextField.set(null);
  }

  protected addContext(): void {
    this.contexts.update((list) => [
      ...list,
      { projectId: ALL, issueTypeId: ALL, required: false },
    ]);
  }

  protected patchContext(index: number, patch: Partial<ContextRow>): void {
    this.contexts.update((list) =>
      list.map((row, i) => (i === index ? { ...row, ...patch } : row)),
    );
  }

  protected removeContext(index: number): void {
    this.contexts.update((list) => list.filter((_, i) => i !== index));
  }

  protected async saveContexts(): Promise<void> {
    const organizationId = this.orgs.currentOrgId();
    const field = this.contextField();
    if (!organizationId || !field) return;
    const seen = new Set<string>();
    for (const row of this.contexts()) {
      const key = `${row.projectId}|${row.issueTypeId}`;
      if (seen.has(key)) {
        this.contextError.set('Cada combinación de proyecto y tipo solo puede aparecer una vez.');
        return;
      }
      seen.add(key);
    }
    this.saving.set(true);
    this.contextError.set(null);
    try {
      const payload: CustomFieldContext[] = this.contexts().map((row) => ({
        projectId: row.projectId === ALL ? null : row.projectId,
        issueTypeId: row.issueTypeId === ALL ? null : row.issueTypeId,
        required: row.required,
        defaultValue: null,
      }));
      await firstValueFrom(this.api.replaceContexts(organizationId, field.id, payload));
      this.contextField.set(null);
      this.notify.success('Contextos guardados');
      await this.load(organizationId, false);
    } catch (error) {
      this.contextError.set(asApiError(error).message);
    } finally {
      this.saving.set(false);
    }
  }

  // Delete.

  protected async confirmDelete(): Promise<void> {
    const organizationId = this.orgs.currentOrgId();
    const field = this.deleting();
    if (!organizationId || !field) return;
    this.saving.set(true);
    try {
      await firstValueFrom(this.api.remove(organizationId, field.id));
      this.deleting.set(null);
      this.notify.success('Campo eliminado');
      await this.load(organizationId, false);
    } catch (error) {
      this.deleting.set(null);
      this.notify.error(asApiError(error).message);
    } finally {
      this.saving.set(false);
    }
  }

  private async load(organizationId: string, skeleton = true): Promise<void> {
    if (skeleton) this.loading.set(true);
    this.loadError.set(false);
    try {
      this.fields.set(await firstValueFrom(this.api.list(organizationId)));
    } catch {
      this.loadError.set(true);
    } finally {
      this.loading.set(false);
    }
  }
}
