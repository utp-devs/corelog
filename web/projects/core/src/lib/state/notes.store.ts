import { Injectable, computed, inject, signal } from '@angular/core';
import { Observable, Subscription, catchError, tap, throwError } from 'rxjs';

import { ApiError, CreateNoteInput, Note, NoteTagCount, UpdateNoteInput } from '../models';
import { NoteApiService } from '../services/api/note-api.service';
import { storeError } from '../utils/store-error';
import { resetOnLogout } from './session-scoped';

/** Notes per page of the notes page and size of the scratchpad list. */
export const NOTES_PAGE_SIZE = 24;
export const NOTES_RECENT_SIZE = 5;
/** Time a deleted note can still be restored before the server is asked to delete it. */
export const NOTE_UNDO_WINDOW_MS = 5000;
/** Quiet time after the last edit before the changes are sent. */
export const NOTE_AUTOSAVE_DELAY_MS = 800;

export type NoteSaveState = 'idle' | 'saving' | 'saved' | 'error';

/** What the notes page is showing: a text search, a tag or only pinned notes. */
export interface NotesView {
  readonly query: string;
  readonly tag: string | null;
  readonly pinned: boolean;
}

const EMPTY_VIEW: NotesView = { query: '', tag: null, pinned: false };

/** Pinned notes first, then the most recently updated. */
function sortNotes(notes: readonly Note[]): Note[] {
  return [...notes].sort((a, b) => {
    if (a.pinned !== b.pinned) return a.pinned ? -1 : 1;
    return b.updatedAt.localeCompare(a.updatedAt);
  });
}

function matchesView(note: Note, view: NotesView): boolean {
  if (view.pinned && !note.pinned) return false;
  if (view.tag && !note.tags.includes(view.tag)) return false;
  const query = view.query.trim().toLowerCase();
  if (query && !`${note.title}\n${note.body}`.toLowerCase().includes(query)) return false;
  return true;
}

function upsert(list: readonly Note[], note: Note): Note[] {
  const exists = list.some((item) => item.id === note.id);
  return sortNotes(
    exists ? list.map((item) => (item.id === note.id ? note : item)) : [note, ...list],
  );
}

interface PendingDelete {
  readonly note: Note;
  readonly timer: ReturnType<typeof setTimeout>;
}

/**
 * Private notes of the signed-in user: the list of the notes page (filtered, paged by cursor), the
 * five most recent notes of the scratchpad, the notes of one issue and the tag counts.
 * Pin and delete are optimistic: a failed pin is rolled back and a deleted note can be restored
 * during NOTE_UNDO_WINDOW_MS. Edits of an open note are sent together after a quiet period.
 */
@Injectable({ providedIn: 'root' })
export class NotesStore {
  private readonly api = inject(NoteApiService);

  private readonly itemsState = signal<readonly Note[]>([]);
  private readonly recentState = signal<readonly Note[]>([]);
  private readonly issueItemsState = signal<readonly Note[]>([]);
  private readonly tagsState = signal<readonly NoteTagCount[]>([]);
  private readonly viewState = signal<NotesView>(EMPTY_VIEW);
  private readonly cursorState = signal<string | null>(null);
  private readonly loadingState = signal(false);
  private readonly loadingMoreState = signal(false);
  private readonly loadedState = signal(false);
  private readonly errorState = signal<ApiError | null>(null);
  private readonly saveStateState = signal<NoteSaveState>('idle');

  private listRequest: Subscription | null = null;
  /** Issue whose notes are listed in issueNotes. */
  private issueScope: string | null = null;
  private readonly pendingDeletes = new Map<string, PendingDelete>();
  private readonly pendingEdits = new Map<string, UpdateNoteInput>();
  private readonly editTimers = new Map<string, ReturnType<typeof setTimeout>>();
  private readonly editsInFlight = new Set<string>();

