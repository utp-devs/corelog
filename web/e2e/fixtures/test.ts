import {
  expect,
  test as base,
  type APIRequestContext,
  type BrowserContext,
  type Page,
} from '@playwright/test';

import { readE2eEnv, storageStatePath } from '../lib/env.ts';
import {
  collectLayoutAudit,
  formatAuditFailure,
  captureAuditScreenshot,
  trackHygiene,
  type AuditOptions,
  type AuditResult,
} from '../lib/layout-audit.ts';
import { THEME_STORAGE_KEY, readStoredToken } from '../lib/storage-state.ts';
import { roles, type RoleKey } from './roles.ts';

/** Opens a new page logged in as the given role (storage state saved by global setup). */
export type OpenAsRole = (role: RoleKey) => Promise<Page>;

/** Authenticated API client per role. Paths are relative to the API prefix, without a leading slash. */
export type ApiForRole = (role?: RoleKey) => Promise<APIRequestContext>;

export type InviteRole = 'owner' | 'admin' | 'member' | 'guest';

export interface RegisteredUser {
  readonly email: string;
  readonly password: string;
  readonly token: string;
  readonly id: string;
  readonly name: string;
}

export interface DataHelpers {
  /** Short unique name: `e2e-<prefix>-<6 random chars>`. */
  uniqueName(prefix: string): string;
  /** POST a resource as `role`, unwraps the `data` envelope and returns it. */
  create<T>(role: RoleKey, path: string, body: unknown): Promise<T>;
  /** Registers a resource to be deleted (DELETE path) when the test ends. Only for endpoints that have DELETE. */
  track(role: RoleKey, deletePath: string, body?: unknown): void;
  /** Registers a project (created by the test through the UI or the API) to be deleted with its name as confirmation. */
  trackProject(role: RoleKey, project: { id: string; name: string }): void;
  /** Creates an organization as `role` (it becomes the owner). Deleted at the end of the test. */
  createOrg(role: RoleKey, prefix?: string): Promise<{ id: string; slug: string; name: string }>;
  /** Creates a team in the organization as `role`. Deleted at the end of the test. */
  createTeam(
    role: RoleKey,
    organizationId: string,
    prefix?: string,
  ): Promise<{ id: string; name: string }>;
  /** Creates an open Scrum project in the organization as `role`. Deleted at the end of the test. */
  createProject(
    role: RoleKey,
    organizationId: string,
    prefix?: string,
  ): Promise<{ id: string; key: string; name: string }>;
  /** Invites an email as `role` (owner or admin of the organization). Revoked at the end of the test. */
  inviteUser(
    role: RoleKey,
    organizationId: string,
    email: string,
    inviteRole?: InviteRole,
  ): Promise<{ id: string; token: string; acceptUrl: string }>;
  /**
   * Registers a brand-new account through the API with a unique email and logs it in.
   * Deactivated (logical delete, DELETE is not offered) at the end of the test.
   */
  registerUser(prefix?: string): Promise<RegisteredUser>;
}

export type AuditFn = (page: Page, options: AuditOptions) => Promise<AuditResult>;

/** Saves a named screenshot under test-results and returns its absolute path. */
export type ShotsFn = (page: Page, name: string) => Promise<string>;

interface E2eFixtures {
  as: OpenAsRole;
  api: ApiForRole;
  data: DataHelpers;
  audit: AuditFn;
  shots: ShotsFn;
}

function slug(value: string): string {
  return (
    value
      .replace(/[^a-zA-Z0-9-_]+/g, '-')
      .replace(/^-+|-+$/g, '')
      .slice(0, 40) || 'item'
  );
}

/** Random lowercase alphanumeric string of the given length. */
function randomSuffix(length = 6): string {
  let out = '';
  while (out.length < length) out += Math.random().toString(36).slice(2);
  return out.slice(0, length);
}

/**
 * Extended test: every spec imports `test` and `expect` from here (never from @playwright/test)
 * so roles, API access, data cleanup, layout audit and screenshots are available uniformly.
 */
