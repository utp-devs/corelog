import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { AppTooltip } from '@corelog/ui/tooltip';

import { DropdownMenu } from './dropdown-menu';
import { ShellIcon } from './shell-icon';

/**
 * Help dropdown: keyboard shortcuts plus the documentation, news and support links. A link is
 * listed only when its address is configured.
 */
@Component({
  selector: 'app-help-menu',
  imports: [AppTooltip, DropdownMenu, ShellIcon],
  templateUrl: './help-menu.html',
  styleUrl: './help-menu.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class HelpMenu {
  /** Documentation address. */
  readonly docsUrl = input<string | null>(null);
  /** Release notes address. */
  readonly whatsNewUrl = input<string | null>(null);
  /** Support address (a page or a `mailto:` link). */
  readonly supportUrl = input<string | null>(null);
  readonly openShortcuts = output<void>();
}
