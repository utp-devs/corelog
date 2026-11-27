import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

export type ShellIconName =
  | 'menu'
  | 'plus'
  | 'caret'
  | 'search'
  | 'bell'
  | 'help'
  | 'edit'
  | 'close'
  | 'board'
  | 'backlog'
  | 'sprints'
  | 'calendar'
  | 'permissions'
  | 'settings'
  | 'members'
  | 'invitations'
  | 'profile'
  | 'fold'
  | 'unfold';

/** Stroke paths on a 24 px grid. Each icon may use several paths. */
const ICON_PATHS: Readonly<Record<ShellIconName, readonly string[]>> = {
  menu: ['M4 7h16', 'M4 12h16', 'M4 17h16'],
  plus: ['M12 5v14', 'M5 12h14'],
  caret: ['M6 9l6 6 6-6'],
  search: ['M10.5 3.5a7 7 0 1 0 0 14 7 7 0 1 0 0-14z', 'M20 20l-4.3-4.3'],
  bell: ['M6 16V11a6 6 0 0 1 12 0v5l2 2H4z', 'M10 21h4'],
  help: ['M9 9a3 3 0 1 1 4 2.8c-.7.3-1 1-1 1.7V14', 'M12 18v.01'],
  edit: ['M4 20h4L19 9l-4-4L4 16z'],
  close: ['M6 6l12 12', 'M18 6L6 18'],
  board: ['M4 4h7v7H4z', 'M13 4h7v7h-7z', 'M4 13h7v7H4z', 'M13 13h7v7h-7z'],
  backlog: ['M9 6h11', 'M9 12h11', 'M9 18h11', 'M4 6h.01', 'M4 12h.01', 'M4 18h.01'],
  sprints: ['M13 2L4 14h7l-1 8 9-12h-7z'],
  calendar: ['M4 6h16v14H4z', 'M4 10h16', 'M8 3v4', 'M16 3v4'],
  permissions: ['M12 3l8 3v6c0 5-3.5 8-8 9-4.5-1-8-4-8-9V6z'],
  settings: [
    'M12 9a3 3 0 1 0 0 6 3 3 0 0 0 0-6z',
    'M4 12h2M18 12h2M12 4v2M12 18v2M6.3 6.3l1.4 1.4M16.3 16.3l1.4 1.4M6.3 17.7l1.4-1.4M16.3 7.7l1.4-1.4',
  ],
  members: [
    'M9 11a3 3 0 1 0 0-6 3 3 0 0 0 0 6z',
    'M3 20c0-3 3-5 6-5s6 2 6 5',
    'M16 11a3 3 0 1 0 0-6',
    'M17 15c2 .5 4 2 4 5',
  ],
  invitations: ['M3 6h18v12H3z', 'M3 7l9 6 9-6'],
  profile: ['M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8z', 'M4 21c0-4 4-6 8-6s8 2 8 6'],
  fold: ['M15 6l-6 6 6 6'],
  unfold: ['M9 6l6 6-6 6'],
};

/** Decorative line icon of the shell. Hidden from assistive technology; the owning control carries the name. */
@Component({
  selector: 'app-shell-icon',
  template: `<svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
    @for (d of paths(); track $index) {
      <path [attr.d]="d" />
    }
  </svg>`,
  styleUrl: './shell-icon.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ShellIcon {
  readonly name = input.required<ShellIconName>();
  protected readonly paths = computed(() => ICON_PATHS[this.name()]);
}
