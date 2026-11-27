import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  signal,
  viewChild,
} from '@angular/core';
import { Router } from '@angular/router';
import {
  AuthStore,
  AvatarUrlService,
  type AvatarUrlWatch,
  ThemeService,
  UiPreferencesService,
  type AppRole,
} from '@corelog/core';
import { Avatar } from '@corelog/ui/avatar';
import type { MenuItem } from 'primeng/api';
import { Menu } from 'primeng/menu';

const ROLE_LABELS: Readonly<Record<AppRole, string>> = {
  admin: 'Administrador',
  user: 'Usuario',
};

/** Avatar menu: profile, theme, density and sign out. */
@Component({
  selector: 'app-user-menu',
  imports: [Avatar, Menu],
  templateUrl: './user-menu.html',
  styleUrl: './user-menu.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class UserMenu {
  private readonly auth = inject(AuthStore);
  private readonly router = inject(Router);
  private readonly theme = inject(ThemeService);
  private readonly preferences = inject(UiPreferencesService);
  private readonly avatars = inject(AvatarUrlService);

  private readonly menu = viewChild.required<Menu>('menu');
  private readonly avatarWatch = signal<AvatarUrlWatch | null>(null);

  protected readonly user = this.auth.user;
  protected readonly photo = computed(() => this.avatarWatch()?.url() ?? null);

  protected readonly items = computed<MenuItem[]>(() => {
    const current = this.user();
    if (!current) return [];
    const compact = this.preferences.density() === 'compact';
    return [
      { label: current.name, disabled: true, styleClass: 'identity-name' },
      {
        label: `${current.email} · ${ROLE_LABELS[current.role]}`,
        disabled: true,
        styleClass: 'identity-meta',
      },
      { separator: true },
      { label: 'Mi perfil', command: () => this.go('/profile') },
      {
        label: this.theme.isDark() ? 'Usar tema claro' : 'Usar tema oscuro',
        command: () => this.theme.toggle(),
      },
      {
        label: compact ? 'Densidad cómoda' : 'Densidad compacta',
        command: () => this.preferences.setDensity(compact ? 'comfortable' : 'compact'),
      },
      { separator: true },
      {
        label: 'Cerrar sesión',
        styleClass: 'danger',
        command: () => {
          this.auth.logout();
          this.go('/login');
        },
      },
    ];
  });

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

  protected toggle(event: Event): void {
    this.menu().toggle(event);
  }

  private go(path: string): void {
    void this.router.navigateByUrl(path);
  }
}
