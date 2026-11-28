import { CdkDrag, CdkDragDrop, CdkDropList, moveItemInArray } from '@angular/cdk/drag-drop';
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
  CatalogStore,
  CustomField,
  CustomFieldApiService,
  FieldLayoutEntryInput,
  NotificationService,
  OrganizationStore,
  ProjectIssueTypes,
  asApiError,
} from '@corelog/core';
import { ErrorState } from '@corelog/ui/error-state';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { Section } from '@corelog/ui/section';
import { SelectField } from '@corelog/ui/select-field';
import { Button } from 'primeng/button';
import { Message } from 'primeng/message';
import { firstValueFrom } from 'rxjs';

import { ProjectSettingsContext } from '../project-settings-context';

/** System fields a layout may list, with their Spanish names. The summary is always required. */
export const SYSTEM_FIELDS: readonly { key: string; label: string }[] = [
  { key: 'summary', label: 'Resumen' },
  { key: 'description', label: 'Descripción' },
  { key: 'priority', label: 'Prioridad' },
  { key: 'assignee', label: 'Responsable' },
  { key: 'parent', label: 'Incidencia padre' },
  { key: 'due_date', label: 'Fecha límite' },
  { key: 'story_points', label: 'Puntos de historia' },
  { key: 'labels', label: 'Etiquetas' },
];

interface LayoutRow {
  readonly kind: 'system' | 'custom';
  readonly key: string;
  readonly name: string;
  readonly required: boolean;
}

/**
 * Field layout of the project: for each issue type, the ordered list of fields the create form asks
 * for and which of them are required. Rows are reordered by dragging or with the move buttons.
 */
