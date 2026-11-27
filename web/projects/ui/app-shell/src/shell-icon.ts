import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

import { ICON_PATHS, type ShellIconName } from './shell-icon-paths';

export type { ShellIconName } from './shell-icon-paths';

/** Decorative line icon of the shell. Hidden from assistive technology; the owning control carries the name. */
@Component({
  selector: 'app-shell-icon',
  templateUrl: './shell-icon.html',
  styleUrl: './shell-icon.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ShellIcon {
  readonly name = input.required<ShellIconName>();
  protected readonly paths = computed(() => ICON_PATHS[this.name()]);
}
