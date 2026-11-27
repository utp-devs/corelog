import { Injectable, computed, inject, signal } from '@angular/core';
import { Observable, catchError, map, tap, throwError } from 'rxjs';

import {
  WorkflowIssue,
  WorkflowSaveResult,
  WorkflowTemplate,
  WorkflowTemplateId,
  WorkflowValidation,
  WorkflowView,
} from '../models/workflow.model';
import { WorkflowApiService } from '../services/api/workflow-api.service';
import { asApiError } from '../utils/api-error';
import {
  WorkflowDraft,
  draftFromView,
  draftToInput,
  draftsEqual,
  removedStatuses,
} from '../utils/workflow-draft';
import { AsyncState } from './async-state';
import { resetOnLogout } from './session-scoped';

/**
 * Workflow of the project being edited: the saved version, an editable draft, the last validation and the
 * saving state. The draft is replaced through `edit`; `save` and `applyTemplate` return cold observables
 * that fail with the normalized ApiError (version conflict, invalid definition, mapping required).
 */
@Injectable({ providedIn: 'root' })
export class WorkflowStore {
  private readonly api = inject(WorkflowApiService);
  private projectId: string | null = null;

  readonly workflow = new AsyncState<WorkflowView>();
  readonly templates = new AsyncState<WorkflowTemplate[]>();

  private readonly draftState = signal<WorkflowDraft | null>(null);
  private readonly baselineState = signal<WorkflowDraft | null>(null);
  private readonly validationState = signal<WorkflowValidation | null>(null);
  private readonly warningsState = signal<readonly WorkflowIssue[]>([]);
  private readonly savingState = signal(false);
  private readonly validatingState = signal(false);

  readonly draft = this.draftState.asReadonly();
  readonly baseline = this.baselineState.asReadonly();
  readonly validation = this.validationState.asReadonly();
  readonly warnings = this.warningsState.asReadonly();
  readonly saving = this.savingState.asReadonly();
  readonly validating = this.validatingState.asReadonly();
  readonly version = computed(() => this.workflow.value()?.version ?? 0);
  readonly isTemplateDefault = computed(() => this.workflow.value()?.isTemplateDefault ?? false);

  readonly dirty = computed(() => {
    const draft = this.draftState();
    const baseline = this.baselineState();
    return draft !== null && baseline !== null && !draftsEqual(draft, baseline);
  });

  /** Saved statuses that the draft removes while issues still sit in them. */
  readonly removedWithIssues = computed(() => {
    const draft = this.draftState();
    const baseline = this.baselineState();
    if (!draft || !baseline) return [];
    return removedStatuses(baseline, draft).filter((status) => status.issueCount > 0);
  });

  constructor() {
    resetOnLogout(() => this.reset());
  }

  load(projectId: string): void {
    this.projectId = projectId;
    this.clearResults();
    this.workflow.load(
      this.api.get(projectId).pipe(
        tap((view) => {
          if (this.projectId === projectId) this.adopt(view);
        }),
      ),
    );
  }

  ensureTemplates(): void {
    if (this.templates.hasValue() || this.templates.loading()) return;
    this.templates.load(this.api.listTemplates());
  }

  edit(change: (draft: WorkflowDraft) => WorkflowDraft): void {
    const current = this.draftState();
    if (!current) return;
    this.draftState.set(change(current));
    // A validation of an older draft no longer says anything about this one.
    this.validationState.set(null);
  }

  /** A saved status was renamed in the catalog: the baseline and the draft follow without making the draft dirty. */
  applyRename(ref: string, name: string): void {
    const rename = (draft: WorkflowDraft | null): WorkflowDraft | null =>
      draft && {
        ...draft,
        statuses: draft.statuses.map((status) =>
          status.ref === ref ? { ...status, name } : status,
        ),
      };
    this.baselineState.update(rename);
    this.draftState.update(rename);
  }

  discard(): void {
    this.draftState.set(this.baselineState());
    this.clearResults();
  }

  validate(statusMapping: Readonly<Record<string, string>> = {}): Observable<WorkflowValidation> {
    const projectId = this.projectId;
    const draft = this.draftState();
    if (!projectId || !draft) return throwError(() => new Error('workflow not loaded'));
    this.validatingState.set(true);
    return this.api.validate(projectId, draftToInput(draft, this.version(), statusMapping)).pipe(
      tap({
        next: (result) => {
          this.validatingState.set(false);
          this.validationState.set(result);
        },
        error: () => this.validatingState.set(false),
      }),
      catchError((error: unknown) => throwError(() => asApiError(error))),
    );
  }

  save(statusMapping: Readonly<Record<string, string>> = {}): Observable<WorkflowSaveResult> {
    const projectId = this.projectId;
    const draft = this.draftState();
    if (!projectId || !draft) return throwError(() => new Error('workflow not loaded'));
    this.savingState.set(true);
    return this.api.save(projectId, draftToInput(draft, this.version(), statusMapping)).pipe(
      tap({
        next: (result) => this.finishSave(result),
        error: () => this.savingState.set(false),
      }),
      catchError((error: unknown) => throwError(() => asApiError(error))),
    );
  }

  applyTemplate(
    template: WorkflowTemplateId,
    statusMapping: Readonly<Record<string, string>> = {},
  ): Observable<WorkflowSaveResult> {
    const projectId = this.projectId;
    if (!projectId) return throwError(() => new Error('workflow not loaded'));
    this.savingState.set(true);
    return this.api.reset(projectId, template, this.version(), statusMapping).pipe(
      tap({
        next: (result) => this.finishSave(result),
        error: () => this.savingState.set(false),
      }),
      catchError((error: unknown) => throwError(() => asApiError(error))),
    );
  }

  /** Moves the issues of some statuses to others without saving the workflow; returns how many moved. */
  migrateIssues(mapping: Readonly<Record<string, string>>): Observable<number> {
    const projectId = this.projectId;
    if (!projectId) return throwError(() => new Error('workflow not loaded'));
    return this.api.migrateIssues(projectId, mapping).pipe(
      map((result) => result.moved),
      catchError((error: unknown) => throwError(() => asApiError(error))),
    );
  }

  reset(): void {
    this.projectId = null;
    this.workflow.reset();
    this.draftState.set(null);
    this.baselineState.set(null);
    this.clearResults();
  }

  private finishSave(result: WorkflowSaveResult): void {
    this.savingState.set(false);
    this.workflow.set(result.workflow);
    this.adopt(result.workflow);
    this.warningsState.set(result.warnings);
    this.validationState.set(null);
  }

  private adopt(view: WorkflowView): void {
    const draft = draftFromView(view);
    this.baselineState.set(draft);
    this.draftState.set(draft);
  }

  private clearResults(): void {
    this.validationState.set(null);
    this.warningsState.set([]);
  }
}
