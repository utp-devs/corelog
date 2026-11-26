import {
  expect,
  test as base,
  type APIRequestContext,
  type BrowserContext,
  type Page,
} from '@playwright/test'

import { readE2eEnv, storageStatePath } from '../lib/env.ts'
import {
  collectLayoutAudit,
  formatAuditFailure,
  captureAuditScreenshot,
  trackHygiene,
  type AuditOptions,
  type AuditResult,
} from '../lib/layout-audit.ts'
import { readStoredToken } from '../lib/storage-state.ts'
import { roles, type RoleKey } from './roles.ts'

/** Opens a new page logged in as the given role (storage state saved by global setup). */
export type OpenAsRole = (role: RoleKey) => Promise<Page>

/** Authenticated API client per role. Paths are relative to the API prefix, without a leading slash. */
export type ApiForRole = (role?: RoleKey) => Promise<APIRequestContext>

export interface DataHelpers {
  /** Unique, readable name: `e2e-<prefix>-<time>-<random>`. */
  uniqueName(prefix: string): string
  /** POST a resource as `role`, unwraps the `data` envelope and returns it. */
  create<T>(role: RoleKey, path: string, body: unknown): Promise<T>
  /** Registers a resource to be deleted (DELETE path) when the test ends. Only for endpoints that have DELETE. */
  track(role: RoleKey, deletePath: string): void
}

export type AuditFn = (page: Page, options: AuditOptions) => Promise<AuditResult>

/** Saves a named screenshot under test-results and returns its absolute path. */
export type ShotsFn = (page: Page, name: string) => Promise<string>

interface E2eFixtures {
  as: OpenAsRole
  api: ApiForRole
  data: DataHelpers
  audit: AuditFn
  shots: ShotsFn
}

function slug(value: string): string {
  return value.replace(/[^a-zA-Z0-9-_]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 80) || 'item'
}

const randomSuffix = (): string => Math.random().toString(36).slice(2, 8)

/**
 * Extended test: every spec imports `test` and `expect` from here (never from @playwright/test)
 * so roles, API access, data cleanup, layout audit and screenshots are available uniformly.
 */
export const test = base.extend<E2eFixtures>({
  as: async ({ browser }, provide, testInfo) => {
    const contexts: BrowserContext[] = []
    const open: OpenAsRole = async (role) => {
      const context = await browser.newContext({
        storageState: storageStatePath(role),
        baseURL: readE2eEnv().baseURL,
        viewport: testInfo.project.use.viewport ?? null,
      })
      contexts.push(context)
      const page = await context.newPage()
      trackHygiene(page)
      return page
    }
    await provide(open)
    for (const context of contexts) await context.close()
  },

  api: async ({ playwright }, provide) => {
    const env = readE2eEnv()
    const clients = new Map<RoleKey, APIRequestContext>()
    const forRole: ApiForRole = async (role = 'admin') => {
      const cached = clients.get(role)
      if (cached) return cached
      const token = readStoredToken(storageStatePath(role))
      const client = await playwright.request.newContext({
        // Trailing slash keeps the /api/v1 prefix when resolving relative paths.
        baseURL: `${env.apiURL}/`,
        extraHTTPHeaders: { Authorization: `Bearer ${token}`, Accept: 'application/json' },
      })
      clients.set(role, client)
      return client
    }
    await provide(forRole)
    for (const client of clients.values()) await client.dispose()
  },

  data: async ({ api }, provide) => {
    const tracked: Array<{ role: RoleKey; deletePath: string }> = []
    const helpers: DataHelpers = {
      uniqueName: (prefix) => `e2e-${slug(prefix)}-${Date.now().toString(36)}-${randomSuffix()}`,
      create: async <T,>(role: RoleKey, path: string, body: unknown): Promise<T> => {
        const client = await api(role)
        const response = await client.post(path, { data: body })
        expect(response.ok(), `POST ${path} should succeed (status ${response.status()})`).toBe(true)
        const json = (await response.json()) as { data: T }
        return json.data
      },
      track: (role, deletePath) => {
        tracked.push({ role, deletePath })
      },
    }
    await provide(helpers)
    for (const item of tracked.reverse()) {
      const client = await api(item.role)
      const response = await client.delete(item.deletePath)
      expect(response.ok(), `cleanup DELETE ${item.deletePath} should succeed`).toBe(true)
    }
  },

  audit: async ({ browserName }, provide, testInfo) => {
    const run: AuditFn = async (page, options) => {
      const result = await collectLayoutAudit(page, options)
      await testInfo.attach(`layout-audit-${slug(options.name ?? 'page')}.json`, {
        body: JSON.stringify({ browser: browserName, violations: result.violations, metrics: result.metrics }, null, 2),
        contentType: 'application/json',
      })
      if (result.violations.length > 0) {
        result.screenshot = await captureAuditScreenshot(page, options.name)
        expect(result.violations, formatAuditFailure(result, options.name)).toEqual([])
      }
      return result
    }
    await provide(run)
  },

  shots: async ({ browserName }, provide, testInfo) => {
    const take: ShotsFn = async (page, name) => {
      const file = testInfo.outputPath(`${slug(name)}-${browserName}.png`)
      await page.screenshot({ path: file, fullPage: true })
      return file
    }
    await provide(take)
  },
})

export { expect } from '@playwright/test'
export { roles }
