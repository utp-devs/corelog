import { Injectable, computed, inject, signal } from '@angular/core';
import { Observable, catchError, forkJoin, map, of, tap } from 'rxjs';

import type { BacklogItem } from '../models/backlog.model';
import type {
  BurndownPoint,
  CreateSprintInput,
  Sprint,
  SprintIssue,
  SprintUnfinishedPolicy,
  VelocityPoint,
} from '../models/sprint.model';
import { BacklogApiService } from '../services/api/backlog-api.service';
import { SprintApiService } from '../services/api/sprint-api.service';
import { asApiError } from '../utils/api-error';
import { AsyncState } from './async-state';
import { resetOnLogout } from './session-scoped';

/** Sprints of the project with the velocity of its closed sprints. */
export interface SprintOverview {
  readonly sprints: readonly Sprint[];
  readonly velocity: readonly VelocityPoint[];
}

/** Data of the selected sprint. The sprint id keeps stale data from showing after a switch. */
export interface SprintDetail {
  readonly sprintId: string;
  readonly issues: readonly SprintIssue[];
  readonly burndown: readonly BurndownPoint[];
  readonly backlog: readonly BacklogItem[];
}

/** Form for planning a new sprint. */
export interface SprintDraft {
  readonly name: string;
  readonly goal: string;
  readonly startDate: string;
  readonly endDate: string;
}

export interface SprintIssueTotals {
  readonly count: number;
  readonly points: number;
}

const EMPTY_DRAFT: SprintDraft = { name: '', goal: '', startDate: '', endDate: '' };
const MESSAGE_NAME_REQUIRED = 'El nombre del sprint es obligatorio.';
const MESSAGE_DETAIL_FAILED = 'No se pudieron cargar las incidencias del sprint.';
const MESSAGE_OVERVIEW_FAILED =
  'No se pudieron cargar los sprints. Verifica que tengas acceso al proyecto e intenta de nuevo.';
const MESSAGE_ACTION_FAILED = 'No se pudo completar la acción. Intenta de nuevo.';

/**
 * Sprint planning for one project: the list, the selected sprint with its issues, burndown and
 * assignable backlog, the lifecycle actions (plan, start, close) and the velocity.
 * Every action reloads the overview and the selected sprint on success, so the page always shows
 * the server state; a failure leaves the data untouched and stores a Spanish message.
 */
@Injectable({ providedIn: 'root' })
export class SprintStore {
  private readonly sprintApi = inject(SprintApiService);
  private readonly backlogApi = inject(BacklogApiService);
  private projectId: string | null = null;

  private readonly selectedState = signal<string | undefined>(undefined);
  private readonly actionErrorState = signal<string | null>(null);
  private readonly draftState = signal<SprintDraft>(EMPTY_DRAFT);
  private readonly closePolicyState = signal<SprintUnfinishedPolicy>('backlog');
  private readonly closeTargetState = signal<string | undefined>(undefined);
  private readonly issueToAddState = signal<string | undefined>(undefined);

  readonly overview = new AsyncState<SprintOverview>();
  readonly detail = new AsyncState<SprintDetail>();

  readonly selectedId = this.selectedState.asReadonly();
  readonly draft = this.draftState.asReadonly();
  readonly closePolicy = this.closePolicyState.asReadonly();
  readonly closeTargetId = this.closeTargetState.asReadonly();
  readonly issueToAdd = this.issueToAddState.asReadonly();

  readonly sprints = computed(() => this.overview.value()?.sprints ?? []);
  readonly velocity = computed(() => this.overview.value()?.velocity ?? []);
  readonly selected = computed(() =>
    this.sprints().find((sprint) => sprint.id === this.selectedState()),
  );
  readonly plannedTargets = computed(() =>
    this.sprints().filter((sprint) => sprint.status === 'planned'),
  );

  private readonly currentDetail = computed(() => {
    const detail = this.detail.value();
    return detail && detail.sprintId === this.selectedState() ? detail : undefined;
  });
  readonly issues = computed<readonly SprintIssue[]>(() => this.currentDetail()?.issues ?? []);
  readonly burndown = computed<readonly BurndownPoint[]>(
    () => this.currentDetail()?.burndown ?? [],
  );

  /** Backlog items that are not yet in the selected sprint. */
  readonly assignableBacklog = computed<readonly BacklogItem[]>(() => {
    const inSprint = new Set(this.issues().map((issue) => issue.id));
    return (this.currentDetail()?.backlog ?? []).filter((item) => !inSprint.has(item.id));
  });

  readonly issueTotals = computed<SprintIssueTotals>(() => ({
    count: this.issues().length,
    points: this.issues().reduce((total, issue) => total + (issue.storyPoints ?? 0), 0),
  }));

  /** Closing with "another sprint" needs a planned target. */
  readonly canClose = computed(
    () => this.closePolicyState() === 'backlog' || this.closeTargetState() !== undefined,
  );

  /** Error of the last action, or of loading the selected sprint's details. */
  readonly actionError = computed(() => {
    const actionError = this.actionErrorState();
    if (actionError) {
      return actionError;
    }
    return this.detail.error() ? MESSAGE_DETAIL_FAILED : null;
  });

  readonly loadErrorMessage = computed(() =>
    this.overview.error() ? MESSAGE_OVERVIEW_FAILED : null,
  );

  constructor() {
    resetOnLogout(() => this.reset());
  }

  /** Loads the sprints of a project and selects the active sprint, else the planned one, else the first. */
  load(projectId: string): void {
    this.projectId = projectId;
    this.selectedState.set(undefined);
    this.actionErrorState.set(null);
    this.overview.load(this.fetchOverview(projectId));
  }

  retry(): void {
    if (this.projectId) {
      this.load(this.projectId);
    }
  }

