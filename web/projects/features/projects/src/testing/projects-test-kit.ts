import { Provider, WritableSignal, signal } from '@angular/core';
import {
  AppRole,
  CalendarApiService,
  ORGANIZATION_ID_PROVIDER,
  AuthStore,
  OrganizationApiService,
  OrgRole,
  OrgWhoCan,
  OrganizationStore,
  Project,
  ProjectApiService,
  ProjectComponent,
  ProjectLabel,
  ProjectMember,
  SessionService,
  User,
} from '@corelog/core';
import { ActivatedRoute, Router, convertToParamMap } from '@angular/router';
import { MessageService } from 'primeng/api';
import { Observable, of } from 'rxjs';
import { Mock, vi } from 'vitest';

export const ORG_ID = 'o1';
export const ME_ID = 'u1';

type Stub = (...args: never[]) => Observable<unknown>;

/** Builds one spy per method: the override when given, the default answer otherwise. */
function spies(
  defaults: Record<string, Stub>,
  overrides: Record<string, Stub>,
): Record<string, Mock> {
  const result: Record<string, Mock> = {};
  for (const key of Object.keys(defaults)) {
    result[key] = vi.fn(overrides[key] ?? defaults[key]);
  }
  return result;
}

export function userOf(role: AppRole, id = ME_ID): User {
  return {
    id,
    name: 'Ana Pérez',
    email: 'ana@example.com',
    role,
    teamId: null,
    displayName: 'Ana',
    jobTitle: '',
    department: '',
    avatarKey: null,
    timezone: 'UTC',
    createdAt: '2026-01-01T00:00:00Z',
    lastLoginAt: null,
    emailVerifiedAt: null,
  };
}

export function projectOf(overrides: Partial<Project> = {}): Project {
  return {
    id: 'p-1',
    organizationId: ORG_ID,
    key: 'CORE',
    name: 'CoreLog',
    description: 'Plataforma de trabajo',
    type: 'kanban',
    leadId: ME_ID,
    visibility: 'open',
    archivedAt: null,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

export function memberOf(
  userId: string,
  role: ProjectMember['role'],
  projectId = 'p-1',
): ProjectMember {
  return { projectId, userId, role, createdAt: '2026-01-01T00:00:00Z' };
}

export function labelOf(id: string, name: string, color = 'blue'): ProjectLabel {
  return {
    id,
    projectId: 'p-1',
    name,
    color,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  };
}

export function componentOf(id: string, name: string, description = ''): ProjectComponent {
  return {
    id,
    projectId: 'p-1',
    name,
    description,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  };
}

export interface ProjectTestOptions {
  readonly role?: AppRole;
  readonly projectApi?: Partial<Record<keyof ProjectApiService, Stub>>;
  readonly orgApi?: Partial<Record<keyof OrganizationApiService, Stub>>;
  readonly calendarApi?: Partial<Record<keyof CalendarApiService, Stub>>;
  /** Role in the active organization and who its policy lets create projects. */
  readonly organization?: { readonly role: OrgRole | null; readonly policy: OrgWhoCan };
  /** Query params of the current URL; the router is replaced by a stub that records navigations. */
  readonly queryParams?: Record<string, string>;
}

export interface ProjectTestEnvironment {
  readonly providers: Provider[];
  readonly projectApi: Record<string, Mock>;
  readonly orgApi: Record<string, Mock>;
  readonly calendarApi: Record<string, Mock>;
  readonly user: WritableSignal<User | null>;
  readonly navigate: Mock;
}

/**
 * Providers for the projects screens: fake HTTP services, a stubbed session and the active
 * organization. The real stores run on top of the fakes.
 */
export function projectTestEnvironment(options: ProjectTestOptions = {}): ProjectTestEnvironment {
  const projectApi = spies(
    {
      list: () => of([]),
      get: () => of(projectOf()),
      create: () => of(projectOf()),
      update: () => of(projectOf()),
      remove: () => of(undefined),
      listMembers: () => of([]),
      addMember: () => of(memberOf('u-2', 'contributor')),
      updateMember: () => of(memberOf('u-2', 'lead')),
      removeMember: () => of(undefined),
      listLabels: () => of([]),
      createLabel: () => of(labelOf('l-1', 'Bug')),
      deleteLabel: () => of(undefined),
      listComponents: () => of([]),
      createComponent: () => of(componentOf('c-1', 'API')),
      deleteComponent: () => of(undefined),
    },
    (options.projectApi ?? {}) as Record<string, Stub>,
  );
  const orgApi = spies(
    { listMembers: () => of([]) },
    (options.orgApi ?? {}) as Record<string, Stub>,
  );
  const calendarApi = spies(
    { events: () => of([]) },
    (options.calendarApi ?? {}) as Record<string, Stub>,
  );
  const user = signal<User | null>(userOf(options.role ?? 'admin'));

  const providers: Provider[] = [
    { provide: ProjectApiService, useValue: projectApi },
    { provide: OrganizationApiService, useValue: orgApi },
    { provide: CalendarApiService, useValue: calendarApi },
    { provide: AuthStore, useValue: { user } },
    // Stores clear their data while no session is active; the tests run inside a signed-in session.
    { provide: SessionService, useValue: { isAuthenticated: signal(true).asReadonly() } },
    { provide: ORGANIZATION_ID_PROVIDER, useValue: () => ORG_ID },
    MessageService,
  ];

  if (options.organization) {
    const { role, policy } = options.organization;
    providers.push({
      provide: OrganizationStore,
      useValue: {
        currentRole: signal(role).asReadonly(),
        currentOrg: signal({ settings: { whoCanCreateProjects: policy } }).asReadonly(),
        currentOrgId: signal(ORG_ID).asReadonly(),
      },
    });
  }
  const navigate = vi.fn(() => Promise.resolve(true));
  if (options.queryParams) {
    providers.push(
      {
        provide: ActivatedRoute,
        useValue: { snapshot: { queryParamMap: convertToParamMap(options.queryParams) } },
      },
      { provide: Router, useValue: { navigate } },
    );
  }

  return { providers, projectApi, orgApi, calendarApi, user, navigate };
}
