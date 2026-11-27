import {
  ChangeDetectionStrategy,
  Component,
  computed,
  input,
  output,
  viewChild,
} from '@angular/core';
import type { MenuItem } from 'primeng/api';
import { Menu } from 'primeng/menu';

import { ShellIcon } from './shell-icon';

/** Help dropdown: keyboard shortcuts and, when a documentation address is configured, the documentation link. */
@Component({
  selector: 'app-help-menu',
  imports: [Menu, ShellIcon],
  templateUrl: './help-menu.html',
  styleUrl: './help-menu.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class HelpMenu {
  /** Documentation address. The entry is hidden when it is not set. */
  readonly docsUrl = input<string | null>(null);
  readonly openShortcuts = output<void>();

  private readonly menu = viewChild.required<Menu>('menu');

  protected readonly items = computed<MenuItem[]>(() => {
    const docs = this.docsUrl();
    const entries: MenuItem[] = [
      { label: 'Atajos de teclado', command: () => this.openShortcuts.emit() },
    ];
    if (docs) {
      entries.push({ label: 'Documentación', url: docs, target: '_blank' });
    }
    return entries;
  });

  protected toggle(event: Event): void {
    this.menu().toggle(event);
  }
}
