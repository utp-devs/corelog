import { Injectable, computed, inject, signal } from '@angular/core';
import { Observable, Subscription, catchError, switchMap, tap, throwError } from 'rxjs';

import type { ApiError } from '../models/api-error.model';
import type {
  BoardColumnInput,
  BoardDetail,
  BoardEstimation,
  BoardMoveInput,
  BoardQueryInput,
  BoardSummary,
  BoardTransitionChoice,
  BoardView,
  BoardViewCard,
  CreateBoardInput,
  UpdateBoardInput,
} from '../models/board-v2.model';
import type { Issue } from '../models/issue.model';
import { BacklogV2ApiService } from '../services/api/backlog-v2-api.service';
import { BoardV2ApiService } from '../services/api/board-v2-api.service';
import { IssueApiService } from '../services/api/issue-api.service';
import { StorageService } from '../services/storage.service';
import { asApiError } from '../utils/api-error';
import { MoveCardOptions, moveCardInView } from '../utils/board-view-ops';
import { AsyncState } from './async-state';
import { resetOnLogout } from './session-scoped';

const STORAGE_PREFIX = 'corelog.board.';

const MESSAGES = {
  forbidden: 'No tienes permiso para mover incidencias en este proyecto.',
  unavailable:
    'El flujo de trabajo no tiene una transición que lleve la incidencia a esa columna desde su estado actual.',
  stale:
    'Otra persona cambió el estado de la incidencia. El tablero se actualizó; inténtalo de nuevo.',
  archived: 'El proyecto está archivado y es de solo lectura.',
  invalid: 'No se puede soltar la incidencia ahí.',
  failed: 'No se pudo mover la incidencia. Intenta de nuevo.',
  wipOver: 'La columna superó su límite de trabajo en curso.',
} as const;

/** Spanish message for a failed card move, by the code of the server answer. */
export function boardMoveErrorMessage(error: ApiError): string {
  switch (error.code) {
    case 'issue.transition_unavailable':
      return MESSAGES.unavailable;
    case 'issue.status_conflict':
    case 'board.rank_conflict':
      return MESSAGES.stale;
    case 'project.archived':
      return MESSAGES.archived;
    case 'board.invalid_move':
      return MESSAGES.invalid;
    default:
      return error.status === 403 ? MESSAGES.forbidden : MESSAGES.failed;
  }
}

/** Spanish message for a failed change of the board configuration. */
export function boardSettingsErrorMessage(error: ApiError): string {
  switch (error.code) {
    case 'board.name_taken':
      return 'Ya existe un tablero con ese nombre en el proyecto.';
    case 'board.too_many':
      return 'El proyecto alcanzó el máximo de 20 tableros.';
    case 'board.default_not_deletable':
      return 'El tablero principal del proyecto no se puede eliminar.';
    case 'board.invalid_columns':
      return (
        error.details[0]?.message ||
        'Las columnas no son válidas. Revisa el nombre y los estados de cada una.'
      );
    case 'board.invalid_query':
      return 'La consulta no es válida. Usa la gramática de los ejemplos, por ejemplo: assignee = currentUser().';
    case 'project.archived':
      return MESSAGES.archived;
    default:
      return error.status === 403
        ? 'Solo quien administra el tablero puede cambiar su configuración.'
        : 'No se pudo guardar la configuración. Intenta de nuevo.';
  }
}

/** A move the server could not resolve alone: several transitions lead to the column. */
export interface BoardMoveChoice {
  readonly cardId: string;
  readonly columnId: string;
  readonly options: MoveCardOptions;
  readonly candidates: readonly BoardTransitionChoice[];
}

/** A move whose transition asks the person for data (fields, comment, resolution). */
export interface BoardMoveInputRequest {
  readonly cardId: string;
  readonly transitionId: string;
}

/**
 * Boards of the project in view: the list, the selected board, its view with quick filters and the
 * card moves. A move shows at once (optimistic) and is undone when the server refuses it.
 */
