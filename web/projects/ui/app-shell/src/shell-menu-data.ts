import { Injectable, computed, inject, signal } from '@angular/core';
import { AuthStore, ProjectStore, TeamStore, getRecent, type RecentItem } from '@corelog/core';

import { buildMenuModel, type MenuContext, type MenuModel } from './menu-model';
import type { NavMenu } from './navigation';

/**
 * Data behind the top menus: the recently visited items, the projects and the teams of the user.
 * The lists are read when a menu opens, so they are as fresh as the last visit.
 */
@Injectable({ providedIn: 'root' })
export class ShellMenuData {
  private readonly auth = inject(AuthStore);
  private readonly projectStore = inject(ProjectStore);
  private readonly teamStore = inject(TeamStore);

  private readonly recent = signal<readonly RecentItem[]>(getRecent());
  private projectsRequested = false;
  private teamsRequested = false;

  readonly context = computed<MenuContext>(() => ({
    recent: this.recent(),
    projects: this.projectStore.projects().map((project) => ({
      id: project.id,
      name: project.name,
      hint: project.key,
    })),
    teams: this.teamStore.teams().map((team) => ({
      id: team.id,
      name: team.name,
      hint:
        team.memberCount === undefined
          ? ''
          : `${team.memberCount} ${team.memberCount === 1 ? 'persona' : 'personas'}`,
    })),
    canCreateProject: this.auth.user()?.role === 'admin',
    canCreateTeam: this.auth.isAuthenticated(),
  }));

  /** Re-reads the recent items and loads the lists a menu needs, once. */
  refresh(menu: NavMenu): void {
    this.recent.set(getRecent());
    if (
      menu.key === 'projects' &&
      !this.projectsRequested &&
      this.projectStore.projects().length === 0
    ) {
      this.projectsRequested = true;
      this.projectStore.fetchProjects();
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