  selectSprint(sprintId: string): void {
    const projectId = this.projectId;
    if (!projectId) {
      return;
    }
    this.selectedState.set(sprintId);
    this.loadDetail(projectId, sprintId);
  }

  setDraft(patch: Partial<SprintDraft>): void {
    this.draftState.update((current) => ({ ...current, ...patch }));
  }

  setCloseMode(policy: SprintUnfinishedPolicy): void {
    this.closePolicyState.set(policy);
  }

  setCloseTarget(sprintId: string | undefined): void {
    this.closeTargetState.set(sprintId);
  }

  setIssueToAdd(issueId: string | undefined): void {
    this.issueToAddState.set(issueId);
  }

  /** Plans a sprint from the draft. The draft is cleared only when the sprint is created. */
  create(): void {
    const draft = this.draftState();
    if (!draft.name.trim()) {
      this.actionErrorState.set(MESSAGE_NAME_REQUIRED);
      return;
    }
    const projectId = this.projectId;
    if (!projectId) {
      return;
    }
    const input: CreateSprintInput = {
      name: draft.name,
      goal: draft.goal,
      startDate: draft.startDate,
      endDate: draft.endDate,
    };
    this.run(this.sprintApi.create(projectId, input), (created) => {
      this.draftState.set(EMPTY_DRAFT);
      return created.id;
    });
  }

  start(sprintId: string): void {
    const projectId = this.projectId;
    if (!projectId) {
      return;
    }
    this.run(this.sprintApi.start(projectId, sprintId), () => sprintId);
  }

  /** Closes a sprint. Closing into another sprint needs a target; without one nothing happens. */
  close(sprintId: string): void {
    const projectId = this.projectId;
    const policy = this.closePolicyState();
    const target = this.closeTargetState();
    if (!projectId || !this.canClose()) {
      return;
    }
    this.run(
      this.sprintApi.close(projectId, sprintId, policy, policy === 'sprint' ? target : undefined),
    );
  }

  /** Adds the chosen backlog item to the sprint. The choice is cleared even if the request fails. */
  addChosenIssue(sprintId: string): void {
    const projectId = this.projectId;
    const issueId = this.issueToAddState();
    if (!projectId || !issueId) {
      return;
    }
    this.issueToAddState.set(undefined);
    this.run(this.sprintApi.assignIssue(projectId, sprintId, issueId), () => sprintId);
  }

  removeIssue(sprintId: string, issueId: string): void {
    const projectId = this.projectId;
    if (!projectId) {
      return;
    }
    this.run(this.sprintApi.removeIssue(projectId, sprintId, issueId), () => sprintId);
  }

  /** Dismisses the error of the last action (the banner was closed). */
  clearActionError(): void {
    this.actionErrorState.set(null);
  }

  reset(): void {
    this.projectId = null;
    this.selectedState.set(undefined);
    this.actionErrorState.set(null);
    this.draftState.set(EMPTY_DRAFT);
    this.closePolicyState.set('backlog');
    this.closeTargetState.set(undefined);
    this.issueToAddState.set(undefined);
    this.overview.reset();
    this.detail.reset();
  }

  /** Runs a mutation. On success the overview is reloaded and `preferSelected` picks the selection. */
  private run<T>(
    request: Observable<T>,
    preferSelected: (value: T) => string | undefined = () => undefined,
  ): void {
    const projectId = this.projectId;
    if (!projectId) {
      return;
    }
    this.actionErrorState.set(null);
    request.subscribe({
      next: (value) => this.overview.load(this.fetchOverview(projectId, preferSelected(value))),
      error: (error: unknown) => this.actionErrorState.set(actionErrorMessage(asApiError(error))),
    });
  }

  private fetchOverview(projectId: string, preferredId?: string): Observable<SprintOverview> {
    return forkJoin({
      sprints: this.sprintApi.list(projectId),
      velocity: this.sprintApi.velocity(projectId),
    }).pipe(
      tap(({ sprints }) => this.selectAfterLoad(projectId, sprints, preferredId)),
      map(({ sprints, velocity }) => ({ sprints, velocity })),
    );
  }

  /** Keeps the preferred or current sprint when it still exists; otherwise active, planned or first. */
  private selectAfterLoad(
    projectId: string,
    sprints: readonly Sprint[],
    preferredId?: string,
  ): void {
    const current = preferredId ?? this.selectedState();
    const next =
      sprints.find((sprint) => sprint.id === current) ??
      sprints.find((sprint) => sprint.status === 'active') ??
      sprints.find((sprint) => sprint.status === 'planned') ??
      sprints[0];
    this.selectedState.set(next?.id);
    if (next) {
      this.loadDetail(projectId, next.id);
    } else {
      this.detail.reset();
    }
  }

  private loadDetail(projectId: string, sprintId: string): void {
    this.detail.load(
      forkJoin({
        issues: this.sprintApi.listIssues(projectId, sprintId),
        burndown: this.sprintApi
          .burndown(projectId, sprintId)
          .pipe(catchError(() => of<BurndownPoint[]>([]))),
        backlog: this.backlogApi.list(projectId),
      }).pipe(map(({ issues, burndown, backlog }) => ({ sprintId, issues, burndown, backlog }))),
    );
  }
}

function actionErrorMessage(error: { readonly status: number }): string {
  if (error.status === 403) {
    return 'Solo el lead del proyecto o el administrador pueden gestionar sprints.';
  }
  if (error.status === 409) {
    return 'Esa acción no está permitida en el estado actual del sprint o ya hay otro sprint activo.';
  }
  if (error.status === 422) {
    return 'Revisa los datos del sprint: fechas en formato AAAA-MM-DD y fin posterior al inicio.';
  }
  return MESSAGE_ACTION_FAILED;
}