@Injectable({ providedIn: 'root' })
export class BoardViewStore {
  private readonly api = inject(BoardV2ApiService);
  private readonly issues = inject(IssueApiService);
  private readonly backlogApi = inject(BacklogV2ApiService);
  private readonly storage = inject(StorageService);

  private projectId: string | null = null;
  private refreshSubscription: Subscription | null = null;

  private readonly boardIdState = signal<string | null>(null);
  private readonly filterIdsState = signal<readonly string[]>([]);
  private readonly estimationState = signal<BoardEstimation | null>(null);
  private readonly collapsedState = signal<ReadonlySet<string>>(new Set());
  private readonly moveErrorState = signal<string | null>(null);
  private readonly announcementState = signal('');
  private readonly wipWarningState = signal<string | null>(null);
  private readonly choiceState = signal<BoardMoveChoice | null>(null);
  private readonly inputRequestState = signal<BoardMoveInputRequest | null>(null);
  private readonly busyState = signal(false);
  private readonly refreshingState = signal(false);

  readonly boards = new AsyncState<BoardSummary[]>();
  readonly view = new AsyncState<BoardView>();

  readonly boardId = this.boardIdState.asReadonly();
  /** Ids of the quick filters that are on. */
  readonly activeFilterIds = this.filterIdsState.asReadonly();
  readonly collapsedLanes = this.collapsedState.asReadonly();
  readonly moveError = this.moveErrorState.asReadonly();
  /** Polite live-region text after a move. */
  readonly announcement = this.announcementState.asReadonly();
  /** Advisory warning when the last move left a column over its limit. */
  readonly wipWarning = this.wipWarningState.asReadonly();
  readonly choice = this.choiceState.asReadonly();
  readonly inputRequest = this.inputRequestState.asReadonly();
  /** A settings write is in flight. */
  readonly busy = this.busyState.asReadonly();
  /** The view is being read again behind the board shown. */
  readonly refreshing = this.refreshingState.asReadonly();

  /** What column headers add up: the person's choice, else the board's setting. */
  readonly estimation = computed<BoardEstimation>(
    () => this.estimationState() ?? this.view.value()?.board.estimation ?? 'story_points',
  );
  readonly canMove = computed(() => this.view.value()?.board.canMoveCards ?? false);
  readonly canManage = computed(() => this.view.value()?.board.canManageBoard ?? false);

  constructor() {
    resetOnLogout(() => this.reset());
  }

  /** Opens the boards of a project and shows the remembered one, else the main board. */
  open(projectId: string): void {
    this.projectId = projectId;
    this.filterIdsState.set([]);
    this.moveErrorState.set(null);
    this.view.reset();
    this.boards.load(
      this.api.list(projectId).pipe(
        tap((list) => {
          const remembered = this.storage.getItem(STORAGE_PREFIX + projectId);
          const pick =
            list.find((board) => board.id === remembered) ??
            list.find((board) => board.isDefault) ??
            list[0];
          if (pick) this.select(pick.id, false);
        }),
      ),
    );
  }

  retry(): void {
    if (this.projectId) this.open(this.projectId);
  }

  /** Shows another board of the project. Quick filters belong to a board, so they are cleared. */
  select(boardId: string, remember = true): void {
    this.boardIdState.set(boardId);
    this.filterIdsState.set([]);
    this.estimationState.set(null);
    this.refreshSubscription?.unsubscribe();
    this.refreshingState.set(false);
    if (remember && this.projectId) this.storage.setItem(STORAGE_PREFIX + this.projectId, boardId);
    this.view.load(this.api.view(boardId, { quickFilterIds: [] }));
  }

  /** Reads the view again without clearing the board on screen. */
  refresh(): void {
    const boardId = this.boardIdState();
    if (!boardId) return;
    this.refreshSubscription?.unsubscribe();
    this.refreshingState.set(true);
    this.refreshSubscription = this.api
      .view(boardId, { quickFilterIds: this.filterIdsState() })
      .subscribe({
        next: (view) => {
          this.view.set(view);
          this.refreshingState.set(false);
        },
        error: () => this.refreshingState.set(false),
      });
  }

