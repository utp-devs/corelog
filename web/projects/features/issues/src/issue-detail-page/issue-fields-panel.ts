import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  input,
  signal,
  untracked,
} from '@angular/core';
import { RouterLink } from '@angular/router';
import {
  IssueView,
  IssueViewApiService,
  IssueViewStore,
  NotificationService,
  PatchIssueInput,
  ProjectApiService,
  ProjectComponent,
  ProjectVersion,
  asApiError,
  ISSUE_STALE_CODE,
  issuePath,
  CatalogStore,
} from '@corelog/core';
import { FieldEditor, FieldEditorOption } from '@corelog/ui/field-editor';
import { FieldRow } from '@corelog/ui/field-row';
import { FieldView } from '@corelog/ui/field-view';
import { InlineEdit } from '@corelog/ui/inline-edit';
import { Section } from '@corelog/ui/section';
import { UserAvatar } from '@corelog/ui/user-avatar';
import { PickableUser } from '@corelog/ui/user-picker';
import { Button } from 'primeng/button';
import { firstValueFrom } from 'rxjs';

import { injectMemberNames } from '../hooks/member-names';
import { injectTeammateSearch } from '../hooks/teammate-search';
import { IssuePicker } from '../issue-picker/issue-picker';
import { exactDate, timeAgo } from '../shared/time';

export const ISSUE_STALE_MESSAGE =
  'Alguien cambió esta incidencia mientras la editabas. Se cargaron los datos actuales; vuelve a intentarlo.';
const READ_ONLY_REASON = 'No tienes permiso para editar esta incidencia.';
const ARCHIVED_REASON = 'El proyecto está archivado y es de solo lectura.';

/**
 * Side panel with every editable field of the issue besides status, priority and assignee: type,
 * labels, components, versions, parent, points, dates, flag and the custom fields of the project.
 * Each change is one partial update through the view store (optimistic, with rollback and the
 * stale-update message). Editability follows the permissions of the viewer and the archived flag.
 */
