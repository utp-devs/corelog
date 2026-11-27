import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  signal,
  untracked,
} from '@angular/core';
import {
  CatalogStore,
  CurrentProjectStore,
  DraftStatus,
  NotificationService,
  OrganizationStore,
  StatusCatalogStore,
  UnsavedChangesAware,
  WorkflowDraft,
  WorkflowIssue,
  WorkflowMappingRequirement,
  WorkflowStatusRef,
  WorkflowStore,
  WorkflowTemplateId,
  WorkflowTransition,
  addStatus,
  addTransition,
  canManageOrg,
  duplicateTransition,
  isApiError,
  isWorkflowVersionConflict,
  moveStatus,
  removeStatus,
  removeTransition,
  renameStatus,
  setInitial,
  setStatusCategory,
  updateTransition,
  warnBeforeUnload,
  workflowErrorMessage,
} from '@corelog/core';
import { ErrorState } from '@corelog/ui/error-state';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { SelectFieldOption } from '@corelog/ui/select-field';
import { Section } from '@corelog/ui/section';
import { WorkflowDiagram } from '@corelog/ui/workflow-diagram';
import { Button } from 'primeng/button';
import { Message } from 'primeng/message';

import { WorkflowMappingDialog } from './mapping-dialog';
import { NewStatus, StatusMove, WorkflowStatusList } from './status-list';
import { WorkflowTemplatesDialog } from './templates-dialog';
import { TransitionPatch, WorkflowTransitionPanel } from './transition-panel';
import { WorkflowTransitionTable } from './transition-table';

type EditorTab = 'table' | 'diagram';
type MappingPurpose = 'remove' | 'validate' | 'save' | 'template';

interface PendingMapping {
  readonly purpose: MappingPurpose;
  readonly requirements: readonly WorkflowMappingRequirement[];
  /** Status that is being removed (purpose `remove`) or the template to apply (purpose `template`). */
  readonly subject: string | null;
}

interface ShownIssue {
  readonly path: string;
  readonly text: string;
  readonly severity: 'error' | 'warning';
}

/**
 * Workflow of a project: statuses, transitions, typed rules and an automatic diagram. People with the
 * manage-workflow permission edit a draft that they validate and save in one step (with an optimistic
 * version check); everyone else reads. Leaving with unsaved changes asks first.
 */