@Component({
  selector: 'app-project-fields-page',
  imports: [
    Button,
    CdkDrag,
    CdkDropList,
    ErrorState,
    FormsModule,
    LoadingSkeleton,
    Message,
    Section,
    SelectField,
  ],
  templateUrl: './project-fields-page.html',
  styleUrl: './project-fields-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProjectFieldsPage {
  private readonly context = inject(ProjectSettingsContext);
  private readonly api = inject(CustomFieldApiService);
  private readonly catalog = inject(CatalogStore);
  private readonly orgs = inject(OrganizationStore);
  private readonly notify = inject(NotificationService);

  protected readonly canEdit = this.context.canEditNow;
  protected readonly typeId = signal('');
  protected readonly rows = signal<readonly LayoutRow[]>([]);
  protected readonly customFields = signal<readonly CustomField[]>([]);
  protected readonly projectTypes = signal<ProjectIssueTypes | null>(null);
  protected readonly loading = signal(true);
  protected readonly failed = signal(false);
  protected readonly saving = signal(false);
  protected readonly dirty = signal(false);
  protected readonly error = signal<string | null>(null);
  protected readonly addPick = signal('');

  protected readonly typeOptions = computed(() => {
    const allowed = this.projectTypes()?.items ?? this.catalog.issueTypes();
    return allowed.map((item) => ({ value: item.id, label: item.name }));
  });

  /** Fields not in the layout yet. */
  protected readonly addable = computed(() => {
    const used = new Set(this.rows().map((row) => `${row.kind}:${row.key}`));
    return [
      ...SYSTEM_FIELDS.filter((field) => !used.has(`system:${field.key}`)).map((field) => ({
        value: `system:${field.key}`,
        label: field.label,
      })),
      ...this.customFields()
        .filter((field) => !used.has(`custom:${field.id}`))
        .map((field) => ({
          value: `custom:${field.id}`,
          label: `${field.name} (campo personalizado)`,
        })),
    ];
  });

  constructor() {
    this.catalog.ensureLoaded();
    effect(() => {
      const projectId = this.context.projectId();
      if (projectId) untracked(() => void this.init(projectId));
    });
    effect(() => {
      const projectId = this.context.projectId();
      const typeId = this.typeId();
      if (projectId && typeId) untracked(() => void this.loadLayout(projectId, typeId));
    });
  }

  protected retry(): void {
    const projectId = this.context.projectId();
    if (projectId) void this.init(projectId);
  }

  protected drop(event: CdkDragDrop<readonly LayoutRow[]>): void {
    const next = [...this.rows()];
    moveItemInArray(next, event.previousIndex, event.currentIndex);
    this.rows.set(next);
    this.dirty.set(true);
  }

  protected move(index: number, delta: number): void {
    const target = index + delta;
    if (target < 0 || target >= this.rows().length) return;
    const next = [...this.rows()];
    moveItemInArray(next, index, target);
    this.rows.set(next);
    this.dirty.set(true);
  }

  protected setRequired(index: number, required: boolean): void {
    this.rows.update((list) => list.map((row, i) => (i === index ? { ...row, required } : row)));
    this.dirty.set(true);
  }

  protected remove(index: number): void {
    this.rows.update((list) => list.filter((_, i) => i !== index));
    this.dirty.set(true);
  }

  protected add(value: string | null): void {
    if (!value) return;
    const [kind, key] = value.split(':');
    const name =
      kind === 'system'
        ? (SYSTEM_FIELDS.find((field) => field.key === key)?.label ?? key)
        : (this.customFields().find((field) => field.id === key)?.name ?? key);
    this.rows.update((list) => [
      ...list,
      { kind: kind as 'system' | 'custom', key, name, required: false },
    ]);
    this.dirty.set(true);
    this.addPick.set('');
  }

  protected isLocked(row: LayoutRow): boolean {
    return row.kind === 'system' && row.key === 'summary';
  }

  protected async save(): Promise<void> {
    const projectId = this.context.projectId();
    const typeId = this.typeId();
    if (!projectId || !typeId) return;
    this.saving.set(true);
    this.error.set(null);
    try {
      const items: FieldLayoutEntryInput[] = this.rows().map((row) =>
        row.kind === 'system'
          ? { kind: 'system', key: row.key, required: row.required || row.key === 'summary' }
          : { kind: 'custom', fieldId: row.key, required: row.required },
      );
      await firstValueFrom(this.api.saveLayout(projectId, typeId, items));
      this.dirty.set(false);
      this.notify.success('Campos guardados');
    } catch (error) {
      this.error.set(asApiError(error).message);
    } finally {
      this.saving.set(false);
    }
  }

  private async init(projectId: string): Promise<void> {
    this.loading.set(true);
    this.failed.set(false);
    try {
      const organizationId = this.orgs.currentOrgId();
      const [types, fields] = await Promise.all([
        firstValueFrom(this.catalog.projectIssueTypes(projectId)),
        organizationId
          ? firstValueFrom(this.api.list(organizationId)).catch(() => [] as CustomField[])
          : Promise.resolve([] as CustomField[]),
      ]);
      this.projectTypes.set(types);
      this.customFields.set(fields);
      const first = types.items.find((item) => item.hierarchyLevel === 0) ?? types.items[0];
      this.typeId.set(this.typeId() || first?.id || '');
      if (!first) this.loading.set(false);
    } catch {
      this.failed.set(true);
      this.loading.set(false);
    }
  }

  private async loadLayout(projectId: string, typeId: string): Promise<void> {
    this.loading.set(true);
    this.error.set(null);
    try {
      const layout = await firstValueFrom(this.api.layout(projectId, typeId));
      if (typeId !== this.typeId()) return;
      this.rows.set(
        layout.items.map((item) => ({
          kind: item.kind,
          key: (item.kind === 'system' ? item.key : item.fieldId) ?? '',
          name:
            item.kind === 'system'
              ? (SYSTEM_FIELDS.find((field) => field.key === item.key)?.label ?? item.name)
              : item.name,
          required: item.required,
        })),
      );
      this.dirty.set(false);
    } catch {
      this.failed.set(true);
    } finally {
      this.loading.set(false);
    }
  }
}