@Component({
  selector: 'app-issue-fields-panel',
  imports: [
    Button,
    FieldEditor,
    FieldRow,
    FieldView,
    InlineEdit,
    IssuePicker,
    RouterLink,
    Section,
    UserAvatar,
  ],
  templateUrl: './issue-fields-panel.html',
  styleUrl: './issue-fields-panel.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class IssueFieldsPanel {
  private readonly viewStore = inject(IssueViewStore);
  private readonly viewApi = inject(IssueViewApiService);
  private readonly projectApi = inject(ProjectApiService);
  private readonly catalog = inject(CatalogStore);
  private readonly notify = inject(NotificationService);

  protected readonly names = injectMemberNames();
  protected readonly teammates = injectTeammateSearch();

  /** The project is archived: every field is read-only. */
  readonly archived = input(false);

  protected readonly view = this.viewStore.view;
  protected readonly labels = this.viewStore.labels;
  protected readonly components = signal<readonly ProjectComponent[]>([]);
  protected readonly versions = signal<readonly ProjectVersion[]>([]);
  protected readonly projectLabels = signal<readonly string[]>([]);
  protected readonly changingParent = signal(false);
  protected readonly parentPick = signal('');

  protected readonly canEdit = computed(() => !this.archived() && this.viewStore.can('edit_issue'));
  protected readonly lockedReason = computed(() =>
    this.archived() ? ARCHIVED_REASON : READ_ONLY_REASON,
  );

  protected readonly typeOptions = computed(() =>
    this.catalog.issueTypes().map((item) => ({ value: item.id, label: item.name })),
  );
  protected readonly componentOptions = computed<FieldEditorOption[]>(() =>
    this.components().map((item) => ({ id: item.id, label: item.name })),
  );
  protected readonly versionOptions = computed<FieldEditorOption[]>(() =>
    this.versions().map((item) => ({
      id: item.id,
      label: item.name,
      archived: item.archived,
    })),
  );
  protected readonly componentIds = computed(() => this.view()?.components.map((c) => c.id) ?? []);
  protected readonly fixVersionIds = computed(
    () => this.view()?.fixVersions.map((v) => v.id) ?? [],
  );
  protected readonly affectsVersionIds = computed(
    () => this.view()?.affectsVersions.map((v) => v.id) ?? [],
  );
  protected readonly pickableUsers = computed<PickableUser[]>(() => {
    const byId = new Map<string, PickableUser>();
    for (const member of this.names.members()) byId.set(member.id, { ...member });
    for (const user of this.teammates.results()) {
      byId.set(user.id, { id: user.id, name: user.name, email: user.email });
    }
    return Array.from(byId.values());
  });
  protected readonly labelHint = computed(() => {
    const known = this.projectLabels();
    return known.length > 0 ? `Existentes: ${known.slice(0, 8).join(', ')}` : undefined;
  });

  protected readonly userName = (id: string): string => this.names.nameOf(id);
  protected readonly issuePathOf = issuePath;
  protected readonly ago = timeAgo;
  protected readonly exact = exactDate;

  constructor() {
    this.catalog.ensureLoaded();
    effect(() => {
      const projectId = this.view()?.projectId;
      if (projectId) untracked(() => this.loadProjectLists(projectId));
    });
  }

  protected async save(
    input: PatchIssueInput,
    preview: Partial<IssueView>,
    message: string,
  ): Promise<void> {
    try {
      await this.viewStore.patch(input, preview);
      this.notify.success(message);
    } catch (error) {
      const apiError = asApiError(error);
      this.notify.error(
        apiError.code === ISSUE_STALE_CODE ? ISSUE_STALE_MESSAGE : apiError.message,
      );
    }
  }

  protected readonly saveType = async (next: string | null): Promise<void> => {
    if (!next) throw new Error('El tipo es obligatorio');
    try {
      await this.viewStore.patch({ issueTypeId: next });
      this.notify.success('Tipo actualizado');
    } catch (error) {
      throw new Error(this.messageOf(error));
    }
  };

  protected readonly saveStart = (next: string | null): Promise<void> =>
    this.saveInline({ startDate: next }, 'Fecha de inicio actualizada');

  protected readonly saveDue = (next: string | null): Promise<void> =>
    this.saveInline({ dueDate: next }, 'Fecha límite actualizada');

  protected readonly savePoints = async (next: string | null): Promise<void> => {
    const text = next?.trim().replace(',', '.') ?? '';
    if (text === '') return this.saveInline({ storyPoints: null }, 'Puntos actualizados');
    const points = Number(text);
    if (Number.isNaN(points) || points < 0 || points > 999.5 || (points * 2) % 1 !== 0) {
      throw new Error('Usa un número entre 0 y 999,5, en pasos de 0,5');
    }
    return this.saveInline({ storyPoints: points }, 'Puntos actualizados');
  };

  protected setFlag(flagged: unknown): void {
    void this.save(
      { flagged: flagged === true },
      { flagged: flagged === true },
      'Marca actualizada',
    );
  }

  protected setLabels(value: unknown): void {
    const labels = Array.isArray(value) ? value.map(String) : [];
    void this.save({ labels }, {}, 'Etiquetas actualizadas');
  }

  protected setComponents(value: unknown): void {
    const ids = Array.isArray(value) ? value.map(String) : [];
    const preview = {
      components: this.components()
        .filter((item) => ids.includes(item.id))
        .map((item) => ({ id: item.id, name: item.name })),
    };
    void this.save({ components: ids }, preview, 'Componentes actualizados');
  }

  protected setVersions(kind: 'fix' | 'affects', value: unknown): void {
    const ids = Array.isArray(value) ? value.map(String) : [];
    const refs = this.versions()
      .filter((item) => ids.includes(item.id))
      .map((item) => ({ id: item.id, name: item.name }));
    if (kind === 'fix') {
      void this.save({ fixVersions: ids }, { fixVersions: refs }, 'Versiones actualizadas');
    } else {
      void this.save({ affectsVersions: ids }, { affectsVersions: refs }, 'Versiones actualizadas');
    }
  }

  protected setCustom(fieldId: string, value: unknown): void {
    const view = this.view();
    if (!view) return;
    void this.save(
      { custom: { [fieldId]: value } },
      { custom: { ...view.custom, [fieldId]: value } },
      'Campo actualizado',
    );
  }

  protected openParentPicker(): void {
    this.parentPick.set('');
    this.changingParent.set(true);
  }

  protected applyParent(id: string): void {
    this.parentPick.set(id);
    if (!id) return;
    this.changingParent.set(false);
    void this.save({ parentId: id }, {}, 'Incidencia padre actualizada');
  }

  protected clearParent(): void {
    this.changingParent.set(false);
    void this.save({ parentId: null }, {}, 'Incidencia padre quitada');
  }

  protected customValue(fieldId: string): unknown {
    return this.view()?.custom[fieldId] ?? null;
  }

  private async saveInline(input: PatchIssueInput, message: string): Promise<void> {
    try {
      await this.viewStore.patch(input);
      this.notify.success(message);
    } catch (error) {
      throw new Error(this.messageOf(error));
    }
  }

  private messageOf(error: unknown): string {
    const apiError = asApiError(error);
    return apiError.code === ISSUE_STALE_CODE ? ISSUE_STALE_MESSAGE : apiError.message;
  }

  private async loadProjectLists(projectId: string): Promise<void> {
    const [components, versions, labels] = await Promise.all([
      firstValueFrom(this.projectApi.listComponents(projectId)).catch(() => []),
      firstValueFrom(this.viewApi.listVersions(projectId)).catch(() => []),
      firstValueFrom(this.projectApi.listLabels(projectId)).catch(() => []),
    ]);
    this.components.set(components);
    this.versions.set(versions);
    this.projectLabels.set(labels.map((label) => label.name));
  }
}