export const test = base.extend<E2eFixtures>({
  as: async ({ browser }, provide, testInfo) => {
    const contexts: BrowserContext[] = [];
    const open: OpenAsRole = async (role) => {
      const context = await browser.newContext({
        storageState: storageStatePath(role),
        baseURL: readE2eEnv().baseURL,
        viewport: testInfo.project.use.viewport ?? null,
        colorScheme: testInfo.project.use.colorScheme,
      });
      // Dark projects (name ends in -dark) store the dark theme the same way the theme switch does.
      if (testInfo.project.name.endsWith('-dark')) {
        await context.addInitScript(
          (key: string) => window.localStorage.setItem(key, 'dark'),
          THEME_STORAGE_KEY,
        );
      }
      contexts.push(context);
      const page = await context.newPage();
      trackHygiene(page);
      return page;
    };
    await provide(open);
    for (const context of contexts) await context.close();
  },

  api: async ({ playwright }, provide) => {
    const env = readE2eEnv();
    const clients = new Map<RoleKey, APIRequestContext>();
    const forRole: ApiForRole = async (role = 'admin') => {
      const cached = clients.get(role);
      if (cached) return cached;
      const token = readStoredToken(storageStatePath(role));
      const client = await playwright.request.newContext({
        // Trailing slash keeps the /api/v1 prefix when resolving relative paths.
        baseURL: `${env.apiURL}/`,
        extraHTTPHeaders: { Authorization: `Bearer ${token}`, Accept: 'application/json' },
      });
      clients.set(role, client);
      return client;
    };
    await provide(forRole);
    for (const client of clients.values()) await client.dispose();
  },

  data: async ({ api, playwright }, provide) => {
    const env = readE2eEnv();
    const tracked: Array<{ role: RoleKey; deletePath: string; body?: unknown }> = [];
    const users: Array<{ email: string; password: string; token: string }> = [];

    const unwrap = async <T>(
      response: Awaited<ReturnType<APIRequestContext['post']>>,
      label: string,
    ): Promise<T> => {
      const text = await response.text();
      expect(
        response.ok(),
        `${label} should succeed (status ${response.status()}: ${text.slice(0, 300)})`,
      ).toBe(true);
      return (JSON.parse(text) as { data: T }).data;
    };

    /**
     * Body of the cleanup DELETE. A project is deleted with its current name (a test may have renamed it);
     * null means the project is already gone.
     */
    const currentConfirmation = async (
      client: APIRequestContext,
      item: { deletePath: string; body?: unknown },
    ): Promise<unknown> => {
      if (!item.body || !/^projects\/[^/]+$/.test(item.deletePath)) return item.body;
      const current = await client.get(item.deletePath);
      if (!current.ok()) return null;
      const name = ((await current.json()) as { data: { name: string } }).data.name;
      return { confirmation: name };
    };

    const helpers: DataHelpers = {
      uniqueName: (prefix) => `e2e-${slug(prefix)}-${randomSuffix()}`,
      create: async <T>(role: RoleKey, path: string, body: unknown): Promise<T> => {
        const client = await api(role);
        return unwrap<T>(await client.post(path, { data: body }), `POST ${path}`);
      },
      track: (role, deletePath, body) => {
        tracked.push({ role, deletePath, body });
      },
      trackProject: (role, project) => {
        tracked.push({
          role,
          deletePath: `projects/${project.id}`,
          body: { confirmation: project.name },
        });
      },
      createOrg: async (role, prefix = 'org') => {
        // Slugs: 3 to 40 lowercase letters, digits and hyphens.
        const org = await helpers.create<{ id: string; slug: string; name: string }>(
          role,
          'organizations',
          {
            name: helpers.uniqueName(prefix),
            slug: `e2e-${randomSuffix(8)}`,
          },
        );
        helpers.track(role, `organizations/${org.id}`);
        return org;
      },
      createTeam: async (role, organizationId, prefix = 'team') => {
        const team = await helpers.create<{ id: string; name: string }>(role, 'teams', {
          organization_id: organizationId,
          name: helpers.uniqueName(prefix),
        });
        helpers.track(role, `teams/${team.id}`);
        return team;
      },
      createProject: async (role, organizationId, prefix = 'proj') => {
        const key = `E${randomSuffix(4)}`.toUpperCase();
        const project = await helpers.create<{ id: string; key: string; name: string }>(
          role,
          'projects',
          {
            organization_id: organizationId,
            key,
            name: helpers.uniqueName(prefix),
            type: 'scrum',
            visibility: 'open',
          },
        );
        helpers.trackProject(role, project);
        return project;
      },
      inviteUser: async (role, organizationId, email, inviteRole = 'member') => {
        const client = await api(role);
        const body = await unwrap<{
          invitations: Array<{ id: string; token: string; accept_url: string }>;
        }>(
          await client.post(`organizations/${organizationId}/invitations`, {
            data: { invitations: [{ email, role: inviteRole }] },
          }),
          'POST invitations',
        );
        const created = body.invitations[0];
        expect(created, 'the invitation must be returned').toBeTruthy();
        helpers.track(role, `organizations/${organizationId}/invitations/${created.id}`);
        return { id: created.id, token: created.token, acceptUrl: created.accept_url };
      },
      registerUser: async (prefix = 'user') => {
        const email = `e2e.${slug(prefix)}.${randomSuffix(8)}@corelog.test`;
        const name = `Usuario ${randomSuffix(6)}`;
        const password = readE2eEnv().password ?? '';
        const anonymous = await playwright.request.newContext({ baseURL: `${env.apiURL}/` });
        try {
          const registered = await anonymous.post('auth/register', {
            data: { name, email, password },
          });
          expect(registered.status(), 'POST auth/register should return 201').toBe(201);
          const login = await anonymous.post('auth/login', { data: { email, password } });
          expect(login.status(), 'login of the new account should return 200').toBe(200);
          const { data } = (await login.json()) as { data: { token: string } };
          const me = await anonymous.get('users/me', {
            headers: { Authorization: `Bearer ${data.token}` },
          });
          const { data: profile } = (await me.json()) as { data: { id: string } };
          users.push({ email, password, token: data.token });
          return { email, password, token: data.token, id: profile.id, name };
        } finally {
          await anonymous.dispose();
        }
      },
    };

    await provide(helpers);

    // Cleanup in reverse creation order. Items already gone (404) are fine.
    for (const item of tracked.reverse()) {
      const client = await api(item.role);
      const body = await currentConfirmation(client, item);
      if (body === null) continue;
      const response = await client.delete(item.deletePath, body ? { data: body } : {});
      // 409: an accepted or revoked invitation cannot be deleted again; it is already closed.
      expect(
        [200, 204, 404, 409],
        `cleanup DELETE ${item.deletePath} (status ${response.status()})`,
      ).toContain(response.status());
    }
    for (const user of users.reverse()) {
      const anonymous = await playwright.request.newContext({ baseURL: `${env.apiURL}/` });
      try {
        await anonymous.post('users/me/deactivate', {
          headers: { Authorization: `Bearer ${user.token}` },
          data: { password: user.password },
        });
      } finally {
        await anonymous.dispose();
      }
    }
  },

  audit: async ({ browserName }, provide, testInfo) => {
    const run: AuditFn = async (page, options) => {
      const result = await collectLayoutAudit(page, options);
      await testInfo.attach(`layout-audit-${slug(options.name ?? 'page')}.json`, {
        body: JSON.stringify(
          { browser: browserName, violations: result.violations, metrics: result.metrics },
          null,
          2,
        ),
        contentType: 'application/json',
      });
      if (result.violations.length > 0) {
        result.screenshot = await captureAuditScreenshot(page, options.name);
        expect(result.violations, formatAuditFailure(result, options.name)).toEqual([]);
      }
      return result;
    };
    await provide(run);
  },

  shots: async ({ browserName }, provide, testInfo) => {
    const take: ShotsFn = async (page, name) => {
      const file = testInfo.outputPath(`${slug(name)}-${browserName}.png`);
      await page.screenshot({ path: file, fullPage: true });
      return file;
    };
    await provide(take);
  },
});

export { expect } from '@playwright/test';
export { roles };
