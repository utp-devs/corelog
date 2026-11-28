import {
  CdkConnectedOverlay,
  CdkOverlayOrigin,
  Overlay,
  type ConnectedPosition,
} from '@angular/cdk/overlay';
import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  ElementRef,
  HostListener,
  computed,
  inject,
  input,
  signal,
} from '@angular/core';
import { Router } from '@angular/router';
import {
  HotkeysService,
  SEARCH_MIN_CHARS,
  SearchApiService,
  SearchStore,
  ThemeService,
  quoteValue,
  type SearchHit,
} from '@corelog/core';
import { CommandPalette, CommandRegistry, type PaletteSearch } from '@corelog/ui/command-palette';
import { MarkText } from '@corelog/ui/mark-text';
import { AppTooltip } from '@corelog/ui/tooltip';
import { firstValueFrom } from 'rxjs';

import { ShellIcon } from './shell-icon';

/** Pause after the last keystroke before the header search queries the server. */
export const HEADER_SEARCH_DEBOUNCE_MS = 300;

/** Gap in px between the field and its results. */
const PANEL_GAP = 4;

/** Results below the field lined up with its right edge, or above it when there is no room below. */
const PANEL_POSITIONS: ConnectedPosition[] = [
  { originX: 'end', originY: 'bottom', overlayX: 'end', overlayY: 'top', offsetY: PANEL_GAP },
  { originX: 'end', originY: 'top', overlayX: 'end', overlayY: 'bottom', offsetY: -PANEL_GAP },
];

interface HitSection {
  readonly name: string;
  readonly items: readonly SearchHit[];
}

const SECTIONS: ReadonlyArray<{ kind: SearchHit['kind']; name: string }> = [
  { kind: 'issue', name: 'Incidencias' },
  { kind: 'project', name: 'Proyectos' },
  { kind: 'person', name: 'Personas' },
  { kind: 'team', name: 'Equipos' },
  { kind: 'filter', name: 'Filtros' },
  { kind: 'note', name: 'Notas' },
];

/** Splits hits into one section per kind, keeping the server order inside each. */
function sectionsOf(hits: readonly SearchHit[]): HitSection[] {
  return SECTIONS.map(({ kind, name }) => ({
    name,
    items: hits.filter((hit) => hit.kind === kind),
  })).filter((section) => section.items.length > 0);
}

/**
 * Header search field. Typing shows grouped project and issue hits in a panel; Ctrl+K or '/' (or the
 * panel footer) opens the full command palette with the same search. Also registers the navigation
 * commands of the palette. On narrow layouts the field is an icon that opens the palette.
 */