  /** Notes of the current view, pinned first. */
  readonly items = this.itemsState.asReadonly();
  readonly pinned = computed(() => this.itemsState().filter((note) => note.pinned));
  readonly others = computed(() => this.itemsState().filter((note) => !note.pinned));
  readonly recent = this.recentState.asReadonly();
  readonly issueNotes = this.issueItemsState.asReadonly();
  readonly tags = this.tagsState.asReadonly();
  /** Search, tag and pinned filter of the notes page. */
  readonly view = this.viewState.asReadonly();
  readonly hasMore = computed(() => this.cursorState() !== null);
  readonly loading = this.loadingState.asReadonly();
  readonly loadingMore = this.loadingMoreState.asReadonly();
  /** True once the first page of the current view has arrived. */
  readonly loaded = this.loadedState.asReadonly();
  readonly error = this.errorState.asReadonly();
  /** State of the autosave of the note being edited. */
  readonly saveState = this.saveStateState.asReadonly();
  readonly isFiltered = computed(() => {
    const view = this.viewState();
    return view.query.trim() !== '' || view.tag !== null || view.pinned;
  });

  constructor() {
    resetOnLogout(() => this.reset());
  }

  /** Loads the first page of a view and the tag counts. */
  load(view: NotesView = this.viewState()): void {
    this.viewState.set(view);
    this.listRequest?.unsubscribe();
    this.loadingState.set(true);
    this.errorState.set(null);
    this.listRequest = this.api
      .list({
        query: view.query,
        tag: view.tag ?? undefined,
        pinned: view.pinned ? true : undefined,
        limit: NOTES_PAGE_SIZE,
      })
      .subscribe({
        next: (page) => {
          this.itemsState.set(page.items.filter((note) => !this.pendingDeletes.has(note.id)));
          this.cursorState.set(page.nextCursor);
          this.loadedState.set(true);
          this.loadingState.set(false);
        },
        error: (error: unknown) => {
          this.errorState.set(storeError(error, 'No se pudieron cargar tus notas.'));
          this.loadingState.set(false);
        },
      });
    this.refreshTags();
  }

  /** Appends the next page of the current view ("cargar más"). */
  loadMore(): void {
    const cursor = this.cursorState();
    if (cursor === null || this.loadingMoreState()) return;
    const view = this.viewState();
    this.loadingMoreState.set(true);
    this.api
      .list({
        query: view.query,
        tag: view.tag ?? undefined,
        pinned: view.pinned ? true : undefined,
        cursor,
        limit: NOTES_PAGE_SIZE,
      })
      .subscribe({
        next: (page) => {
          this.itemsState.update((items) => {
            const known = new Set(items.map((note) => note.id));
            const fresh = page.items.filter(
              (note) => !known.has(note.id) && !this.pendingDeletes.has(note.id),
            );
            return [...items, ...fresh];
          });
          this.cursorState.set(page.nextCursor);
          this.loadingMoreState.set(false);
        },
        error: (error: unknown) => {
          this.errorState.set(storeError(error, 'No se pudieron cargar más notas.'));
          this.loadingMoreState.set(false);
        },
      });
  }

  /** Reads the latest notes for the scratchpad. */
  loadRecent(): void {
    this.api.list({ limit: NOTES_RECENT_SIZE }).subscribe({
      next: (page) =>
        this.recentState.set(page.items.filter((note) => !this.pendingDeletes.has(note.id))),
      error: (error: unknown) =>
        this.errorState.set(storeError(error, 'No se pudieron cargar tus notas.')),
    });
  }

  /** Reads the notes linked to an issue. */
  loadForIssue(issueId: string): void {
    this.issueScope = issueId;
    this.issueItemsState.set([]);
    this.api.list({ issueId, limit: 50 }).subscribe({
      next: (page) =>
        this.issueItemsState.set(page.items.filter((note) => !this.pendingDeletes.has(note.id))),
      error: (error: unknown) =>
        this.errorState.set(storeError(error, 'No se pudieron cargar tus notas.')),
    });
  }

  refreshTags(): void {
    this.api.tags().subscribe({
      next: (tags) => this.tagsState.set(tags),
      error: () => undefined,
    });
  }

  create(input: CreateNoteInput): Observable<Note> {
    return this.api.create(input).pipe(
      tap((note) => {
        this.errorState.set(null);
        this.place(note);
        if (note.tags.length > 0) this.refreshTags();
      }),
      catchError((error: unknown) => this.fail(error, 'No se pudo guardar la nota.')),
    );
  }

