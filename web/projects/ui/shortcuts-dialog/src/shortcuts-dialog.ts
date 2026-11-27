import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  Injector,
  afterNextRender,
  computed,
  effect,
  inject,
  input,
  model,
  untracked,
  viewChild,
} from '@angular/core';

/** One keyboard shortcut as listed in the dialog. The shell supplies the list for the current screen. */
export interface ShortcutEntry {
  /** A single key ('c', '/', '?') or a two-key sequence separated by one space ('g b'). */
  readonly keys: string;
  /** Spanish text shown in the dialog. */
  readonly description: string;
  /** Heading of the dialog section. Groups keep the order of first appearance. */
  readonly group: string;
  /** Listed as "Próximamente" instead of its keys. */
  readonly comingSoon?: boolean;
}

export interface ShortcutRow {
  readonly description: string;
  readonly comingSoon: boolean;
  readonly tokens: readonly string[];
}

export interface ShortcutGroup {
  readonly name: string;
  readonly id: string;
  readonly items: readonly ShortcutRow[];
}

let nextDialogId = 0;

/** Groups shortcuts by their group name, keeping the order of first appearance. */
export function groupShortcuts(entries: readonly ShortcutEntry[]): ShortcutGroup[] {
  const groups: { name: string; items: ShortcutRow[] }[] = [];
  for (const entry of entries) {
    const row: ShortcutRow = {
      description: entry.description,
      comingSoon: entry.comingSoon === true,
      tokens: entry.keys.split(' '),
    };
    const existing = groups.find((group) => group.name === entry.group);
    if (existing) {
      existing.items.push(row);
    } else {
      groups.push({ name: entry.group, items: [row] });
    }
  }
  return groups.map((group, index) => ({ ...group, id: `shortcuts-group-${index}` }));
}

/**
 * Lists the keyboard shortcuts the caller provides, grouped. Opened by the shell (for example with
 * '?'). Closes with Escape, the close button or a click on the backdrop, and returns focus to the
 * element that was focused before it opened.
 */
@Component({
  selector: 'app-shortcuts-dialog',
  templateUrl: './shortcuts-dialog.html',
  styleUrl: './shortcuts-dialog.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ShortcutsDialog {
  private readonly injector = inject(Injector);
  private readonly dialogRef = viewChild<ElementRef<HTMLElement>>('dialog');

  /** Whether the dialog is shown. The dialog sets it to false when it closes itself. */
  readonly open = model(false);
  /** Shortcuts to list. */
  readonly hotkeys = input<readonly ShortcutEntry[]>([]);

  protected readonly titleId = `shortcuts-dialog-title-${nextDialogId++}`;
  protected readonly groups = computed(() => groupShortcuts(this.hotkeys()));

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
  }

  protected close(): void {
    this.open.set(false);
  }

  protected onBackdropMouseDown(event: MouseEvent): void {
    if (event.target === event.currentTarget) {
      this.close();
    }
  }

  protected onKeydown(event: KeyboardEvent): void {
    if (event.key === 'Escape') {
      event.preventDefault();
      this.close();
      return;
    }
    if (event.key === 'Tab') {
      this.trapTab(event);
    }
  }

  private onOpened(): void {
    const active = document.activeElement;
    this.previousFocus = active instanceof HTMLElement ? active : null;
    afterNextRender(() => this.dialogRef()?.nativeElement.focus(), { injector: this.injector });
  }

  private onClosed(): void {
    if (this.previousFocus === null) {
      return;
    }
    const previous = this.previousFocus;
    this.previousFocus = null;
    previous.focus();
  }

  /** Keeps Tab and Shift+Tab cycling between the focusable elements of the dialog. */
  private trapTab(event: KeyboardEvent): void {
    const dialog = this.dialogRef()?.nativeElement;
    if (!dialog) {
      return;
    }
    const focusable = Array.from(
      dialog.querySelectorAll<HTMLElement>('button, [href], [tabindex]:not([tabindex="-1"])'),
    );
    if (focusable.length === 0) {
      event.preventDefault();
      return;
    }
    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    if (event.shiftKey && (document.activeElement === first || document.activeElement === dialog)) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  }
}
