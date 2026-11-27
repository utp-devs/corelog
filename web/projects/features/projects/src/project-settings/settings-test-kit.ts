import { Provider, computed, signal } from '@angular/core';
import {
  AuthStore,
  CurrentProjectStore,
  EffectiveRole,
  ORGANIZATION_ID_PROVIDER,
  OrganizationApiService,
  OrganizationMember,
  PermissionScheme,
  Project,
  ProjectApiService,
  ProjectMember,
  ProjectPermissionsApiService,
  ProjectTeamRole,
  SessionService,
  Team,
  TeamApiService,
} from '@corelog/core';
import { MessageService } from 'primeng/api';
import { Observable, of } from 'rxjs';
import { Mock, vi } from 'vitest';

export const ORG = 'org-1';
export const PROJECT_ID = 'p-1';
export const ME = 'u-me';

export const LEAD_PERMISSIONS = [
  'browse_project',
  'edit_project',
  'manage_members',
  'delete_project',
];
export const CONTRIBUTOR_PERMISSIONS = ['browse_project', 'create_issue'];

export function projectOf(overrides: Partial<Project> = {}): Project {
  return {
    id: PROJECT_ID,
    organizationId: ORG,
    key: 'CORE',
    name: 'CoreLog',
    description: 'Plataforma de trabajo',
    type: 'kanban',
    leadId: ME,
    visibility: 'open',
    template: 'kanban',
    color: 'palette-1',
    icon: '',
    defaultAssigneePolicy: 'unassigned',
    archivedAt: null,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

export function memberOf(
  userId: string,
  role: ProjectMember['role'],
  extra: Partial<ProjectMember> = {},
): ProjectMember {
  return {
    projectId: PROJECT_ID,
    userId,
    role,
    effectiveRole: role,
    viaTeam: false,
    createdAt: '2026-01-01T00:00:00Z',
    ...extra,
  };
}

export function orgMemberOf(userId: string, name: string): OrganizationMember {
  return {
    userId,
    name,
    displayName: name,
    email: `${userId}@example.com`,
    jobTitle: '',
    department: '',
    hasAvatar: false,
    role: 'member',
    joinedAt: '2026-01-01T00:00:00Z',
  };
}

export function teamOf(id: string, name: string): Team {
  return {
    id,
    organizationId: ORG,
    name,
    description: '',
    avatarColor: null,
    createdBy: null,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  };
}

export function schemeOf(grants: Record<string, string[]>, isDefault = true): PermissionScheme {
  return {
    projectId: PROJECT_ID,
    isDefault,
    grants: Object.entries(grants).map(([permission, holders]) => ({
      permission,
      holders: holders as PermissionScheme['grants'][number]['holders'],
    })),
  };
}

export const SAMPLE_GRANTS: Record<string, string[]> = {
  browse_project: ['lead', 'contributor', 'viewer'],
  create_issue: ['lead', 'contributor'],
  comment_issue: ['lead', 'contributor'],
  edit_own_issue: ['reporter'],
  manage_members: ['lead'],
  edit_project: ['lead'],
  delete_project: ['lead', 'org_admin'],
};

type Stub = (...args: never[]) => Observable<unknown>;

function spies(
  defaults: Record<string, Stub>,
  overrides: Record<string, Stub>,
): Record<string, Mock> {
  const result: Record<string, Mock> = {};
  for (const key of Object.keys(defaults)) result[key] = vi.fn(overrides[key] ?? defaults[key]);
  return result;
}

export interface SettingsOptions {
  readonly project?: Project;
  readonly role?: EffectiveRole;
  readonly permissions?: string[];
  readonly members?: ProjectMember[];
  readonly directory?: OrganizationMember[];
  readonly teams?: Team[];
  readonly teamRoles?: ProjectTeamRole[];
  readonly scheme?: PermissionScheme;
  readonly projectApi?: Record<string, Stub>;
  readonly permissionsApi?: Record<string, Stub>;
}

export interface SettingsEnvironment {
  readonly providers: Provider[];
  readonly projectApi: Record<string, Mock>;
  readonly permissionsApi: Record<string, Mock>;
  readonly current: {
    readonly project: ReturnType<typeof signal<Project | null>>;
    readonly setProject: Mock;
    readonly reloadPermissions: Mock;
  };
  readonly navigateByUrl: Mock;
}

/** Fake HTTP services and a fake current-project store for the settings pages; the real stores run on top. */
export function settingsEnvironment(options: SettingsOptions = {}): SettingsEnvironment {
  const project = signal<Project | null>(options.project ?? projectOf());
  const permissions = signal<string[] | null>(
    options.permissions ??
      (options.role === 'contributor' ? CONTRIBUTOR_PERMISSIONS : LEAD_PERMISSIONS),
  );
  const role = signal<EffectiveRole | null>(options.role ?? 'lead');

  const setProject = vi.fn((next: Project) => project.set(next));
  const reloadPermissions = vi.fn();
  const current = {
    project,
    projectId: computed(() => project()?.id ?? null),
    isArchived: computed(() => !!project()?.archivedAt),
    role: role.asReadonly(),
    permissionsLoaded: computed(() => permissions() !== null),
    can: (permission: string) => permissions()?.includes(permission) ?? false,
    setProject,
    reloadPermissions,
    reset: vi.fn(),
  };

  const members = options.members ?? [memberOf(ME, 'lead'), memberOf('u-2', 'contributor')];
  const projectApi = spies(
    {
      get: () => of(project()),
      update: () => of(project()),
      archive: () => of(projectOf({ archivedAt: '2026-02-01T00:00:00Z' })),
      restore: () => of(projectOf()),
      remove: () => of(undefined),
      listMembers: () => of(members),
      addMember: () => of(memberOf('u-3', 'contributor')),
      updateMember: () => of(memberOf('u-2', 'viewer')),
      removeMember: () => of(undefined),
    },
    (options.projectApi ?? {}) as Record<string, Stub>,
  );
  const scheme = options.scheme ?? schemeOf(SAMPLE_GRANTS);
  const permissionsApi = spies(
    {
      getScheme: () => of(scheme),
      putScheme: () => of({ ...scheme, isDefault: false }),
      resetScheme: () => of(scheme),
      me: () => of({ role: 'lead', permissions: LEAD_PERMISSIONS }),
      listTeamRoles: () => of(options.teamRoles ?? []),
      putTeamRole: () =>
        of({
          projectId: PROJECT_ID,
          teamId: 't-1',
          teamName: 'Soporte',
          role: 'contributor',
          createdAt: '',
        }),
      deleteTeamRole: () => of(undefined),
    },
    (options.permissionsApi ?? {}) as Record<string, Stub>,
  );
  const directory = options.directory ?? [
    orgMemberOf(ME, 'Ana Pérez'),
    orgMemberOf('u-2', 'Luis Gómez'),
    orgMemberOf('u-3', 'Marta Ruiz'),
  ];
  const navigateByUrl = vi.fn(() => Promise.resolve(true));

  const providers: Provider[] = [
    { provide: CurrentProjectStore, useValue: current },
    { provide: ProjectApiService, useValue: projectApi },
    { provide: ProjectPermissionsApiService, useValue: permissionsApi },
    { provide: OrganizationApiService, useValue: { listMembers: () => of(directory) } },
    { provide: TeamApiService, useValue: { list: () => of(options.teams ?? []) } },
    { provide: AuthStore, useValue: { user: signal({ id: ME }) } },
    { provide: SessionService, useValue: { isAuthenticated: signal(true).asReadonly() } },
    { provide: ORGANIZATION_ID_PROVIDER, useValue: () => ORG },
    MessageService,
  ];

  return { providers, projectApi, permissionsApi, current, navigateByUrl };
}