@Component({
  selector: 'app-workflow-editor-page',
  imports: [
    Button,
    ErrorState,
    LoadingSkeleton,
    Message,
    Section,
    WorkflowDiagram,
    WorkflowMappingDialog,
    WorkflowStatusList,
    WorkflowTemplatesDialog,
    WorkflowTransitionPanel,
    WorkflowTransitionTable,
  ],
  templateUrl: './workflow-editor-page.html',
  styleUrl: './workflow-editor-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class WorkflowEditorPage implements UnsavedChangesAware {
  protected readonly store = inject(WorkflowStore);
  private readonly current = inject(CurrentProjectStore);
  private readonly orgs = inject(OrganizationStore);
  private readonly catalogStatuses = inject(StatusCatalogStore);
  private readonly catalog = inject(CatalogStore);
  private readonly notify = inject(NotificationService);

  readonly dirty = this.store.dirty;

  protected readonly tab = signal<EditorTab>('table');
  protected readonly selectedId = signal<string | null>(null);
  protected readonly mapping = signal<Readonly<Record<string, string>>>({});
  protected readonly pendingMapping = signal<PendingMapping | null>(null);
  protected readonly templatesOpen = signal(false);
  protected readonly templateError = signal<string | null>(null);
  protected readonly shownIssues = signal<readonly ShownIssue[]>([]);
  protected readonly validated = signal<'valid' | 'invalid' | null>(null);
  protected readonly conflict = signal(false);
  protected readonly message = signal<string | null>(null);
  protected readonly announcement = signal('');

  protected readonly draft = this.store.draft;
  protected readonly loading = computed(
    () => this.store.workflow.loading() && !this.store.workflow.hasValue(),
  );
  protected readonly loadError = computed(() =>
    this.store.workflow.hasValue() ? null : this.store.workflow.error(),
  );
  protected readonly statuses = computed<readonly DraftStatus[]>(
    () => this.draft()?.statuses ?? [],
  );
  protected readonly diagramStatuses = computed<readonly WorkflowStatusRef[]>(() =>
    this.statuses().map((status) => ({
      id: status.ref,
      name: status.name,
      category: status.category,
    })),
  );
  protected readonly transitions = computed<readonly WorkflowTransition[]>(
    () => this.draft()?.transitions ?? [],
  );
  protected readonly selected = computed(
    () => this.transitions().find((item) => item.id === this.selectedId()) ?? null,
  );
  protected readonly canEdit = computed(
    () => this.current.can('manage_workflow') && !this.current.isArchived(),
  );
  protected readonly canRenameSaved = computed(() => canManageOrg(this.orgs.currentRole()));
  protected readonly catalogItems = this.catalogStatuses.statuses;
  protected readonly resolutionOptions = computed<SelectFieldOption[]>(() =>
    this.catalog.resolutions().map((item) => ({ value: item.id, label: item.name })),
  );
  protected readonly busy = computed(() => this.store.saving() || this.store.validating());
  protected readonly templates = this.store.templates.value;

  /** Problems of the last validation or save, grouped by the transition they point at. */
  protected readonly problemsOfSelected = computed<string[]>(() => {
    const draft = this.draft();
    const selected = this.selected();
    if (!draft || !selected) return [];
    const index = draft.transitions.findIndex((item) => item.id === selected.id);
    const prefix = `transitions[${index}]`;
    return this.shownIssues()
      .filter((issue) => issue.path.startsWith(prefix))
      .map((issue) => issue.text);
  });

  protected readonly mappingTargets = computed<SelectFieldOption[]>(() => {
    const pending = this.pendingMapping();
    const removed = new Set(pending?.requirements.map((item) => item.statusId));
    return this.statuses()
      .filter((status) => status.id !== null && !removed.has(status.id))
      .map((status) => ({ value: status.id as string, label: status.name }));
  });

  constructor() {
    warnBeforeUnload(this.store.dirty);
    this.catalogStatuses.ensureLoaded();
    this.catalog.ensureLoaded();
    effect(() => {
      const id = this.current.projectId();
      if (id) {
        untracked(() => {
          this.resetEditorState();
          this.store.load(id);
        });
      }
    });
  }

  protected retry(): void {
    const id = this.current.projectId();
    if (id) this.store.load(id);
  }

  protected reload(): void {
    this.conflict.set(false);
    this.retry();
  }

  protected setTab(tab: EditorTab): void {
    this.tab.set(tab);
  }

  protected onTabKeydown(event: KeyboardEvent): void {
    if (event.key === 'ArrowRight' || event.key === 'ArrowLeft') {
      event.preventDefault();
      const next: EditorTab = this.tab() === 'table' ? 'diagram' : 'table';
      this.tab.set(next);
      document.getElementById(`workflow-tab-${next}`)?.focus();
    }
  }

  // Statuses.

  protected addFromCatalog(id: string): void {
    const item = this.catalogItems().find((status) => status.id === id);
    if (!item) return;
    this.edit((draft) =>
      addStatus(draft, { id: item.id, name: item.name, category: item.category }),
    );
  }

  protected createStatus(input: NewStatus): void {
    this.edit((draft) => addStatus(draft, input));
  }

  protected renameDraftStatus(change: { ref: string; name: string }): void {
    const status = this.statuses().find((item) => item.ref === change.ref);
    if (!status) return;
    if (status.id === null) {
      this.edit((draft) => renameStatus(draft, change.ref, change.name));
      return;
    }
    this.catalogStatuses.update(status.id, { name: change.name }).subscribe({
      next: () => {
        this.store.applyRename(change.ref, change.name);
        this.announcement.set(`Estado renombrado a ${change.name}`);
      },
      error: (error: unknown) =>
        this.message.set(workflowErrorMessage(error, 'No se pudo renombrar el estado.')),
    });
  }

  protected changeCategory(change: { ref: string; category: DraftStatus['category'] }): void {
    this.edit((draft) => setStatusCategory(draft, change.ref, change.category));
  }

  protected markInitial(ref: string): void {
    this.edit((draft) => setInitial(draft, ref));
  }

  protected moveStatusItem(move: StatusMove): void {
    this.edit((draft) => moveStatus(draft, move.from, move.to));
  }

  protected removeStatusItem(ref: string): void {
    const status = this.statuses().find((item) => item.ref === ref);
    if (!status) return;
    if (status.id !== null && status.issueCount > 0) {
      this.pendingMapping.set({
        purpose: 'remove',
        subject: ref,
        requirements: [{ statusId: status.id, name: status.name, issueCount: status.issueCount }],
      });
      return;
    }
    this.edit((draft) => removeStatus(draft, ref));
  }

  // Transitions.

  protected selectTransition(id: string): void {
    this.selectedId.set(id);
  }

  protected closePanel(): void {
    this.selectedId.set(null);
  }

  protected addTransitionRow(): void {
    const statuses = this.statuses();
    if (statuses.length < 2) return;
    const initial = this.draft()?.initialRef ?? statuses[0].ref;
    const to = statuses.find((status) => status.ref !== initial) ?? statuses[1];
    let createdId = '';
    this.edit((draft) => {
      const result = addTransition(draft, {
        name: 'Nueva transición',
        from: [initial],
        to: to.ref,
      });
      createdId = result.id;
      return result.draft;
    });
    this.selectedId.set(createdId);
  }

  protected duplicateTransitionRow(id: string): void {
    let createdId = '';
    this.edit((draft) => {
      const result = duplicateTransition(draft, id);
      if (!result) return draft;
      createdId = result.id;
      return result.draft;
    });
    if (createdId) this.selectedId.set(createdId);
  }

  protected removeTransitionRow(id: string): void {
    this.edit((draft) => removeTransition(draft, id));
    if (this.selectedId() === id) this.selectedId.set(null);
  }

  protected patchTransition(patch: TransitionPatch): void {
    const id = this.selectedId();
    if (id) this.edit((draft) => updateTransition(draft, id, patch));
  }

  // Validate, save, discard.

  protected discard(): void {
    this.store.discard();
    this.resetEditorState(false);
  }

  protected validate(): void {
    this.message.set(null);
    const unmapped = this.unmappedRemovals();
    if (unmapped.length > 0) {
      this.pendingMapping.set({ purpose: 'validate', requirements: unmapped, subject: null });
      return;
    }
    this.runValidation();
  }

  protected save(): void {
    this.message.set(null);
    const unmapped = this.unmappedRemovals();
    if (unmapped.length > 0) {
      this.pendingMapping.set({ purpose: 'save', requirements: unmapped, subject: null });
      return;
    }
    this.runSave();
  }

  protected confirmMapping(chosen: Record<string, string>): void {
    const pending = this.pendingMapping();
    if (!pending) return;
    this.pendingMapping.set(null);
    this.mapping.update((current) => ({ ...current, ...chosen }));
    if (pending.purpose === 'remove' && pending.subject) {
      const ref = pending.subject;
      this.edit((draft) => removeStatus(draft, ref));
    } else if (pending.purpose === 'validate') {
      this.runValidation();
    } else if (pending.purpose === 'save') {
      this.runSave();
    } else if (pending.purpose === 'template' && pending.subject) {
      this.applyTemplate(pending.subject as WorkflowTemplateId);
    }
  }

  protected closeMapping(): void {
    this.pendingMapping.set(null);
  }

  // Templates.

  protected openTemplates(): void {
    this.templateError.set(null);
    this.store.ensureTemplates();
    this.templatesOpen.set(true);
  }

  protected closeTemplates(): void {
    if (!this.store.saving()) this.templatesOpen.set(false);
  }

  protected retryTemplates(): void {
    this.store.templates.reset();
    this.store.ensureTemplates();
  }

  protected applyTemplate(id: WorkflowTemplateId): void {
    this.templateError.set(null);
    this.store.applyTemplate(id, this.mapping()).subscribe({
      next: () => {
        this.templatesOpen.set(false);
        this.resetEditorState(false);
        this.mapping.set({});
        this.notify.success('Se aplicó la plantilla al flujo de trabajo');
      },
      error: (error: unknown) => {
        if (isApiError(error) && error.code === 'workflow.status_mapping_required') {
          this.templatesOpen.set(false);
          this.pendingMapping.set({
            purpose: 'template',
            subject: id,
            requirements: error.mappingRequired ?? [],
          });
          return;
        }
        if (isWorkflowVersionConflict(error)) {
          this.templatesOpen.set(false);
          this.conflict.set(true);
          return;
        }
        this.templateError.set(workflowErrorMessage(error, 'No se pudo aplicar la plantilla.'));
      },
    });
  }

  protected issuesTitle(): string {
    const issues = this.shownIssues();
    return issues.some((issue) => issue.severity === 'error')
      ? 'El flujo tiene errores'
      : 'Avisos del flujo';
  }

  // Internals.

  private edit(change: (draft: WorkflowDraft) => WorkflowDraft): void {
    this.store.edit(change);
    this.validated.set(null);
    this.shownIssues.set([]);
  }

  private resetEditorState(clearSelection = true): void {
    this.shownIssues.set([]);
    this.validated.set(null);
    this.conflict.set(false);
    this.message.set(null);
    this.mapping.set({});
    if (clearSelection) this.selectedId.set(null);
  }

  /** Saved statuses the draft removes while issues sit in them and no target was chosen yet. */
  private unmappedRemovals(): WorkflowMappingRequirement[] {
    const chosen = this.mapping();
    return this.store
      .removedWithIssues()
      .filter((status) => status.id !== null && !chosen[status.id])
      .map((status) => ({
        statusId: status.id as string,
        name: status.name,
        issueCount: status.issueCount,
      }));
  }

  /** Targets for the statuses removed now; entries for statuses that stay are not sent. */
  private effectiveMapping(): Record<string, string> {
    const removed = new Set(this.store.removedWithIssues().map((status) => status.id));
    return Object.fromEntries(Object.entries(this.mapping()).filter(([id]) => removed.has(id)));
  }

  private runValidation(): void {
    this.store.validate(this.effectiveMapping()).subscribe({
      next: (result) => {
        this.showIssues(result.errors, result.warnings);
        this.validated.set(result.valid ? 'valid' : 'invalid');
        this.announcement.set(
          result.valid ? 'El flujo es válido' : `El flujo tiene ${result.errors.length} errores`,
        );
        if (result.mappingRequired.length > 0) {
          this.pendingMapping.set({
            purpose: 'validate',
            requirements: result.mappingRequired,
            subject: null,
          });
        }
      },
      error: (error: unknown) =>
        this.message.set(workflowErrorMessage(error, 'No se pudo validar el flujo.')),
    });
  }

  private runSave(): void {
    this.store.save(this.effectiveMapping()).subscribe({
      next: (result) => {
        this.shownIssues.set([]);
        this.showIssues([], result.warnings);
        this.validated.set(null);
        this.mapping.set({});
        this.notify.success('Flujo de trabajo guardado');
        this.announcement.set('Flujo de trabajo guardado');
        const id = this.selectedId();
        if (id && !result.workflow.transitions.some((item) => item.id === id)) {
          this.selectedId.set(null);
        }
      },
      error: (error: unknown) => this.onSaveError(error),
    });
  }

  private onSaveError(error: unknown): void {
    if (isWorkflowVersionConflict(error)) {
      this.conflict.set(true);
      return;
    }
    if (isApiError(error) && error.code === 'workflow.status_mapping_required') {
      this.pendingMapping.set({
        purpose: 'save',
        subject: null,
        requirements: error.mappingRequired ?? [],
      });
      return;
    }
    if (isApiError(error) && error.code === 'workflow.invalid') {
      this.showIssues(
        error.details.map((detail) => ({
          path: detail.field,
          code: detail.code ?? '',
          message: detail.message,
        })),
        [],
      );
      this.validated.set('invalid');
      return;
    }
    this.message.set(workflowErrorMessage(error, 'No se pudo guardar el flujo.'));
  }

  private showIssues(errors: readonly WorkflowIssue[], warnings: readonly WorkflowIssue[]): void {
    const draft = this.draft();
    const describe = (issue: WorkflowIssue): string => {
      const transition = /^transitions\[(\d+)\]/.exec(issue.path);
      if (transition && draft) {
        const name = draft.transitions[Number(transition[1])]?.name;
        if (name) return `Transición «${name}»: ${issue.message}`;
      }
      const status = /^statuses\[(\d+)\]/.exec(issue.path);
      if (status && draft) {
        const name = draft.statuses[Number(status[1])]?.name;
        if (name) return `Estado «${name}»: ${issue.message}`;
      }
      return issue.message;
    };
    this.shownIssues.set([
      ...errors.map((issue) => ({
        path: issue.path,
        text: describe(issue),
        severity: 'error' as const,
      })),
      ...warnings.map((issue) => ({
        path: issue.path,
        text: describe(issue),
        severity: 'warning' as const,
      })),
    ]);
  }
}
