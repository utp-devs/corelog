import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  ElementRef,
  Injector,
  afterNextRender,
  computed,
  effect,
  inject,
  input,
  model,
  output,
  signal,
  untracked,
  viewChild,
} from '@angular/core';
import {
  PALETTE_SEARCH_DEBOUNCE_MS,
  SEARCH_MIN_CHARS,
  getRecent,
  type RecentItem,
  type SearchHit,
} from '@corelog/core';

import { MarkText } from '@corelog/ui/mark-text';

import { CommandRegistry, type Command, type CommandGroup } from './command-registry';
import { groupHits, highlightSegments, matchesQuery, type HitGroup } from './groups';

/** Remote search for projects and issues. A rejection shows the error state. */
export type PaletteSearch = (query: string) => Promise<SearchHit[]>;

export type PaletteSection = CommandGroup | 'Recientes' | HitGroup | 'Búsqueda';

export interface PaletteOption {
  readonly key: string;
  readonly section: PaletteSection;
  readonly title: string;
  /** Issue or project key shown as a chip (the hit subtitle). */
  readonly subtitle?: string;
  /** Title with the matches between mark tags (escaped HTML); shown through the sanitizing text component. */
  readonly highlight?: string;
  /** Runs after the palette has closed. */
  readonly run: () => void;
}

interface RemoteState {
  /** Trimmed query the result belongs to. */
  readonly term: string;
  readonly status: 'ready' | 'error';
  readonly hits: readonly SearchHit[];
}

const SECTION_ORDER: readonly PaletteSection[] = [
  'Acciones',
  'Navegación',
  'Recientes',
  'Proyectos',
  'Incidencias',
  'Personas',
  'Equipos',
  'Filtros',
  'Notas',
  'Búsqueda',
];

/** Stroke paths (24 px grid) for the section icons. */
const SECTION_ICON: Record<PaletteSection, string> = {
  Acciones: 'M13 2 4 14h7l-1 8 9-12h-7z',
  Navegación: 'M5 12h14M13 6l6 6-6 6',
  Recientes: 'M12 2a10 10 0 1 0 0 20 10 10 0 0 0 0-20zm0 5v5l3 2',
  Proyectos: 'M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2zM3 10h18',
  Incidencias: 'M14 3H6a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9zM14 3v6h6M8 13h8M8 17h8',
  Personas:
    'M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2M9 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM22 21v-2a4 4 0 0 0-3-3.9M16 3.1a4 4 0 0 1 0 7.8',
  Equipos:
    'M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2M9 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM23 21v-2a4 4 0 0 0-3-3.9',
  Filtros: 'M22 3H2l8 9.5V19l4 2v-8.5z',
  Notas: 'M4 4h12l4 4v12H4zM8 12h8M8 16h5',
  Búsqueda: 'M11 4a7 7 0 1 0 0 14 7 7 0 0 0 0-14zM20 20l-4-4',
};

let nextPaletteId = 0;

/**
 * Modal command palette. The overlay exists only while `open` is true, so the input starts from
 * `seed` each time it opens. Focus moves into the input on open, stays inside the dialog while it
 * is open, and goes back to the previously focused element on close.
 */
@Component({
  selector: 'app-command-palette',
  imports: [MarkText],
  templateUrl: './command-palette.html',
  styleUrl: './command-palette.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: {
    '(document:focusin)': 'onDocumentFocusIn($event)',
  },
})
export class CommandPalette {
  private readonly registry = inject(CommandRegistry);
  private readonly injector = inject(Injector);
  private readonly inputRef = viewChild<ElementRef<HTMLInputElement>>('searchInput');
  private readonly dialogRef = viewChild<ElementRef<HTMLElement>>('dialog');

  /** Whether the palette is shown. The palette sets it to false when it closes itself. */
  readonly open = model(false);
  /** Text to prefill the input with when the palette opens. */
  readonly seed = input('');
  /** Remote search for projects and issues. */
  readonly search = input.required<PaletteSearch>();
  /** Emits the search hit or recent item the user picked. The palette closes first. */
  readonly openHit = output<SearchHit>();
  /** Emits the typed text when the user asks for every result in the navigator. */
  readonly openAll = output<string>();

