import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  computed,
  effect,
  inject,
  signal,
  untracked,
  viewChild,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { ActivatedRoute, Router } from '@angular/router';
import {
  NoteApiService,
  NotesStore,
  NotificationService,
  type CreateNoteInput,
  type Note,
  type NoteColor,
} from '@corelog/core';
import { EmptyState } from '@corelog/ui/empty-state';
import { ErrorState } from '@corelog/ui/error-state';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader } from '@corelog/ui/page-header';
import { RichTextView } from '@corelog/ui/rich-text-view';
import { SearchInput } from '@corelog/ui/search-input';
import { Button } from 'primeng/button';

import { NoteCard } from '../note-card/note-card';
import { NoteComposer } from '../note-composer/note-composer';
import { NoteEditor } from '../note-editor/note-editor';

/** How long the "Nota eliminada" bar offers to undo. */
export const UNDO_BAR_MS = 5000;
const SKELETON_CARDS = 6;

interface ExampleNote {
  readonly title: string;
  readonly body: string;
  readonly color: NoteColor;
  readonly tags: readonly string[];
}

/** Notes offered to a user who has none yet. */
export const EXAMPLE_NOTES: readonly ExampleNote[] = [
  {
    title: 'Ideas para la próxima reunión',
    body: '- Revisar las incidencias abiertas\n- Preparar la demo\n- Pedir feedback',
    color: 'yellow',
    tags: ['reuniones'],
  },
  {
    title: 'Pendientes de la semana',
    body: '- Responder a soporte\n- Actualizar el tablero\n- Cerrar el sprint',
    color: 'green',
    tags: ['pendientes'],
  },
  {
    title: 'Enlaces útiles',
    body: 'Guarda aquí **enlaces**, comandos o fragmentos que no quieres perder.',
    color: 'blue',
    tags: ['recursos'],
  },
];

/**
 * Personal notes (S-12): tag sidebar, composer, search, pinned section and a masonry-like grid of
 * cards, plus the editor in a side panel. Notes are private: the API only returns the user's own.
 */
@Component({
  selector: 'app-notes-page',
  imports: [
    Button,
    EmptyState,
    ErrorState,
    LoadingSkeleton,
    NoteCard,
    NoteComposer,
    NoteEditor,
    PageContainer,
    PageHeader,
    RichTextView,
    SearchInput,
  ],
  templateUrl: './notes-page.html',
  styleUrl: './notes-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class NotesPage {
  private readonly store = inject(NotesStore);
  private readonly api = inject(NoteApiService);
  private readonly notify = inject(NotificationService);
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly destroyRef = inject(DestroyRef);
  private readonly composer = viewChild(NoteComposer);

  protected readonly pinned = this.store.pinned;
  protected readonly others = this.store.others;
  protected readonly tags = this.store.tags;
  protected readonly loadingMore = this.store.loadingMore;
  protected readonly hasMore = this.store.hasMore;
  protected readonly filtered = this.store.isFiltered;
  protected readonly tagNames = computed(() => this.tags().map((entry) => entry.tag));
  protected readonly examples = EXAMPLE_NOTES;
  protected readonly skeletonCards = Array.from({ length: SKELETON_CARDS }, (_, i) => i);

  protected readonly query = signal('');
  protected readonly tag = signal<string | null>(null);
  protected readonly pinnedOnly = signal(false);
  protected readonly editing = signal<Note | null>(null);
  protected readonly undoable = signal<Note | null>(null);

  private undoTimer: ReturnType<typeof setTimeout> | undefined;

  protected readonly loading = computed(() => !this.store.loaded() && this.store.loading());
  protected readonly failed = computed(
    () => this.store.error() !== null && !this.store.loaded() && !this.store.loading(),
  );
  protected readonly errorMessage = computed(
    () => this.store.error()?.message ?? 'No se pudieron cargar tus notas.',
  );
  protected readonly ready = computed(() => this.store.loaded() || this.failed());
  protected readonly isEmpty = computed(
    () => this.store.loaded() && this.store.items().length === 0,
  );
  /** A user with no notes at all (not just an empty filter) sees the examples. */
  protected readonly showExamples = computed(
    () => this.isEmpty() && !this.filtered() && this.tags().length === 0,
  );
  protected readonly showOthersHeading = computed(
    () => this.pinned().length > 0 && this.others().length > 0,
  );

  constructor() {
    effect(() => {
      const view = { query: this.query(), tag: this.tag(), pinned: this.pinnedOnly() };
      untracked(() => this.store.load(view));
    });

    // Errors after the page loaded (pin, delete, save) are shown as a toast.
    effect(() => {
      const error = this.store.error();
      if (error && (this.store.loaded() || this.store.items().length > 0)) {
        untracked(() => {
          this.notify.error(error);
          this.store.clearError();
        });
      }
    });

    this.route.queryParamMap.pipe(takeUntilDestroyed()).subscribe((params) => {
      const id = params.get('note');
      if (id) this.openFromLink(id);
    });

    this.destroyRef.onDestroy(() => {
      clearTimeout(this.undoTimer);
      this.store.flush();
    });
  }

  protected showAll(): void {
    this.tag.set(null);
    this.pinnedOnly.set(false);
  }

  protected showPinned(): void {
    this.tag.set(null);
    this.pinnedOnly.set(true);
  }

  protected showTag(tag: string): void {
    this.pinnedOnly.set(false);
    this.tag.set(this.tag() === tag ? null : tag);
  }

  protected clearFilters(): void {
    this.query.set('');
    this.showAll();
  }

  protected load(): void {
    this.store.load();
  }

  protected loadMore(): void {
    this.store.loadMore();
  }

  protected open(note: Note): void {
    this.editing.set(note);
  }

  protected closeEditor(): void {
    this.editing.set(null);
  }

  protected togglePin(note: Note): void {
    this.store.togglePin(note);
  }

  protected changeColor(change: { note: Note; color: NoteColor }): void {
    this.store.update(change.note.id, { color: change.color }).subscribe({
      error: (error: unknown) => this.notify.error(error),
    });
  }

  /** Removes the note at once and offers to bring it back for a few seconds. */
  protected remove(note: Note): void {
    if (this.editing()?.id === note.id) this.editing.set(null);
    this.store.remove(note);
    this.undoable.set(note);
    clearTimeout(this.undoTimer);
    this.undoTimer = setTimeout(() => this.undoable.set(null), UNDO_BAR_MS);
  }

  protected undo(): void {
    const note = this.undoable();
    if (!note) return;
    clearTimeout(this.undoTimer);
    this.undoable.set(null);
    this.store.undoRemove(note.id);
  }

  protected startNote(): void {
    this.composer()?.focusComposer();
  }

  protected useExample(example: ExampleNote): void {
    const input: CreateNoteInput = {
      title: example.title,
      body: example.body,
      color: example.color,
      tags: [...example.tags],
    };
    this.store.create(input).subscribe({ error: (error: unknown) => this.notify.error(error) });
  }

  private openFromLink(id: string): void {
    const known = this.store.items().find((note) => note.id === id);
    const done = () =>
      void this.router.navigate([], {
        relativeTo: this.route,
        queryParams: { note: null },
        queryParamsHandling: 'merge',
        replaceUrl: true,
      });
    if (known) {
      this.editing.set(known);
      done();
      return;
    }
    this.api.get(id).subscribe({
      next: (note) => {
        this.editing.set(note);
        done();
      },
      error: (error: unknown) => {
        this.notify.error(error);
        done();
      },
    });
  }
}
