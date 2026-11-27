import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  input,
  output,
  signal,
} from '@angular/core';
import {
  AuthStore,
  AvatarUrlService,
  LOCATION_ASSIGN,
  LOGIN_URL,
  type AvatarUrlWatch,
  ThemeService,
  UiPreferencesService,
  type AppRole,
  type ThemeMode,
} from '@corelog/core';
import { Avatar } from '@corelog/ui/avatar';
import { AppTooltip } from '@corelog/ui/tooltip';

import { DropdownMenu } from './dropdown-menu';
import { ShellIcon, type ShellIconName } from './shell-icon';

const ROLE_LABELS: Readonly<Record<AppRole, string>> = {
  admin: 'Administrador',
  user: 'Usuario',
};

interface ThemeChoice {
  readonly mode: ThemeMode;
  readonly label: string;
  readonly icon: ShellIconName;
}

const THEME_CHOICES: readonly ThemeChoice[] = [
  { mode: 'light', label: 'Claro', icon: 'sun' },
  { mode: 'dark', label: 'Oscuro', icon: 'moon' },
  { mode: 'system', label: 'Sistema', icon: 'monitor' },
];

/** Avatar menu: identity, profile, theme (light, dark, system), compact density, shortcuts and sign out. */
@Component({
  selector: 'app-user-menu',
  imports: [AppTooltip, Avatar, DropdownMenu, ShellIcon],
  templateUrl: './user-menu.html',
  styleUrl: './user-menu.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class UserMenu {
  private readonly auth = inject(AuthStore);
  private readonly assign = inject(LOCATION_ASSIGN);
  private readonly loginUrl = inject(LOGIN_URL);
  private readonly theme = inject(ThemeService);
  private readonly preferences = inject(UiPreferencesService);
  private readonly avatars = inject(AvatarUrlService);

  private readonly avatarWatch = signal<AvatarUrlWatch | null>(null);

  /** Profile lives in the workspace application: opened with a full-page navigation. */
  readonly profileUrl = input('/workspace/profile');
  readonly openShortcuts = output<void>();

  protected readonly user = this.auth.user;
  protected readonly photo = computed(() => this.avatarWatch()?.url() ?? null);
  protected readonly roleLabel = computed(() => {
    const current = this.user();
    return current ? ROLE_LABELS[current.role] : '';
  });
  protected readonly themeChoices = THEME_CHOICES;
  protected readonly themeMode = this.theme.mode;
  protected readonly compact = computed(() => this.preferences.density() === 'compact');

  constructor() {
    // One avatar reader per signed-in user; the previous reader is released when the user changes.
    effect((onCleanup) => {
      const current = this.user();
      const watch = current
        ? this.avatars.watch(current.id, current.avatarKey !== null, current.avatarKey ?? '')
        : null;
      this.avatarWatch.set(watch);
      onCleanup(() => watch?.release());
    });
  }

  protected goToProfile(): void {
    this.assign(this.profileUrl());
  }

  protected setTheme(mode: ThemeMode): void {
    this.theme.setMode(mode);
  }

  protected toggleDensity(): void {
    this.preferences.setDensity(this.compact() ? 'comfortable' : 'compact');
  }

  protected signOut(): void {
    this.auth.logout();
    this.assign(this.loginUrl);
  }
}
