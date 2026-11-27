import { Injectable, computed, inject, signal } from '@angular/core';

import type { BoardCard, BoardColumn, BoardFilter, ProjectBoard } from '../models/board.model';
import { BoardApiService } from '../services/api/board-api.service';
import { asApiError } from '../utils/api-error';
import { isColumnFull, parseWipLimit } from '../utils/board-rules';
import { AsyncState } from './async-state';
import { resetOnLogout } from './session-scoped';

const EMPTY_FILTER: BoardFilter = { search: '', priority: null, onlyMine: false };

const MESSAGE_MOVE_FORBIDDEN = 'No tienes permiso para mover incidencias en este proyecto.';
const MESSAGE_MOVE_CONFLICT =
  'La columna destino alcanzó su límite WIP o el flujo no permite ese cambio.';
const MESSAGE_MOVE_FAILED = 'No se pudo mover la incidencia. Intenta de nuevo.';
const MESSAGE_WIP_FORBIDDEN =
  'Solo el lead del proyecto o el administrador pueden cambiar el límite WIP.';
const MESSAGE_WIP_FAILED = 'No se pudo guardar el límite WIP. Intenta de nuevo.';
const MESSAGE_WIP_INVALID = 'El límite WIP debe ser un número entero mayor que 0.';

/** A column offered as destination of a move, with whether it can take more cards. */
export interface BoardMoveTarget {
  readonly id: string;
  readonly name: string;
  readonly disabled: boolean;
}

/**
 * Project board: loads the default board, filters its cards locally, moves cards between columns
 * and edits WIP limits. Moves and limit changes wait for the server answer; the board is replaced
 * with the response, so there is no optimistic state to roll back.
 */
@Injectable({ providedIn: 'root' })
export class BoardStore {
  private readonly api = inject(BoardApiService);
  private projectId: string | null = null;
  private readonly currentUserId = signal<string | undefined>(undefined);
  private readonly filterState = signal<BoardFilter>(EMPTY_FILTER);
  private readonly moveErrorState = signal<string | null>(null);
  private readonly announcementState = signal('');

  readonly board = new AsyncState<ProjectBoard>();
  readonly filter = this.filterState.asReadonly();
  readonly moveError = this.moveErrorState.asReadonly();
  /** Text for the polite live region after a successful move. */
  readonly announcement = this.announcementState.asReadonly();

  /** Columns with the cards that pass the current filter. Unchanged data when no filter is set. */
  readonly visibleColumns = computed<readonly BoardColumn[]>(() => {
    const board = this.board.value();
    if (!board) {
      return [];
    }
    return board.columns.map((column) => ({
      ...column,
      cards: column.cards.filter((card) => this.matchesFilter(card)),
    }));
  });

  readonly hasColumns = computed(() => (this.board.value()?.columns.length ?? 0) > 0);

  constructor() {
    resetOnLogout(() => this.reset());
  }

  /** Loads the default board of a project. Starting a new load cancels the previous one. */
  load(projectId: string): void {
    this.projectId = projectId;
    this.moveErrorState.set(null);
    this.board.load(this.api.getProjectBoard(projectId));
  }

  retry(): void {
    if (this.projectId) {
      this.load(this.projectId);
    }
  }

  /** Identity used by the "only mine" filter. */
  setCurrentUserId(userId: string | undefined): void {
    this.currentUserId.set(userId);
  }

  setFilter(patch: Partial<BoardFilter>): void {
    this.filterState.update((current) => ({ ...current, ...patch }));
  }

  clearMoveError(): void {
    this.moveErrorState.set(null);
  }

  /** Columns a card can be moved to from the keyboard selector: every column but its own. */
  moveTargetsFor(cardId: string): BoardMoveTarget[] {
    const board = this.board.value();
    const source = this.columnOf(cardId);
    if (!board || !source) {
      return [];
    }
    return board.columns
      .filter((column) => column.id !== source.id)
      .map((column) => ({
        id: column.id,
        name: column.name,
        disabled: isColumnFull(column),
      }));
  }

  /**
   * Moves a card to another column. Nothing happens when the card is unknown or already in the
   * target column. On success the board is replaced by the response; on failure the message is
   * stored in `moveError`.
   */
  move(cardId: string, targetColumnId: string): void {
    const projectId = this.projectId;
    const source = this.columnOf(cardId);
    if (!projectId || !source || source.id === targetColumnId) {
      return;
    }
    const target = this.board.value()?.columns.find((column) => column.id === targetColumnId);
    this.moveErrorState.set(null);
    this.api.moveCard(projectId, cardId, targetColumnId).subscribe({
      next: (updated) => {
        if (projectId !== this.projectId) {
          return;
        }
        this.board.set(updated);
        this.announcementState.set(`Incidencia movida a ${target?.name ?? 'otra columna'}`);
      },
      error: (error: unknown) => this.moveErrorState.set(moveErrorMessage(asApiError(error))),
    });
  }

  /**
   * Sets the WIP limit of a column from the typed text. An empty text removes the limit; an
   * unchanged value makes no request.
   */
  saveWipLimit(columnId: string, raw: string): void {
    const projectId = this.projectId;
    const board = this.board.value();
    if (!projectId || !board) {
      return;
    }
    const parsed = parseWipLimit(raw);
    if (!parsed.valid) {
      this.moveErrorState.set(MESSAGE_WIP_INVALID);
      return;
    }
    const column = board.columns.find((candidate) => candidate.id === columnId);
    if (!column || (parsed.limit ?? undefined) === column.wipLimit) {
      return;
    }
    this.moveErrorState.set(null);
    this.api.updateColumnWip(projectId, columnId, parsed.limit).subscribe({
      next: (updated) => {
        if (projectId === this.projectId) {
          this.board.set(updated);
        }
      },
      error: (error: unknown) => this.moveErrorState.set(wipErrorMessage(asApiError(error))),
    });
  }

  reset(): void {
    this.projectId = null;
    this.board.reset();
    this.filterState.set(EMPTY_FILTER);
    this.moveErrorState.set(null);
    this.announcementState.set('');
  }

  private columnOf(cardId: string): BoardColumn | undefined {
    return this.board
      .value()
      ?.columns.find((column) => column.cards.some((card) => card.id === cardId));
  }

  private matchesFilter(card: BoardCard): boolean {
    const filter = this.filterState();
    if (filter.onlyMine && card.assigneeId !== this.currentUserId()) {
      return false;
    }
    if (filter.priority !== null && card.priority !== filter.priority) {
      return false;
    }
    const term = filter.search.trim().toLowerCase();
    return term === '' || card.summary.toLowerCase().includes(term);
  }
}

function moveErrorMessage(error: { readonly status: number }): string {
  if (error.status === 403) {
    return MESSAGE_MOVE_FORBIDDEN;
  }
  if (error.status === 409) {
    return MESSAGE_MOVE_CONFLICT;
  }
  return MESSAGE_MOVE_FAILED;
}

function wipErrorMessage(error: { readonly status: number }): string {
  return error.status === 403 ? MESSAGE_WIP_FORBIDDEN : MESSAGE_WIP_FAILED;
}
