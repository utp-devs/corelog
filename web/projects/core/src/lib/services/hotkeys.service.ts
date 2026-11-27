import { DestroyRef, DOCUMENT, Injectable, Signal, inject, signal } from '@angular/core';
import { Router } from '@angular/router';

/** Time allowed between the two keys of a sequence such as "g b". */
export const SEQUENCE_TIMEOUT_MS = 1000;

export type HotkeyBinding = {
  /** A single key ('c', '/', '?') or a two-key sequence separated by one space ('g b'). */
  keys: string;
  /** Spanish text shown in the shortcuts dialog. */
  description: string;
  /** Heading of the shortcuts dialog section. */
  group: string;
  /** Listed in the dialog as "Próximamente" and never fired. */
  comingSoon?: boolean;
  run?: (event: KeyboardEvent) => void;
};

export type RegisteredHotkey = Pick<HotkeyBinding, 'keys' | 'description' | 'group' | 'comingSoon'>;

export type GlobalShortcutOptions = {
  /** Opens the shortcuts dialog ('?'). */
  onHelp?: () => void;
  /** Opens the quick create-issue dialog ('c'). Without it the shortcut opens the new-issue page. */
  onCreate?: () => void;
  /** Project in context: 'g b' opens its board. */
  projectId?: string | null;
  /**
   * The app mounts the work pages (issues, projects, dashboard). Without them only the help and
   * note shortcuts are registered, so no shortcut navigates to a path the app does not serve.
   */
  workPages?: boolean;
};

/** True when the target is a field the user types into: shortcuts must not fire there. */
export function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  if (
    target.isContentEditable ||
    target.closest('[contenteditable]:not([contenteditable="false"])')
  ) {
    return true;
  }
  return ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName);
}

/**
 * Keyboard shortcuts on the window. Single keys fire at once; sequences fire when the second key
 * arrives within SEQUENCE_TIMEOUT_MS. Nothing fires while typing in a field, and bindings with
 * Ctrl, Meta or Alt are ignored. Each `register` call returns the function that removes it.
 */
@Injectable({ providedIn: 'root' })
export class HotkeysService {
  private readonly document = inject(DOCUMENT);
  private readonly router = inject(Router);
  private readonly entries = new Map<symbol, readonly HotkeyBinding[]>();
  private readonly registry = signal<readonly RegisteredHotkey[]>([]);
  private pending: { prefix: string; timer: ReturnType<typeof setTimeout> } | null = null;

  /** Every mounted shortcut in registration order. Used by the shortcuts dialog. */
  readonly registered: Signal<readonly RegisteredHotkey[]> = this.registry.asReadonly();

  constructor() {
    const win = this.document.defaultView;
    const listener = (event: KeyboardEvent) => this.handleKeyDown(event);
    win?.addEventListener('keydown', listener);
    inject(DestroyRef).onDestroy(() => {
      win?.removeEventListener('keydown', listener);
      this.clearPending();
    });
  }

  /** Adds bindings and returns the function that removes them. */
  register(bindings: readonly HotkeyBinding[]): () => void {
    const owner = Symbol('hotkeys');
    this.entries.set(owner, [...bindings]);
    this.publish();
    return () => {
      if (this.entries.delete(owner)) this.publish();
    };
  }

  /** Shell shortcuts: c, ?, g i, g d, g p, g b and n (coming soon). '/' belongs to the command palette. */
  registerGlobalShortcuts(options: GlobalShortcutOptions = {}): () => void {
    const { onHelp, onCreate, projectId, workPages = true } = options;
    const boardPath = projectId ? `/projects/${encodeURIComponent(projectId)}/board` : '/projects';
    const go = (path: string) => () => {
      void this.router.navigateByUrl(path);
    };

    const bindings: HotkeyBinding[] = [
      {
        keys: 'c',
        group: 'General',
        description: 'Crear incidencia',
        run: onCreate ? () => onCreate() : go('/issues/new'),
      },
      {
        keys: '?',
        group: 'General',
        description: 'Mostrar atajos de teclado',
        run: () => onHelp?.(),
      },
      { keys: 'n', group: 'General', description: 'Notas rápidas', comingSoon: true },
      { keys: 'g i', group: 'Navegación', description: 'Ir a Incidencias', run: go('/issues') },
      { keys: 'g d', group: 'Navegación', description: 'Ir a Mi resumen', run: go('/dashboard') },
      { keys: 'g p', group: 'Navegación', description: 'Ir a Proyectos', run: go('/projects') },
      {
        keys: 'g b',
        group: 'Navegación',
        description: projectId
          ? 'Ir al tablero del proyecto'
          : 'Ir a Proyectos (no hay proyecto abierto)',
        run: go(boardPath),
      },
    ];
    const helpAndNotes = bindings.filter((binding) => binding.keys === '?' || binding.keys === 'n');
    return this.register(workPages ? bindings : helpAndNotes);
  }

  private publish(): void {
    this.registry.set(
      Array.from(this.entries.values())
        .flat()
        .map(({ keys, description, group, comingSoon }) => ({
          keys,
          description,
          group,
          comingSoon: comingSoon === true,
        })),
    );
  }

  private activeBindings(): HotkeyBinding[] {
    return Array.from(this.entries.values())
      .flat()
      .filter((binding) => !binding.comingSoon && binding.run !== undefined);
  }

  private clearPending(): void {
    if (this.pending) {
      globalThis.clearTimeout(this.pending.timer);
      this.pending = null;
    }
  }

  private handleKeyDown(event: KeyboardEvent): void {
    if (
      event.ctrlKey ||
      event.metaKey ||
      event.altKey ||
      event.isComposing ||
      isEditableTarget(event.target)
    ) {
      this.clearPending();
      return;
    }

    const key = event.key;
    const active = this.activeBindings();
    const prefix = this.pending?.prefix;
    this.clearPending();

    if (prefix !== undefined) {
      const sequence = active.find((binding) => binding.keys === `${prefix} ${key}`);
      if (sequence?.run) {
        event.preventDefault();
        sequence.run(event);
        return;
      }
    }

    const single = active.find((binding) => binding.keys === key);
    if (single?.run) {
      event.preventDefault();
      single.run(event);
      return;
    }

    if (active.some((binding) => binding.keys.startsWith(`${key} `))) {
      this.pending = {
        prefix: key,
        timer: globalThis.setTimeout(() => this.clearPending(), SEQUENCE_TIMEOUT_MS),
      };
    }
  }
}