  /** Sends a change at once (color and tags picked from a menu). The note is replaced by the server copy. */
  update(id: string, input: UpdateNoteInput): Observable<Note> {
    return this.api.update(id, input).pipe(
      tap((note) => {
        this.errorState.set(null);
        this.replace(note);
        if (input.tags !== undefined) this.refreshTags();
      }),
      catchError((error: unknown) => this.fail(error, 'No se pudo actualizar la nota.')),
    );
  }

  /** Pins or unpins at once and rolls back when the server refuses. */
  togglePin(note: Note): void {
    const target = !note.pinned;
    const previous = this.find(note.id) ?? note;
    this.replace({ ...previous, pinned: target });
    (target ? this.api.pin(note.id) : this.api.unpin(note.id)).subscribe({
      next: (saved) => {
        this.errorState.set(null);
        this.replace(saved);
      },
      error: (error: unknown) => {
        this.replace(previous);
        this.errorState.set(
          storeError(
            error,
            target ? 'No se pudo fijar la nota.' : 'No se pudo quitar la fijación.',
          ),
        );
      },
    });
  }

  /**
   * Removes a note from every list at once. The server is asked to delete it after
   * NOTE_UNDO_WINDOW_MS, unless `undoRemove` brings it back before.
   */
  remove(note: Note): void {
    const current = this.find(note.id) ?? note;
    this.clearEdit(note.id);
    this.dropFromLists(note.id);
    const timer = setTimeout(() => this.commitDelete(note.id), NOTE_UNDO_WINDOW_MS);
    this.pendingDeletes.set(note.id, { note: current, timer });
  }

  /** Brings back a note removed less than NOTE_UNDO_WINDOW_MS ago. */
  undoRemove(id: string): void {
    const pending = this.pendingDeletes.get(id);
    if (!pending) return;
    clearTimeout(pending.timer);
    this.pendingDeletes.delete(id);
    this.place(pending.note);
  }

  /**
   * Applies an edit to the open note at once and sends it after a quiet period. Edits made in
   * the meantime are merged into one request.
   */
  autosave(id: string, change: UpdateNoteInput): void {
    const merged = { ...this.pendingEdits.get(id), ...change };
    this.pendingEdits.set(id, merged);
    const current = this.find(id);
    if (current) this.applyLocal(current, change);
    this.saveStateState.set('saving');
    clearTimeout(this.editTimers.get(id));
    this.editTimers.set(
      id,
      setTimeout(() => this.sendEdit(id), NOTE_AUTOSAVE_DELAY_MS),
    );
  }

  /** Sends every edit waiting for its quiet period. Used when the editor closes. */
  flush(): void {
    for (const id of [...this.editTimers.keys()]) {
      clearTimeout(this.editTimers.get(id));
      this.sendEdit(id);
    }
  }

  /** Forgets the save indicator (a new note is opened). */
  resetSaveState(): void {
    if (this.editTimers.size === 0 && this.editsInFlight.size === 0)
      this.saveStateState.set('idle');
  }

  clearError(): void {
    this.errorState.set(null);
  }

  reset(): void {
    this.listRequest?.unsubscribe();
    this.listRequest = null;
    for (const pending of this.pendingDeletes.values()) clearTimeout(pending.timer);
    this.pendingDeletes.clear();
    for (const timer of this.editTimers.values()) clearTimeout(timer);
    this.editTimers.clear();
    this.pendingEdits.clear();
    this.editsInFlight.clear();
    this.itemsState.set([]);
    this.recentState.set([]);
    this.issueItemsState.set([]);
    this.issueScope = null;
    this.tagsState.set([]);
    this.viewState.set(EMPTY_VIEW);
    this.cursorState.set(null);
    this.loadingState.set(false);
    this.loadingMoreState.set(false);
    this.loadedState.set(false);
    this.errorState.set(null);
    this.saveStateState.set('idle');
  }

