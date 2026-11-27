import { DestroyRef, Injectable, computed, effect, inject, signal, untracked } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import {
  CurrentProjectStore,
  OrganizationApiService,
  OrganizationMember,
  ProjectPermissionsStore,
  ProjectStore,
  TeamStore,
} from '@corelog/core';

/** A person of the organization, as the settings pages show them. */
export interface SettingsPerson {
  readonly userId: string;
  readonly name: string;
  readonly email: string;
  readonly hasAvatar: boolean;
}

export type DirectoryStatus = 'loading' | 'ready' | 'error';

/**
 * State shared by the project settings pages: who may change what, the people of the organization,
 * the members of the project and its permission scheme. Provided by the settings shell, so it lives
 * as long as the settings area is open.
 */
@Injectable()
export class ProjectSettingsContext {
  private readonly current = inject(CurrentProjectStore);
  private readonly orgApi = inject(OrganizationApiService);
  private readonly destroyRef = inject(DestroyRef);

  readonly projects = inject(ProjectStore);
  readonly permissions = inject(ProjectPermissionsStore);
  readonly teams = inject(TeamStore);

  readonly project = this.current.project;
  readonly projectId = this.current.projectId;
  readonly archived = this.current.isArchived;
  readonly role = this.current.role;

  /** Edit the details, the permission matrix and archive or restore the project. */
  readonly canEdit = computed(() => this.current.can('edit_project'));
  readonly canManageMembers = computed(() => this.current.can('manage_members'));
  readonly canDelete = computed(() => this.current.can('delete_project'));

  /** Same permissions with the archived project being read-only. */
  readonly canEditNow = computed(() => this.canEdit() && !this.archived());
  readonly canManageMembersNow = computed(() => this.canManageMembers() && !this.archived());

  readonly directory = signal<readonly OrganizationMember[]>([]);
  readonly directoryStatus = signal<DirectoryStatus>('loading');

  readonly people = computed(
    () =>
      new Map<string, SettingsPerson>(
        this.directory().map((member) => [
          member.userId,
          {
            userId: member.userId,
            name: member.displayName || member.name,
            email: member.email,
            hasAvatar: member.hasAvatar,
          },
        ]),
      ),
  );

  readonly members = this.projects.members;
  readonly teamRoles = this.permissions.teamRoles;

  private loadedProject: string | null = null;
  private loadedOrganization: string | null = null;

  constructor() {
    effect(() => {
      const id = this.projectId();
      const organizationId = this.project()?.organizationId ?? null;
      untracked(() => {
        if (id && id !== this.loadedProject) {
          this.loadedProject = id;
          this.reloadProjectData(id);
        }
        if (organizationId && organizationId !== this.loadedOrganization) {
          this.loadedOrganization = organizationId;
          this.loadDirectory(organizationId);
        }
      });
    });

    // Teams and their roles are only readable (and only needed) with the permission to manage people.
    effect(() => {
      const id = this.projectId();
      if (id && this.canManageMembers()) {
        untracked(() => {
          this.permissions.fetchTeamRoles(id);
          this.teams.fetchTeams();
        });
      }
    });
  }

  /** Name of a person; people the directory does not list (left the organization) get a neutral name. */
  personOf(userId: string): SettingsPerson {
    return (
      this.people().get(userId) ?? {
        userId,
        name: 'Persona de la organización',
        email: '',
        hasAvatar: false,
      }
    );
  }

  reloadProjectData(id: string | null = this.projectId()): void {
    if (!id) return;
    this.projects.fetchMembers(id);
    this.permissions.fetchScheme(id);
    if (this.canManageMembers()) this.permissions.fetchTeamRoles(id);
  }

  /** Reads the effective permissions again: a role or scheme change can move the caller's own rights. */
  refreshOwnPermissions(): void {
    this.current.reloadPermissions();
  }

  private loadDirectory(organizationId: string): void {
    this.directoryStatus.set('loading');
    this.orgApi
      .listMembers(organizationId)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (members) => {
          this.directory.set(members);
          this.directoryStatus.set('ready');
        },
        error: () => this.directoryStatus.set('error'),
      });
  }
}