  toggleFilter(filterId: string): void {
    this.filterIdsState.update((ids) =>
      ids.includes(filterId) ? ids.filter((id) => id !== filterId) : [...ids, filterId],
    );
    this.refresh();
  }

  clearFilters(): void {
    if (this.filterIdsState().length === 0) return;
    this.filterIdsState.set([]);
    this.refresh();
  }

  setEstimation(estimation: BoardEstimation): void {
    this.estimationState.set(estimation);
  }

  toggleLane(laneId: string): void {
    this.collapsedState.update((current) => {
      const next = new Set(current);
      if (!next.delete(laneId)) next.add(laneId);
      return next;
    });
  }

  clearMoveError(): void {
    this.moveErrorState.set(null);
  }

  clearWipWarning(): void {
    this.wipWarningState.set(null);
  }

  clearChoice(): void {
    this.choiceState.set(null);
  }

  clearInputRequest(): void {
    this.inputRequestState.set(null);
  }

  /** Name of a column, for announcements. */
  columnName(columnId: string): string {
    return (
      this.view.value()?.columns.find((column) => column.id === columnId)?.name ?? 'otra columna'
    );
  }

  /** The card with the given id anywhere on the board. */
  findCard(cardId: string): BoardViewCard | undefined {
    const view = this.view.value();
    if (!view) return undefined;
    const columns = [...view.columns, ...(view.unmappedColumn ? [view.unmappedColumn] : [])];
    for (const column of columns) {
      const found = column.cards.find((card) => card.id === cardId);
      if (found) return found;
    }
    return undefined;
  }

  /**
   * Moves a card to a column (and place). The board changes at once; if the server refuses, the board
   * goes back and the reason is kept in `moveError`, or the person is asked what the server needs.
   */
  move(
    cardId: string,
    columnId: string,
    options: MoveCardOptions & { transitionId?: string } = {},
  ): void {
    const boardId = this.boardIdState();
    const current = this.view.value();
    if (!boardId || !current || !this.canMove()) return;
    const snapshot = current;
    // A read still in flight would bring back the board as it was before this move.
    this.refreshSubscription?.unsubscribe();
    this.refreshingState.set(false);
    this.moveErrorState.set(null);
    this.wipWarningState.set(null);
    this.view.set(moveCardInView(current, cardId, columnId, options));
    const input: BoardMoveInput = {
      columnId,
      beforeIssueId: options.beforeIssueId,
      afterIssueId: options.afterIssueId,
      swimlane: options.swimlaneId || undefined,
      transitionId: options.transitionId,
    };
    this.api.moveCard(boardId, cardId, input).subscribe({
      next: (result) => {
        const card = this.findCard(cardId);
        this.announcementState.set(
          `${card?.key ?? 'Incidencia'} movida a ${this.columnName(columnId)}`,
        );
        if (result.wip?.exceeded) this.wipWarningState.set(MESSAGES.wipOver);
        this.refresh();
      },
      error: (raw: unknown) => {
        this.view.set(snapshot);
        this.handleMoveError(asApiError(raw), cardId, columnId, options);
      },
    });
  }

  /** Repeats a move with the transition the person picked among the candidates. */
  resolveChoice(transitionId: string): void {
    const pending = this.choiceState();
    if (!pending) return;
    this.choiceState.set(null);
    this.move(pending.cardId, pending.columnId, { ...pending.options, transitionId });
  }