  protected readonly listId = `command-palette-list-${nextPaletteId++}`;
  protected readonly sectionIcon = SECTION_ICON;
  protected readonly minChars = SEARCH_MIN_CHARS;

  protected readonly query = signal('');
  protected readonly active = signal(0);
  private readonly remote = signal<RemoteState>({ term: '', status: 'ready', hits: [] });
  protected readonly retrying = signal(false);
  private readonly recent = signal<readonly RecentItem[]>([]);

  private readonly term = computed(() => this.query().trim());
  protected readonly searchable = computed(() => this.term().length >= SEARCH_MIN_CHARS);
  private readonly stale = computed(() => this.searchable() && this.remote().term !== this.term());
  protected readonly loading = computed(
    () => this.searchable() && (this.stale() || this.retrying()),
  );
  protected readonly hasError = computed(
    () =>
      this.searchable() && !this.stale() && !this.retrying() && this.remote().status === 'error',
  );
  private readonly remoteHits = computed<readonly SearchHit[]>(() =>
    this.searchable() && !this.stale() && this.remote().status === 'ready'
      ? this.remote().hits
      : [],
  );

  private readonly options = computed<PaletteOption[]>(() => {
    const term = this.term();
    const options: PaletteOption[] = [];
    for (const command of this.registry.commands()) {
      if (matchesQuery([command.title, ...(command.keywords ?? [])].join(' '), term)) {
        options.push(this.commandOption(command));
      }
    }
    for (const item of this.recent()) {
      if (matchesQuery(`${item.title} ${item.subtitle}`, term)) {
        options.push(this.hitOption('Recientes', item));
      }
    }
    for (const section of groupHits(this.remoteHits())) {
      for (const hit of section.items) {
        options.push(this.hitOption(section.name, hit));
      }
    }
    if (this.searchable()) {
      options.push({
        key: 'search-all',
        section: 'Búsqueda',
        title: `Ver todos los resultados de «${term}»`,
        run: () => this.openAll.emit(term),
      });
    }
    // Index N must match the Nth option in the DOM, so options are ordered by section here.
    return options.sort(
      (a, b) => SECTION_ORDER.indexOf(a.section) - SECTION_ORDER.indexOf(b.section),
    );
  });

  protected readonly hasOptions = computed(() => this.options().length > 0);
  /** Typed text shown in the empty state. */
  protected readonly typedTerm = this.term;
  protected readonly showEmpty = computed(
    () =>
      this.searchable() &&
      !this.loading() &&
      !this.hasError() &&
      this.options().every((option) => option.section === 'Búsqueda'),
  );
  private readonly safeActive = computed(() =>
    Math.min(this.active(), Math.max(this.options().length - 1, 0)),
  );
  private readonly activeOption = computed(() => this.options().at(this.safeActive()));
  protected readonly activeId = computed(() =>
    this.activeOption() ? this.optionId(this.safeActive()) : null,
  );
  protected readonly sections = computed(() => {
    const options = this.options();
    const safeActive = this.safeActive();
    const term = this.term();
    return SECTION_ORDER.map((name) => ({
      name,
      items: options
        .map((option, index) => ({
          option,
          index,
          selected: index === safeActive,
          segments: highlightSegments(option.title, term),
        }))
        .filter(({ option }) => option.section === name),
    })).filter((section) => section.items.length > 0);
  });

  private searchTimer: ReturnType<typeof setTimeout> | undefined;
  private searchToken = 0;
  private wasOpen = false;
  private previousFocus: HTMLElement | null = null;

  constructor() {
    effect(() => {
      const isOpen = this.open();
      untracked(() => {
        if (isOpen) {
          this.onOpened();
        } else {
          this.onClosed();
        }
      });
    });
    inject(DestroyRef).onDestroy(() => this.onClosed());
  }

  protected onInput(value: string): void {
    const previousTerm = this.term();
    this.query.set(value);
    this.active.set(0);
    if (this.term() !== previousTerm) {
      this.scheduleSearch();
    }
  }

  protected retry(): void {
    this.retrying.set(true);
    this.scheduleSearch();
  }

  protected select(option: PaletteOption): void {
    this.close();
    option.run();
  }