  private find(id: string): Note | undefined {
    return (
      this.itemsState().find((note) => note.id === id) ??
      this.recentState().find((note) => note.id === id) ??
      this.issueItemsState().find((note) => note.id === id)
    );
  }

  private fail(error: unknown, fallback: string): Observable<never> {
    const apiError = storeError(error, fallback);
    this.errorState.set(apiError);
    return throwError(() => apiError);
  }

  /** Puts a note in the lists it belongs to (the page list only when it matches the view). */
  private place(note: Note): void {
    if (matchesView(note, this.viewState())) {
      this.itemsState.update((items) => upsert(items, note));
    }
    this.recentState.update((items) => upsert(items, note).slice(0, NOTES_RECENT_SIZE));
    if (note.issueId && note.issueId === this.issueScope) {
      this.issueItemsState.update((items) => upsert(items, note));
    }
  }

  /** Replaces a note where it already is, keeping the order rules. */
  private replace(note: Note): void {
    const swap = (list: readonly Note[]): readonly Note[] =>
      list.some((item) => item.id === note.id) ? upsert(list, note) : list;
    this.itemsState.update(swap);
    this.recentState.update(swap);
    this.issueItemsState.update(swap);
  }

  private dropFromLists(id: string): void {
    const without = (list: readonly Note[]): readonly Note[] => list.filter((n) => n.id !== id);
    this.itemsState.update(without);
    this.recentState.update(without);
    this.issueItemsState.update(without);
  }

  private applyLocal(note: Note, change: UpdateNoteInput): void {
    const next: Note = {
      ...note,
      title: change.title ?? note.title,
      body: change.body ?? note.body,
      color: change.color ?? note.color,
      tags: change.tags ?? note.tags,
      issueId: change.issueId === undefined ? note.issueId : change.issueId,
      projectId: change.projectId === undefined ? note.projectId : change.projectId,
    };
    const swap = (list: readonly Note[]): readonly Note[] =>
      list.map((item) => (item.id === next.id ? next : item));
    this.itemsState.update(swap);
    this.recentState.update(swap);
    this.issueItemsState.update(swap);
  }

  private clearEdit(id: string): void {
    clearTimeout(this.editTimers.get(id));
    this.editTimers.delete(id);
    this.pendingEdits.delete(id);
  }

  private sendEdit(id: string): void {
    const change = this.pendingEdits.get(id);
    this.editTimers.delete(id);
    if (!change || this.editsInFlight.has(id)) {
      // A request for this note is still running: send again when it ends.
      if (change) {
        this.editTimers.set(
          id,
          setTimeout(() => this.sendEdit(id), NOTE_AUTOSAVE_DELAY_MS),
        );
      }
      return;
    }
    this.pendingEdits.delete(id);
    this.editsInFlight.add(id);
    this.saveStateState.set('saving');
    this.api.update(id, change).subscribe({
      next: (saved) => {
        this.editsInFlight.delete(id);
        const waiting = this.pendingEdits.get(id);
        // Fields typed while the request ran stay as typed.
        this.replace(waiting ? { ...saved, ...waiting } : saved);
        if (change.tags !== undefined) this.refreshTags();
        this.errorState.set(null);
        if (this.editTimers.size === 0 && this.pendingEdits.size === 0) {
          this.saveStateState.set('saved');
        }
      },
      error: (error: unknown) => {
        this.editsInFlight.delete(id);
        // Keep the unsent edit so the next change or a retry sends it again.
        this.pendingEdits.set(id, { ...change, ...this.pendingEdits.get(id) });
        this.saveStateState.set('error');
        this.errorState.set(storeError(error, 'No se pudo guardar la nota.'));
      },
    });
  }

  private commitDelete(id: string): void {
    const pending = this.pendingDeletes.get(id);
    if (!pending) return;
    this.api.delete(id).subscribe({
      next: () => {
        this.pendingDeletes.delete(id);
        this.refreshTags();
      },
      error: (error: unknown) => {
        this.pendingDeletes.delete(id);
        this.place(pending.note);
        this.errorState.set(storeError(error, 'No se pudo eliminar la nota.'));
      },
    });
  }
}
