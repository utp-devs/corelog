import { Injectable, computed, inject, signal } from '@angular/core';
import {
  AuthStore,
  FavoritesStore,
  OrganizationStore,
  TeamStore,
  getRecent,
  type RecentItem,
} from '@corelog/core';

import { buildMenuModel, type MenuContext, type MenuModel, type MenuStatus } from './menu-model';
import type { NavMenu } from './navigation';

/** Recent projects read per opening: room for five rows once the starred ones are left out. */
const MENU_PROJECT_FETCH = 10;

/**
 * Data behind the top menus: the starred and recently visited projects, the recent issues and the teams of the user.
 * The lists are read when a menu opens, so they are as fresh as the last visit.
 */
@Injectable({ providedIn: 'root' })
export class ShellMenuData {
  private readonly auth = inject(AuthStore);
  private readonly favorites = inject(FavoritesStore);
  private readonly orgs = inject(OrganizationStore);
  private readonly teamStore = inject(TeamStore);

  private readonly recent = signal<readonly RecentItem[]>(getRecent());
  private teamsRequested = false;

  /** Loading while a read is in flight and nothing is listed yet; error when both reads failed. */
  private readonly projectsStatus = computed<MenuStatus>(() => {
    const listed = this.favorites.favorites().length + this.favorites.recent().length > 0;
    if (!listed && (this.favorites.favoritesLoading() || this.favorites.recentLoading())) {
      return 'loading';
    }
    if (!listed && this.favorites.favoritesError() && this.favorites.recentError()) return 'error';
    return 'ready';
  });

  readonly context = computed<MenuContext>(() => ({
    recent: this.recent(),
    favoriteProjects: this.favorites.favorites(),
    recentProjects: this.favorites.recent(),
    projectsStatus: this.projectsStatus(),
    teams: this.teamStore.teams().map((team) => ({
      id: team.id,
      name: team.name,
      hint:
        team.memberCount === undefined
          ? ''
          : `${team.memberCount} ${team.memberCount === 1 ? 'persona' : 'personas'}`,
    })),
    canCreateProject: this.orgs.canCreateProject(),
    canCreateTeam: this.auth.isAuthenticated(),
  }));

  /** Re-reads the recent items and loads the lists a menu needs, once. */
  refresh(menu: NavMenu): void {
    this.recent.set(getRecent());
    if (menu.key === 'projects') {
      this.favorites.fetchFavorites();
      this.favorites.fetchRecent(MENU_PROJECT_FETCH);
    }
    if (menu.key === 'teams' && !this.teamsRequested && this.teamStore.teams().length === 0) {
      this.teamsRequested = true;
      this.teamStore.fetchTeams();
    }
  }

  modelOf(menu: NavMenu): MenuModel {
    return buildMenuModel(menu, this.context());
  }
}
