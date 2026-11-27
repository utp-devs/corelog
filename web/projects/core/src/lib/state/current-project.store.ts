import { Injectable, computed, inject, signal } from '@angular/core';
import { tap } from 'rxjs';

import { ProjectNavItem, buildProjectNav } from '../config/project-nav';
import { EffectivePermissions, EffectiveRole, Project } from '../models';
import { ProjectApiService } from '../services/api/project-api.service';
import { ProjectPermissionsApiService } from '../services/api/project-permissions-api.service';
import { AsyncState } from './async-state';
import { resetOnLogout } from './session-scoped';

/**
 * The project the user is inside of (the route of `/projects/:key/...`) with the caller's effective
 * permissions. The route resolver starts it through `open`; the pages and the navigation read it.
 * `project` is null while loading and after a failure; `error` keeps the failure (404 not found or
 * hidden, 403 forbidden, network) so the project area can render its full-page state.
 */
@Injectable({ providedIn: 'root' })
export class CurrentProjectStore {
  private readonly api = inject(ProjectApiService);
  private readonly permissionsApi = inject(ProjectPermissionsApiService);

  private readonly projectRead = new AsyncState<Project>();
  private readonly permissionsRead = new AsyncState<EffectivePermissions>();
  private readonly requested = signal<string | null>(null);

  /** Identifier (key or UUID) taken from the route that the store was last opened with. */
  readonly identifier = this.requested.asReadonly();
  readonly project = this.projectRead.value;
  readonly loading = this.projectRead.loading;
  /** Failure of the project read. Null while it succeeds or is pending. */
  readonly error = this.projectRead.error;
  readonly permissions = this.permissionsRead.value;
  readonly permissionsLoaded = this.permissionsRead.hasValue;

  /** UUID of the project, or null while there is none. */
  readonly projectId = computed(() => this.project()?.id ?? null);
  /** Key of the project, or null while there is none. */
  readonly key = computed(() => this.project()?.key ?? null);
  /** Effective role of the caller (`none` when it has no role), or null until it is known. */
  readonly role = computed<EffectiveRole | null>(() => this.permissions()?.role ?? null);
  readonly isArchived = computed(() => !!this.project()?.archivedAt);

  /** Whether the caller holds the permission. False until the permissions are known. */
  can(permission: string): boolean {
    return this.permissions()?.permissions.includes(permission) ?? false;
  }

  /**
   * Navigation entries of the project, filtered by template and permissions. While the permissions
   * are still loading every entry that the template allows is listed, so the tabs do not flicker.
   */
  readonly navItems = computed<ProjectNavItem[]>(() => {
    const project = this.project();
    if (!project) return [];
    const permissions = this.permissions();
    return buildProjectNav(project, (permission) =>
      permissions ? permissions.permissions.includes(permission) : true,
    );
  });

  constructor() {
    resetOnLogout(() => this.reset());
  }

  /**
   * Starts reading the project and, once known, its effective permissions. Does not wait: `project`
   * stays null while the read is pending and `error` keeps a failure. Opening another identifier
   * clears the previous project first; a newer call cancels the read in flight.
   */
  open(identifier: string): void {
    if (identifier !== this.requested()) {
      this.projectRead.reset();
      this.permissionsRead.reset();
      this.requested.set(identifier);
    }
    this.projectRead.load(
      this.api
        .get(identifier, { silent: true })
        .pipe(tap((project) => this.loadPermissions(project.id))),
    );
  }

  /** Reads the project again by the identifier it was opened with. */
  reload(): void {
    const identifier = this.requested();
    if (identifier) this.open(identifier);
  }

  /** Replaces the project after an edit, archive or restore without another read. */
  setProject(project: Project): void {
    if (this.project()?.id === project.id) this.projectRead.set(project);
  }

  /** Reads the permissions again (after a scheme or role change). */
  reloadPermissions(): void {
    const id = this.projectId();
    if (id) this.loadPermissions(id);
  }

  reset(): void {
    this.projectRead.reset();
    this.permissionsRead.reset();
    this.requested.set(null);
  }

  private loadPermissions(projectId: string): void {
    this.permissionsRead.load(this.permissionsApi.me(projectId, { silent: true }));
  }
}