@Component({
  selector: 'app-global-search',
  imports: [AppTooltip, CommandPalette, MarkText, ShellIcon, CdkConnectedOverlay, CdkOverlayOrigin],
  templateUrl: './global-search.html',
  styleUrl: './global-search.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class GlobalSearch {
  private readonly store = inject(SearchStore);
  private readonly searchApi = inject(SearchApiService);
  private readonly commands = inject(CommandRegistry);
  private readonly hotkeys = inject(HotkeysService);
  private readonly theme = inject(ThemeService);
  private readonly router = inject(Router);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly destroyRef = inject(DestroyRef);

  /** Narrow layouts: the field becomes an icon button that opens the command palette. */
  readonly compact = input(false);

  /** Shortcut that opens the palette, written for the platform of the user. */
  protected readonly shortcutHint = /Mac|iPhone|iPad/i.test(globalThis.navigator?.platform ?? '')
    ? '⌘ K'
    : 'Ctrl K';

  protected readonly positions = PANEL_POSITIONS;
  /** The results close when the page scrolls under them, so they never float away from the field. */
  protected readonly scrollStrategy = inject(Overlay).scrollStrategies.close();

  protected readonly query = signal('');
  private readonly panelRequested = signal(false);
  protected readonly paletteOpen = signal(false);
  protected readonly paletteSeed = signal('');
  private timer: ReturnType<typeof setTimeout> | undefined;

  protected readonly hits = this.store.hits;
  protected readonly sections = computed(() => sectionsOf(this.hits()));
  protected readonly message = computed<string | null>(() => {
    if (this.store.results.loading()) return null;
    if (this.store.results.error() !== null) return 'No se pudo buscar. Intenta de nuevo.';
    if (this.store.searchable() && this.store.results.hasValue() && this.hits().length === 0) {
      return 'Sin resultados';
    }
    return null;
  });
  protected readonly panelOpen = computed(
    () => this.panelRequested() && (this.hits().length > 0 || this.message() !== null),
  );

  /** The palette reads its results through the same search endpoint. */
  protected readonly searchPalette: PaletteSearch = (term) =>
    firstValueFrom(this.searchApi.search(term));

  constructor() {
    const unbindSlash = this.hotkeys.register([
      {
        keys: '/',
        group: 'General',
        description: 'Abrir la paleta de comandos',
        run: () => this.openPalette(),
      },
    ]);
    const unregister = [
      this.commands.register({
        id: 'nav-dashboard',
        title: 'Ir a Mi resumen',
        keywords: ['dashboard', 'inicio'],
        group: 'Navegación',
        run: () => this.go('/dashboard'),
      }),
      this.commands.register({
        id: 'nav-issues',
        title: 'Ir a Incidencias',
        keywords: ['issues', 'incidencias'],
        group: 'Navegación',
        run: () => this.go('/issues'),
      }),
      this.commands.register({
        id: 'nav-projects',
        title: 'Ir a Proyectos',
        keywords: ['projects'],
        group: 'Navegación',
        run: () => this.go('/projects'),
      }),
      this.commands.register({
        id: 'nav-teams',
        title: 'Ir a Equipos',
        keywords: ['teams'],
        group: 'Navegación',
        run: () => this.go('/teams'),
      }),
      this.commands.register({
        id: 'nav-profile',
        title: 'Ir a Perfil',
        keywords: ['profile', 'cuenta'],
        group: 'Navegación',
        run: () => this.go('/profile'),
      }),
      this.commands.register({
        id: 'act-create-issue',
        title: 'Crear incidencia',
        keywords: ['nueva', 'create', 'issue'],
        group: 'Acciones',
        run: () => this.go('/issues/new'),
      }),
      this.commands.register({
        id: 'act-theme',
        title: 'Cambiar tema',
        keywords: ['claro', 'oscuro', 'theme'],
        group: 'Acciones',
        run: () => this.theme.toggle(),
      }),
      this.commands.register({
        id: 'act-shortcuts',
        title: 'Atajos de teclado',
        keywords: ['teclas', 'shortcuts', 'ayuda'],
        group: 'Acciones',
        // The shell listens for '?' on the window, so the command sends that key.
        run: () => window.dispatchEvent(new KeyboardEvent('keydown', { key: '?', bubbles: true })),
      }),
    ];
    this.destroyRef.onDestroy(() => {
      unbindSlash();
      unregister.forEach((remove) => remove());
      clearTimeout(this.timer);
    });
  }

  /** Ctrl+K (or Cmd+K) toggles the command palette. */
  @HostListener('window:keydown', ['$event'])
  protected onWindowKeydown(event: KeyboardEvent): void {
    if ((event.ctrlKey || event.metaKey) && !event.altKey && event.key.toLowerCase() === 'k') {
      event.preventDefault();
      this.paletteOpen.update((open) => !open);
    }
  }

  /** Closes the results panel when the pointer goes down outside the search control. */
  @HostListener('document:mousedown', ['$event'])
  protected onDocumentMouseDown(event: MouseEvent): void {
    const target = event.target as Node | null;
    // The results live in the overlay container, outside the host element.
    const inside =
      this.host.nativeElement.contains(target) ||
      (target instanceof Element && target.closest('.global-search__panel') !== null);
    if (!inside) this.panelRequested.set(false);
  }

  /** The overlay detached while the results were still wanted: the page scrolled, so they close. */
  protected onPanelDetach(): void {
    if (this.panelOpen()) this.panelRequested.set(false);
  }

  /** Waits for a pause in typing before searching. Shorter queries clear the results and send nothing. */
  protected onInput(event: Event): void {
    const value = (event.target as HTMLInputElement).value;
    this.query.set(value);
    clearTimeout(this.timer);
    if (value.trim().length < SEARCH_MIN_CHARS) {
      this.store.search(value);
      this.panelRequested.set(false);
      return;
    }
    this.timer = setTimeout(() => {
      this.store.search(value.trim());
      this.panelRequested.set(true);
    }, HEADER_SEARCH_DEBOUNCE_MS);
  }

  protected onKeydown(event: KeyboardEvent): void {
    if (event.key === 'Escape') {
      this.panelRequested.set(false);
    }
  }

  protected choose(hit: SearchHit): void {
    this.panelRequested.set(false);
    this.query.set('');
    this.store.search('');
    void this.router.navigateByUrl(hit.link);
  }

  protected openPalette(seed = ''): void {
    this.paletteSeed.set(seed);
    this.paletteOpen.set(true);
  }

  protected openFullPalette(): void {
    this.panelRequested.set(false);
    this.openPalette(this.query().trim());
  }

  /** Every result of the typed text, in the issue navigator. */
  protected openAllResults(term = this.query().trim()): void {
    this.panelRequested.set(false);
    if (term === '') return;
    void this.router.navigate(['/issues'], {
      queryParams: { jql: `text ~ ${quoteValue(term)}` },
    });
  }

  protected onPaletteHit(hit: SearchHit): void {
    void this.router.navigateByUrl(hit.link);
  }

  private go(path: string): void {
    void this.router.navigateByUrl(path);
  }
}