  /** Creates an issue at the end of the first column. On a scrum board it joins the sprint in view. */
  createCard(summary: string): Observable<Issue> {
    const view = this.view.value();
    const projectId = this.projectId;
    if (!view || !projectId) return throwError(() => new Error('no board'));
    const sprintId = view.board.type === 'scrum' ? view.sprint?.id : undefined;
    return this.issues.create({ projectId, summary, description: summary }).pipe(
      switchMap((issue) =>
        sprintId
          ? this.backlogApi.rank({ issueIds: [issue.id], sprintId }).pipe(switchMap(() => [issue]))
          : [issue],
      ),
      tap(() => this.refresh()),
      catchError((error: unknown) => throwError(() => asApiError(error))),
    );
  }

  saveSettings(input: UpdateBoardInput): Observable<BoardDetail> {
    return this.write(this.api.update(this.requireBoard(), input));
  }

  saveColumns(columns: readonly BoardColumnInput[]): Observable<BoardDetail> {
    return this.write(this.api.replaceColumns(this.requireBoard(), columns));
  }

  saveQuickFilters(filters: readonly BoardQueryInput[]): Observable<BoardDetail> {
    return this.write(
      this.api.replaceQuickFilters(this.requireBoard(), filters).pipe(
        tap((board) => {
          const valid = new Set(board.quickFilters.map((filter) => filter.id));
          this.filterIdsState.update((ids) => ids.filter((id) => valid.has(id)));
        }),
      ),
    );
  }

  saveSwimlanes(swimlanes: readonly BoardQueryInput[]): Observable<BoardDetail> {
    return this.write(this.api.replaceSwimlanes(this.requireBoard(), swimlanes));
  }

  /** Creates a board and shows it. */
  createBoard(input: CreateBoardInput): Observable<BoardDetail> {
    const projectId = this.projectId;
    if (!projectId) return throwError(() => new Error('no project'));
    return this.api.create(projectId, input).pipe(
      tap((board) => {
        this.boards.load(this.api.list(projectId));
        this.select(board.id);
      }),
      catchError((error: unknown) => throwError(() => asApiError(error))),
    );
  }

  /** Deletes the selected board and goes back to the main one. */
  deleteBoard(): Observable<void> {
    const boardId = this.requireBoard();
    return this.api.delete(boardId).pipe(
      tap(() => {
        if (this.projectId) {
          this.storage.removeItem(STORAGE_PREFIX + this.projectId);
          this.open(this.projectId);
        }
      }),
      catchError((error: unknown) => throwError(() => asApiError(error))),
    );
  }

  reset(): void {
    this.refreshSubscription?.unsubscribe();
    this.boards.reset();
    this.view.reset();
    this.projectId = null;
    this.boardIdState.set(null);
    this.filterIdsState.set([]);
    this.moveErrorState.set(null);
    this.choiceState.set(null);
    this.inputRequestState.set(null);
  }

  private requireBoard(): string {
    const id = this.boardIdState();
    if (!id) throw new Error('no board selected');
    return id;
  }

  private write(source: Observable<BoardDetail>): Observable<BoardDetail> {
    this.busyState.set(true);
    return source.pipe(
      tap(() => {
        this.busyState.set(false);
        this.boards.load(this.api.list(this.projectId ?? ''));
        this.refresh();
      }),
      catchError((error: unknown) => {
        this.busyState.set(false);
        return throwError(() => asApiError(error));
      }),
    );
  }

  private handleMoveError(
    error: ApiError,
    cardId: string,
    columnId: string,
    options: MoveCardOptions,
  ): void {
    if (error.code === 'board.ambiguous_transition') {
      this.choiceState.set({
        cardId,
        columnId,
        options,
        candidates: error.details.map((detail) => ({ id: detail.field, name: detail.message })),
      });
      return;
    }
    if (error.code === 'issue.required_field_missing') {
      const transitionId = error.details[0]?.message;
      if (transitionId) {
        this.inputRequestState.set({ cardId, transitionId });
        return;
      }
    }
    if (error.code === 'issue.status_conflict' || error.code === 'board.rank_conflict') {
      this.refresh();
    }
    this.moveErrorState.set(boardMoveErrorMessage(error));
  }
}
