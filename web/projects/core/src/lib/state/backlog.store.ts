import { Injectable, computed, inject, signal } from '@angular/core';

import type { BacklogItem } from '../models/backlog.model';
import { BacklogApiService } from '../services/api/backlog-api.service';
import { asApiError } from '../utils/api-error';
import { AsyncState } from './async-state';
import { resetOnLogout } from './session-scoped';

const MESSAGE_FORBIDDEN = 'No tienes permiso para ordenar el backlog de este proyecto.';
const MESSAGE_INVALID_POSITION = 'La posición de la incidencia no es válida.';
const MESSAGE_FAILED = 'No se pudo reordenar el backlog. Intenta de nuevo.';

/** Project backlog in rank order, with reordering by position. */
@Injectable({ providedIn: 'root' })
export class BacklogStore {
  private readonly api = inject(BacklogApiService);
  private projectId: string | null = null;
  private readonly moveErrorState = signal<string | null>(null);

  readonly items = new AsyncState<BacklogItem[]>();
  readonly moveError = this.moveErrorState.asReadonly();

  /** Items in rank order. The first item is the one worked first. */
  readonly ordered = computed<readonly BacklogItem[]>(() => this.items.value() ?? []);
  readonly isEmpty = computed(() => this.items.hasValue() && this.ordered().length === 0);

  constructor() {
    resetOnLogout(() => this.reset());
  }

  load(projectId: string): void {
    this.projectId = projectId;
    this.moveErrorState.set(null);
    this.items.load(this.api.list(projectId));
  }

  retry(): void {
    if (this.projectId) {
      this.load(this.projectId);
    }
  }

  /** The "move up" control is disabled on the first row. */
  canMoveUp(index: number): boolean {
    return index > 0;
  }

  /** The "move down" control is disabled on the last row and on unknown rows. */
  canMoveDown(index: number): boolean {
    return index >= 0 && index < this.ordered().length - 1;
  }

  /**
   * Places the item at `index` at `target`. Out-of-range requests are ignored. The response is the
   * renumbered backlog, which replaces the list.
   */
  move(index: number, target: number): void {
    const projectId = this.projectId;
    const item = this.ordered()[index];
    if (!projectId || !item || target < 0 || target >= this.ordered().length) {
      return;
    }
    this.moveErrorState.set(null);
    this.api.move(projectId, item.id, target).subscribe({
      next: (updated) => {
        if (projectId === this.projectId) {
          this.items.set(updated);
        }
      },
      error: (error: unknown) => this.moveErrorState.set(backlogErrorMessage(asApiError(error))),
    });
  }

  clearMoveError(): void {
    this.moveErrorState.set(null);
  }

  reset(): void {
    this.projectId = null;
    this.moveErrorState.set(null);
    this.items.reset();
  }
}

function backlogErrorMessage(error: { readonly status: number }): string {
  if (error.status === 403) {
    return MESSAGE_FORBIDDEN;
  }
  if (error.status === 422) {
    return MESSAGE_INVALID_POSITION;
  }
  return MESSAGE_FAILED;
}