  protected onOverlayMouseDown(event: MouseEvent): void {
    if (event.target === event.currentTarget) {
      this.close();
    }
  }

  protected onKeydown(event: KeyboardEvent): void {
    switch (event.key) {
      case 'Escape':
        event.preventDefault();
        this.close();
        return;
      case 'ArrowDown':
      case 'ArrowUp': {
        event.preventDefault();
        const count = this.options().length;
        if (count === 0) {
          return;
        }
        const step = event.key === 'ArrowDown' ? 1 : -1;
        this.active.set((this.safeActive() + step + count) % count);
        return;
      }
      case 'Enter': {
        event.preventDefault();
        const option = this.activeOption();
        if (option) {
          this.select(option);
        }
        return;
      }
      case 'Tab':
        this.trapTab(event);
        return;
      default:
        return;
    }
  }

  /** Keeps focus inside the dialog while it is open, including programmatic focus from elsewhere. */
  protected onDocumentFocusIn(event: FocusEvent): void {
    if (!this.open()) {
      return;
    }
    const dialog = this.dialogRef()?.nativeElement;
    const input = this.inputRef()?.nativeElement;
    if (dialog && input && event.target instanceof Node && !dialog.contains(event.target)) {
      input.focus();
    }
  }

  protected optionId(index: number): string {
    return `${this.listId}-option-${index}`;
  }

  private close(): void {
    this.open.set(false);
  }

  private onOpened(): void {
    this.wasOpen = true;
    const active = document.activeElement;
    this.previousFocus = active instanceof HTMLElement ? active : null;
    this.query.set(this.seed());
    this.active.set(0);
    this.remote.set({ term: '', status: 'ready', hits: [] });
    this.retrying.set(false);
    this.recent.set(getRecent());
    this.scheduleSearch();
    afterNextRender(() => this.inputRef()?.nativeElement.focus(), { injector: this.injector });
  }

  private onClosed(): void {
    this.cancelSearch();
    if (!this.wasOpen) {
      return;
    }
    this.wasOpen = false;
    const previous = this.previousFocus;
    this.previousFocus = null;
    previous?.focus();
  }

  /**
   * Debounced remote search. Every schedule or cancel invalidates the previous request, so only
   * the latest query can write its result.
   */
  private scheduleSearch(): void {
    this.cancelSearch();
    const term = this.term();
    if (term.length < SEARCH_MIN_CHARS) {
      return;
    }
    const token = this.searchToken;
    this.searchTimer = setTimeout(() => {
      this.searchTimer = undefined;
      new Promise<SearchHit[]>((resolve) => resolve(this.search()(term)))
        .then((hits) => {
          if (token === this.searchToken) {
            this.remote.set({ term, status: 'ready', hits });
          }
        })
        .catch(() => {
          if (token === this.searchToken) {
            this.remote.set({ term, status: 'error', hits: [] });
          }
        })
        .finally(() => {
          if (token === this.searchToken) {
            this.retrying.set(false);
          }
        });
    }, PALETTE_SEARCH_DEBOUNCE_MS);
  }

  private cancelSearch(): void {
    if (this.searchTimer !== undefined) {
      clearTimeout(this.searchTimer);
      this.searchTimer = undefined;
    }
    this.searchToken += 1;
  }

  /** The input and the retry button are the only focusable elements, so Tab cycles between them. */
  private trapTab(event: KeyboardEvent): void {
    const dialog = this.dialogRef()?.nativeElement;
    if (!dialog) {
      return;
    }
    const focusable = Array.from(dialog.querySelectorAll<HTMLElement>('input, button'));
    if (focusable.length === 0) {
      return;
    }
    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  }

  private commandOption(command: Command): PaletteOption {
    return {
      key: `command-${command.id}`,
      section: command.group,
      title: command.title,
      run: () => command.run(),
    };
  }

  private hitOption(section: PaletteSection, hit: SearchHit): PaletteOption {
    return {
      key: `${section}-${hit.kind}-${hit.id}`,
      section,
      title: hit.title,
      subtitle: hit.key ?? hit.subtitle,
      highlight: section === 'Recientes' ? undefined : hit.highlight,
      run: () => this.openHit.emit(hit),
    };
  }
}
