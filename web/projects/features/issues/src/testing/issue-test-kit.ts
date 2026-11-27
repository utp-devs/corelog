import { WritableSignal, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import {
  AttachmentApiService,
  AuthStore,
  IssueActivity,
  IssueAttachment,
  IssueComment,
  IssueLink,
  IssueWatcherApiService,
  IssueWatcherState,
  MEMBER_NAMES_SOURCE,
  MemberNamesSource,
  OrganizationStore,
  Project,
  ProjectApiService,
  ProjectMember,
  SavedFilter,
  SavedFilterApiService,
  Issue,
  IssueApiService,
  IssuePage,
  User,
  UserDirectoryApiService,
} from '@corelog/core';
import { provideCorelogTesting } from '@corelog/core/testing';
import { Observable, of } from 'rxjs';
import { Mock, vi } from 'vitest';

export const ORG_ID = 'org-1';
export const ME_ID = 'user-1';
export const ISSUE_ID = 'issue-1';
export const ISSUE_KEY = 'CORE-1';
export const PROJECT_ID = 'project-1';

type Impl = (...args: never[]) => Observable<unknown>;
type Fake<T> = { [K in keyof T]: Mock };

/** Builds a fake of a service: every method is a vi.fn answering with its default, unless overridden. */
function fakeOf<T extends object>(
  defaults: Record<keyof T, Impl>,
  overrides: Partial<Record<keyof T, Impl>>,
): Fake<T> {
  const fake = {} as Fake<T>;
  for (const key of Object.keys(defaults) as (keyof T)[]) {
    fake[key] = vi.fn(overrides[key] ?? defaults[key]);
  }
  return fake;
}

export function issueOf(overrides: Partial<Issue> = {}): Issue {
  return {
    id: ISSUE_ID,
    key: ISSUE_KEY,
    projectKey: 'CORE',
    summary: 'Error de prueba',
    description: 'Descripción de la incidencia',
    status: 'open',
    priority: 'medium',
    projectId: PROJECT_ID,
    issueType: 'bug',
    parentId: null,
    dueDate: null,
    storyPoints: null,
    rank: '00000001',
    closedAt: null,
    reporterId: 'user-9',
    reporterName: 'Ana',
    assigneeId: null,
    assigneeName: null,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-02T10:00:00Z',
    ...overrides,
  };
}

export function pageOf(items: Issue[], total = items.length): IssuePage {
  return { items, total, page: 1, pageSize: 20 };
}

export function userOf(overrides: Partial<User> = {}): User {
  return { id: ME_ID, name: 'Yo', email: 'yo@example.test', role: 'user', ...overrides } as User;
}

export function memberOf(
  userId: string,
  role: ProjectMember['role'] = 'contributor',
): ProjectMember {
  return { projectId: PROJECT_ID, userId, role, createdAt: '2026-01-01T00:00:00Z' };
}

export function projectOf(overrides: Partial<Project> = {}): Project {
  return {
    id: PROJECT_ID,
    organizationId: ORG_ID,
    key: 'DEM',
    name: 'Proyecto demo',
    description: '',
    type: 'scrum',
    leadId: ME_ID,
    visibility: 'private',
    archivedAt: null,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

export const COMMENT_AT = '2026-01-02T10:00:00Z';

export interface IssueTestSetup {
  readonly issueApi: Fake<IssueApiService>;
  readonly attachmentApi: Fake<AttachmentApiService>;
  readonly watcherApi: Fake<IssueWatcherApiService>;
  readonly projectApi: Fake<ProjectApiService>;
  readonly directoryApi: Fake<UserDirectoryApiService>;
  readonly savedFilterApi: Fake<SavedFilterApiService>;
  readonly auth: { readonly user: WritableSignal<User | null> };
  readonly organizations: { readonly currentOrgId: WritableSignal<string | null> };
}

export interface IssueTestOptions {
  readonly issueApi?: Partial<Record<keyof IssueApiService, Impl>>;
  readonly attachmentApi?: Partial<Record<keyof AttachmentApiService, Impl>>;
  readonly watcherApi?: Partial<Record<keyof IssueWatcherApiService, Impl>>;
  readonly projectApi?: Partial<Record<keyof ProjectApiService, Impl>>;
  readonly directoryApi?: Partial<Record<keyof UserDirectoryApiService, Impl>>;
  readonly savedFilterApi?: Partial<Record<keyof SavedFilterApiService, Impl>>;
  readonly memberSource?: MemberNamesSource;
  readonly user?: User | null;
  readonly orgId?: string | null;
}

/**
 * Registers the issue feature with fake HTTP services. The stores and the feature code are real;
 * only the network boundary is replaced. Call it inside beforeEach, before TestBed.createComponent.
 */
export function setupIssues(options: IssueTestOptions = {}): IssueTestSetup {
  const issueApi = fakeOf<IssueApiService>(
    {
      list: () => of(pageOf([])),
      getById: () => of(issueOf()),
      create: () => of(issueOf({ id: 'issue-new', key: 'CORE-2' })),
      updateStatus: () => of(issueOf()),
      assign: () => of(issueOf()),
      updateMetadata: () => of(issueOf()),
      updateDetails: () => of(issueOf()),
      updateRank: () => of(issueOf()),
      listComments: () => of([] as IssueComment[]),
      createComment: () => of({} as IssueComment),
      updateComment: (_issueId: string, id: string, body: string) =>
        of({
          id,
          issueId: _issueId,
          authorId: '',
          body,
          createdAt: COMMENT_AT,
          updatedAt: COMMENT_AT,
          deletedAt: null,
        } as IssueComment),
      deleteComment: () => of(undefined),
      listActivities: () => of([] as IssueActivity[]),
      listLinks: () => of([] as IssueLink[]),
      createLink: () => of({} as IssueLink),
      deleteLink: () => of(undefined),
    } as unknown as Record<keyof IssueApiService, Impl>,
    options.issueApi ?? {},
  );
  const attachmentApi = fakeOf<AttachmentApiService>(
    {
      list: () => of([] as IssueAttachment[]),
      upload: () => of({} as IssueAttachment),
      download: () => of(undefined),
    } as unknown as Record<keyof AttachmentApiService, Impl>,
    options.attachmentApi ?? {},
  );
  const watcherApi = fakeOf<IssueWatcherApiService>(
    {
      getState: () => of({ isWatching: false, count: 0 } as IssueWatcherState),
      watch: () => of({ isWatching: true, count: 1 } as IssueWatcherState),
      unwatch: () => of({ isWatching: false, count: 0 } as IssueWatcherState),
    } as unknown as Record<keyof IssueWatcherApiService, Impl>,
    options.watcherApi ?? {},
  );
  const projectApi = fakeOf<ProjectApiService>(
    {
      listMembers: () => of([] as ProjectMember[]),
      list: () => of([projectOf()]),
      get: () => of(projectOf()),
    } as unknown as Record<keyof ProjectApiService, Impl>,
    options.projectApi ?? {},
  );
  const directoryApi = fakeOf<UserDirectoryApiService>(
    { searchTeammates: () => of([] as User[]) } as unknown as Record<
      keyof UserDirectoryApiService,
      Impl
    >,
    options.directoryApi ?? {},
  );
  const savedFilterApi = fakeOf<SavedFilterApiService>(
    {
      list: () => of([] as SavedFilter[]),
      create: () => of({} as SavedFilter),
      delete: () => of(undefined),
    } as unknown as Record<keyof SavedFilterApiService, Impl>,
    options.savedFilterApi ?? {},
  );

  const auth = { user: signal<User | null>(options.user === undefined ? userOf() : options.user) };
  const organizations = {
    currentOrgId: signal<string | null>(options.orgId === undefined ? ORG_ID : options.orgId),
  };
  const memberSource: MemberNamesSource = options.memberSource ?? { listMembers: () => of([]) };

  TestBed.configureTestingModule({
    providers: [
      ...provideCorelogTesting(),
      { provide: IssueApiService, useValue: issueApi },
      { provide: AttachmentApiService, useValue: attachmentApi },
      { provide: IssueWatcherApiService, useValue: watcherApi },
      { provide: ProjectApiService, useValue: projectApi },
      { provide: UserDirectoryApiService, useValue: directoryApi },
      { provide: SavedFilterApiService, useValue: savedFilterApi },
      { provide: OrganizationStore, useValue: organizations },
      { provide: AuthStore, useValue: auth },
      { provide: MEMBER_NAMES_SOURCE, useValue: memberSource },
    ],
  });

  return {
    issueApi,
    attachmentApi,
    watcherApi,
    projectApi,
    directoryApi,
    savedFilterApi,
    auth,
    organizations,
  };
}
