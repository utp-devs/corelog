import { mkdirSync, writeFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

import type { APIRequestContext, Page } from '@playwright/test'

import {
  collectLayoutAudit,
  trackHygiene,
  type AuditResult,
  type ExpectedFailure,
  type LayoutTemplate,
} from '../lib/layout-audit.ts'
import { classifyViolations, loadKnownViolations, writeObserved } from '../lib/known-violations.ts'
import { waitForImages } from '../lib/wait.ts'
import { expect, test, type DataHelpers } from '../fixtures/test.ts'
import { ROLE_KEYS, roles, type RoleKey } from '../fixtures/roles.ts'
import { ruleIdOf } from '../lib/known-violations.ts'

// Smoke journey: every role visits every screen at each viewport project (desktop, tablet, mobile),
// the layout audit runs on each screen, screenshots are kept and a JSON report is written to
// test-results/journeys/<role>/<viewport>.json. Data (project, issue) is created through the API.

const REPORT_ROOT = fileURLToPath(new URL('../../test-results/journeys/', import.meta.url))

/** Teams link is part of the navigation only for admins (asserted on the desktop project). */
const canSeeTeams: Partial<Record<RoleKey, boolean>> = {
  admin: true,
  agent: false,
  requester: false,
  requesterAna: false,
}

interface Screen {
  readonly name: string
  readonly path: (ids: { projectId: string; issueId: string }) => string
  readonly template: LayoutTemplate
}

const screens: readonly Screen[] = [
  { name: 'dashboard', path: () => '/dashboard', template: 'fluid' },
  { name: 'issues', path: () => '/issues', template: 'wide' },
  { name: 'issue-new', path: () => '/issues/new', template: 'reading' },
  { name: 'issue-detail', path: (ids) => `/issues/${ids.issueId}`, template: 'reading' },
  { name: 'projects', path: () => '/projects', template: 'wide' },
  { name: 'project-board', path: (ids) => `/projects/${ids.projectId}/board`, template: 'fluid' },
  { name: 'project-backlog', path: (ids) => `/projects/${ids.projectId}/backlog`, template: 'wide' },
  { name: 'project-sprints', path: (ids) => `/projects/${ids.projectId}/sprints`, template: 'wide' },
  { name: 'project-calendar', path: (ids) => `/projects/${ids.projectId}/calendar`, template: 'wide' },
  { name: 'project-permissions', path: (ids) => `/projects/${ids.projectId}/permisos`, template: 'reading' },
  { name: 'teams', path: () => '/teams', template: 'wide' },
  { name: 'profile', path: () => '/profile', template: 'reading' },
]

/**
 * Non-admin roles may see forbidden (403) or missing (404) API responses on screens that belong to
 * projects or issues they cannot access: the forbidden state is the expected outcome for them.
 * Admin must reach every screen, so nothing is declared for admin.
 */
function forbiddenFor(key: RoleKey): ExpectedFailure[] {
  if (roles[key].appRole === 'admin') return []
  return [
    { status: 403, urlPattern: /\/api\/v1\// },
    { status: 404, urlPattern: /\/api\/v1\// },
  ]
}

interface ScreenReport {
  readonly screen: string
  readonly path: string
  readonly finalPath: string
  readonly screenshot: string
  readonly violations: string[]
  readonly metrics: AuditResult['metrics']
}

/** Fixed smoke data: short names (the audit's clipping check depends on name length). */
const SMOKE_PROJECT = { key: 'E2ESMOKE', name: 'E2E Smoke' } as const
const SMOKE_ISSUE = { title: 'E2E smoke issue' } as const

/** Reuses the first listed item matching `pick`, otherwise creates it. Keeps the data set stable across runs. */
async function findOrCreate<T extends { id: string }>(
  client: APIRequestContext,
  data: DataHelpers,
  pick: (list: T[]) => T | undefined,
  body: unknown,
  resource: 'projects' | 'issues',
  projectId?: string,
): Promise<T> {
  const query = resource === 'issues' && projectId ? { project_id: projectId } : undefined
  const response = await client.get(resource, query ? { params: query } : undefined)
  expect(response.ok(), `GET ${resource} should succeed`).toBe(true)
  const listed = ((await response.json()) as { data: T[] }).data
  const found = pick(listed)
  if (found) return found
  return data.create<T>('admin', resource, body)
}

async function visitScreen(
  page: Page,
  screen: Screen,
  ids: { projectId: string; issueId: string },
  shot: (page: Page, name: string) => Promise<string>,
  role: RoleKey,
  viewport: string,
  expectedFailures: readonly ExpectedFailure[],
): Promise<ScreenReport> {
  const hygiene = trackHygiene(page)
  hygiene.clear()
  const target = screen.path(ids)
  await page.goto(target)
  await waitForImages(page)

  const result = await collectLayoutAudit(page, {
    template: screen.template,
    name: `${role}-${screen.name}`,
    expectedFailures,
  })
  const screenshot = await shot(page, `${role}-${viewport}-${screen.name}`)
  return {
    screen: screen.name,
    path: target,
    finalPath: new URL(page.url()).pathname,
    screenshot,
    violations: result.violations,
    metrics: result.metrics,
  }
}

// Only the legacy accounts are walked until P2-S5..S9 adapt the UI to organizations; new roles join then.
for (const key of ROLE_KEYS.filter((k) => k in canSeeTeams)) {
  test(`recorre todas las pantallas como ${key} @smoke`, async ({ as, api, data, shots }, testInfo) => {
    test.setTimeout(300_000)
    const viewport = testInfo.project.name

    // Fixed, short data reused across runs (projects cannot be deleted through the API).
    const admin = await api('admin')
    const me = await admin.get('users/me')
    expect(me.ok(), 'GET users/me should succeed').toBe(true)
    const leadId = ((await me.json()) as { data: { id: string } }).data.id
    const project = await findOrCreate<{ id: string; key: string }>(admin, data, (list) => list.find((item) => item.key === SMOKE_PROJECT.key), {
      key: SMOKE_PROJECT.key,
      name: SMOKE_PROJECT.name,
      type: 'scrum',
      lead_id: leadId,
      visibility: 'team',
    }, 'projects')
    const issue = await findOrCreate<{ id: string; title: string }>(admin, data, (list) => list.find((item) => item.title === SMOKE_ISSUE.title), {
      title: SMOKE_ISSUE.title,
      description: 'Incidencia creada por el smoke test de pantallas.',
      priority: 'medium',
      project_id: project.id,
    }, 'issues', project.id)
    const ids = { projectId: project.id, issueId: issue.id }

    const page = await as(key)
    const reports: ScreenReport[] = []
    for (const screen of screens) {
      reports.push(await visitScreen(page, screen, ids, shots, key, viewport, forbiddenFor(key)))
    }

    const dir = path.join(REPORT_ROOT, key)
    mkdirSync(dir, { recursive: true })
    writeFileSync(path.join(dir, `${viewport}.json`), JSON.stringify({ role: key, viewport, reports }, null, 2))

    // Navigation check only where the sidebar is always expanded (desktop).
    if (viewport === 'desktop') {
      const hasTeamsLink = (await page.getByRole('navigation').getByRole('link', { name: 'Equipos' }).count()) > 0
      expect(hasTeamsLink, `Equipos in navigation for ${key} (${roles[key].appRole})`).toBe(canSeeTeams[key])
    }

    // Compare against the known-violations baseline, per screen and viewport.
    const baseline = loadKnownViolations()
    const summary: string[] = []
    const fresh: string[] = []
    const observed: Record<string, string[]> = {}
    for (const report of reports) {
      const screenKey = `${report.screen}|${viewport}`
      const split = classifyViolations(screenKey, report.violations, baseline)
      observed[screenKey] = [...new Set(report.violations.map(ruleIdOf))].sort()
      for (const known of split.known) summary.push(`known  ${screenKey} ${known}`)
      for (const rule of split.fixed) summary.push(`fixed  ${screenKey} ${rule} (remove from baseline)`)
      for (const violation of split.fresh) fresh.push(`${screenKey}: ${violation}`)
    }
    const summaryText = summary.length > 0 ? summary.join('\n') : 'none'
    console.log(`[smoke ${key} ${viewport}]\n${summaryText}`)
    await testInfo.attach('known-violations-summary.txt', { body: summaryText, contentType: 'text/plain' })

    if (process.env.E2E_UPDATE_KNOWN === '1') {
      // Update mode: record what was observed; global teardown merges all roles into the baseline.
      writeObserved(`${key}-${viewport}`, observed)
      return
    }
    expect(fresh, `new layout or hygiene violations for ${key} on ${viewport} (not in known-violations.json)`).toEqual([])
  })
}
