import { ChangeDetectionStrategy, Component, computed, input, model } from '@angular/core';
import { Dialog } from '@corelog/ui/dialog';
import { Button } from 'primeng/button';

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
  imports: [Button, Dialog],
  templateUrl: './shortcuts-dialog.html',
  styleUrl: './shortcuts-dialog.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ShortcutsDialog {
  /** Whether the dialog is shown. The dialog sets it to false when it closes itself. */
  readonly open = model(false);
  /** Shortcuts to list. */
  readonly hotkeys = input<readonly ShortcutEntry[]>([]);

  protected readonly groups = computed(() => groupShortcuts(this.hotkeys()));

  protected close(): void {
    this.open.set(false);
  }
}
