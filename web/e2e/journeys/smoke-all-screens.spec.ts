import { appPath } from '../lib/paths.ts';
import { mkdirSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import type { APIRequestContext, Page } from '@playwright/test';

import {
  collectLayoutAudit,
  trackHygiene,
  type AuditResult,
  type ExpectedFailure,
  type LayoutTemplate,
} from '../lib/layout-audit.ts';
import {
  classifyViolations,
  loadKnownViolations,
  ruleIdOf,
  writeObserved,
} from '../lib/known-violations.ts';
import { orgIdBySlug } from '../lib/api.ts';
import { waitForImages } from '../lib/wait.ts';
import { expect, test } from '../fixtures/test.ts';
import { ROLE_KEYS, roles, type RoleKey } from '../fixtures/roles.ts';

// Smoke journey: every role visits the screens that exist for its organization context, at each
// viewport project (desktop, tablet, mobile). The layout audit runs on each screen, screenshots are
// kept and a JSON report is written to test-results/journeys/<role>/<viewport>.json.
// Ids (project, issue, the caller's own user for /people/:id) are read through the API as the role.

const REPORT_ROOT = fileURLToPath(new URL('../../test-results/journeys/', import.meta.url));

/** Roles that may open the organization administration screens (RequireOrgAdmin). */
const ORG_ADMIN_ROLES: readonly RoleKey[] = ['owner', 'orgadmin', 'admin'];

interface Ids {
  readonly projectId: string;
  readonly issueId: string;
  readonly userId: string;
}

interface Screen {
  readonly name: string;
  readonly path: (ids: Ids) => string;
  readonly template: LayoutTemplate;
  /** Only for these roles; undefined = every role with an organization. */
  readonly onlyFor?: readonly RoleKey[];
}

const screens: readonly Screen[] = [
  { name: 'dashboard', path: () => '/dashboard', template: 'fluid' },
  { name: 'issues', path: () => '/issues', template: 'wide' },
  { name: 'issue-new', path: () => '/issues/new', template: 'reading' },
  { name: 'issue-detail', path: (ids) => `/browse/${ids.issueId}`, template: 'reading' },
  { name: 'projects', path: () => '/projects', template: 'wide' },
  { name: 'project-board', path: (ids) => `/projects/${ids.projectId}/board`, template: 'fluid' },
  {
    name: 'project-backlog',
    path: (ids) => `/projects/${ids.projectId}/backlog`,
    template: 'wide',
  },
  {
    name: 'project-sprints',
    path: (ids) => `/projects/${ids.projectId}/sprints`,
    template: 'wide',
  },
  {
    name: 'project-calendar',
    path: (ids) => `/projects/${ids.projectId}/calendar`,
    template: 'wide',
  },
  {
    name: 'project-settings',
    path: (ids) => `/projects/${ids.projectId}/settings/details`,
    template: 'wide',
  },
  { name: 'teams', path: () => '/teams', template: 'wide' },
  { name: 'people', path: () => '/people', template: 'wide' },
  { name: 'person', path: (ids) => `/people/${ids.userId}`, template: 'split' },
  { name: 'profile', path: () => '/profile', template: 'reading' },
  { name: 'profile-general', path: () => '/profile/general', template: 'reading' },
  {
    name: 'org-settings',
    path: () => '/organization/settings',
    template: 'reading',
    onlyFor: ORG_ADMIN_ROLES,
  },
  {
    name: 'org-members',
    path: () => '/organization/members',
    template: 'wide',
    onlyFor: ORG_ADMIN_ROLES,
  },
  {
    name: 'org-invitations',
    path: () => '/organization/invitations',
    template: 'wide',
    onlyFor: ORG_ADMIN_ROLES,
  },
];

/** Onboarding is the only screen of an account without an organization (newbie). */
const onboardingScreen: Screen = {
  name: 'onboarding',
  path: () => '/onboarding',
  template: 'reading',
};

/**
 * Non-admin roles may see forbidden (403) or missing (404) API responses on screens that belong to
 * projects or issues they cannot access. Admin must reach every screen, so nothing is declared for admin.
 */
function forbiddenFor(key: RoleKey): ExpectedFailure[] {
  if (roles[key].appRole === 'admin') return [];
  return [
    { status: 403, urlPattern: /\/api\/v1\// },
    { status: 404, urlPattern: /\/api\/v1\// },
  ];
}

interface ScreenReport {
  readonly screen: string;
  readonly path: string;
  readonly finalPath: string;
  readonly screenshot: string;
  readonly violations: string[];
  readonly metrics: AuditResult['metrics'];
}

async function visitScreen(
  page: Page,
  screen: Screen,
  ids: Ids,
  shot: (page: Page, name: string) => Promise<string>,
  role: RoleKey,
  expectedFailures: readonly ExpectedFailure[],
): Promise<ScreenReport> {
  const hygiene = trackHygiene(page);
  hygiene.clear();
  const target = screen.path(ids);
  await page.goto(appPath(target));
  await waitForImages(page);

  const result = await collectLayoutAudit(page, {
    template: screen.template,
    name: `${role}-${screen.name}`,
    expectedFailures,
  });
  const viewport = test.info().project.name;
  const screenshot = await shot(page, `${role}-${viewport}-${screen.name}`);
  return {
    screen: screen.name,
    path: target,
    finalPath: new URL(page.url()).pathname,
    screenshot,
    violations: result.violations,
    metrics: result.metrics,
  };
}

/**
 * First project and issue the role can see in its organization, and the role's own user id.
 * When the role sees none and may create one, a smoke project or issue is created (and deleted at the end).
 * Otherwise a missing id is used: the detail screens then show the not-found state, which the
 * forbidden expectations of non-admin roles already cover.
 */
async function smokeIds(client: APIRequestContext, orgId: string): Promise<Ids> {
  const missing = '00000000-0000-4000-8000-00000000dead';
  const me = (await (await client.get('users/me')).json()) as { data: { id: string } };

  const listProjects = async () =>
    (
      (await (await client.get('projects', { params: { organization_id: orgId } })).json()) as {
        data: Array<{ id: string }>;
      }
    ).data;
  let projects = await listProjects();
  if (projects.length === 0) {
    const created = await client.post('projects', {
      data: {
        organization_id: orgId,
        key: 'E2ESMK',
        name: `e2e-smoke-${Math.random().toString(36).slice(2, 8)}`,
        type: 'scrum',
        visibility: 'open',
      },
    });
    if (created.ok()) {
      const project = ((await created.json()) as { data: { id: string } }).data;
      projects = [project];
    }
  }
  const projectId = projects[0]?.id ?? missing;

  const listIssues = async () =>
    (
      (await (
        await client.get('issues', { params: { organization_id: orgId, project_id: projectId } })
      ).json()) as {
        data: Array<{ id: string; project_id?: string }>;
      }
    ).data;
  // Only issues of the chosen project: the list may still hold issues of projects that were deleted.
  let issues =
    projectId === missing
      ? []
      : (await listIssues()).filter((issue) => !issue.project_id || issue.project_id === projectId);
  if (issues.length === 0 && projectId !== missing) {
    const created = await client.post('issues', {
      data: {
        summary: 'e2e smoke issue',
        description: 'Incidencia del smoke test de pantallas.',
        priority: 'medium',
        project_id: projectId,
      },
    });
    if (created.ok()) {
      const issue = ((await created.json()) as { data: { id: string } }).data;
      issues = [issue];
    }
  }
  return { projectId, issueId: issues[0]?.id ?? missing, userId: me.data.id };
}

for (const key of ROLE_KEYS) {
  test(`recorre las pantallas de ${key} @smoke`, async ({ as, api, shots }, testInfo) => {
    test.setTimeout(420_000);
    const viewport = testInfo.project.name;
    const client = await api(key);
    const isNewbie = roles[key].org === null;

    const page = await as(key);
    const reports: ScreenReport[] = [];
    const applicable = isNewbie
      ? [onboardingScreen]
      : screens.filter((screen) => !screen.onlyFor || screen.onlyFor.includes(key));

    const ids = isNewbie
      ? {
          projectId: '00000000-0000-4000-8000-000000000000',
          issueId: '00000000-0000-4000-8000-000000000000',
          userId: '00000000-0000-4000-8000-000000000000',
        }
      : await smokeIds(client, await orgIdBySlug(client, roles[key].org ?? 'corelog'));

    for (const screen of applicable) {
      reports.push(await visitScreen(page, screen, ids, shots, key, forbiddenFor(key)));
    }

    const dir = path.join(REPORT_ROOT, key);
    mkdirSync(dir, { recursive: true });
    writeFileSync(
      path.join(dir, `${viewport}.json`),
      JSON.stringify({ role: key, viewport, reports }, null, 2),
    );

    // Navigation check where the top bar is visible on every viewport width (desktop project).
    if (viewport === 'desktop' && !isNewbie) {
      await page.goto(appPath('/issues'));
      await expect(
        page
          .getByRole('navigation', { name: 'Principal' })
          .getByRole('button', { name: 'Equipos' }),
        `Equipos in the top navigation for ${key}`,
      ).toBeVisible();
    }

    // Compare against the known-violations baseline, per screen and viewport.
    const baseline = loadKnownViolations();
    const summary: string[] = [];
    const fresh: string[] = [];
    const observed: Record<string, string[]> = {};
    for (const report of reports) {
      const screenKey = `${report.screen}|${viewport}`;
      const split = classifyViolations(screenKey, report.violations, baseline);
      observed[screenKey] = [...new Set(report.violations.map(ruleIdOf))].sort();
      for (const known of split.known) summary.push(`known  ${screenKey} ${known}`);
      for (const rule of split.fixed)
        summary.push(`fixed  ${screenKey} ${rule} (remove from baseline)`);
      for (const violation of split.fresh) fresh.push(`${key} ${screenKey}: ${violation}`);
    }
    const summaryText = summary.length > 0 ? summary.join('\n') : 'none';
    console.log(`[smoke ${key} ${viewport}]\n${summaryText}`);
    await testInfo.attach('known-violations-summary.txt', {
      body: summaryText,
      contentType: 'text/plain',
    });

    if (process.env.E2E_UPDATE_KNOWN === '1') {
      // Update mode: record what was observed; global teardown merges all roles into the baseline.
      writeObserved(`${key}-${viewport}`, observed);
      return;
    }
    expect(
      fresh,
      `new layout or hygiene violations for ${key} on ${viewport} (not in known-violations.json)`,
    ).toEqual([]);
  });
}
