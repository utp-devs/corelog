import { expect, test, type Page } from '@playwright/test'
import { mkdirSync, writeFileSync } from 'node:fs'
import path from 'node:path'

// Recorrido integral por rol: entra, visita cada seccion en escritorio y movil, guarda capturas
// y un informe (test-results/journeys/<rol>/report.json). Las credenciales vienen del entorno.
const password = process.env.E2E_USER_PASSWORD
if (!password) {
  throw new Error('Defina E2E_USER_PASSWORD (contrasena de las cuentas de prueba) antes de correr los e2e')
}

const projectId = process.env.E2E_PROJECT_ID ?? '30000000-0000-4000-8000-000000000001'
const issueId = process.env.E2E_ISSUE_ID ?? '90fc651c-9ebc-495c-b220-ae7dca585e78'
const outRoot = path.join(process.cwd(), 'test-results', 'journeys')

const roles = [
  { role: 'admin', email: 'admin@corelog.local', canSeeTeams: true },
  { role: 'agent', email: 'agent@corelog.local', canSeeTeams: false },
  { role: 'requester', email: 'ana.torres@corelog.test', canSeeTeams: false },
] as const

const routes = [
  { name: 'dashboard', path: '/dashboard' },
  { name: 'issues', path: '/issues' },
  { name: 'issue-new', path: '/issues/new' },
  { name: 'issue-detail', path: `/issues/${issueId}` },
  { name: 'projects', path: '/projects' },
  { name: 'project-board', path: `/projects/${projectId}/board` },
  { name: 'project-backlog', path: `/projects/${projectId}/backlog` },
  { name: 'project-sprints', path: `/projects/${projectId}/sprints` },
  { name: 'project-calendar', path: `/projects/${projectId}/calendar` },
  { name: 'project-permissions', path: `/projects/${projectId}/permisos` },
  { name: 'teams', path: '/teams' },
  { name: 'profile', path: '/profile' },
]

const viewports = [
  { name: 'desktop', width: 1440, height: 900 },
  { name: 'mobile', width: 390, height: 844 },
]

interface PageReport {
  route: string
  viewport: string
  finalUrl: string
  heading: string
  horizontalOverflowPx: number
  pageErrors: string[]
  consoleErrors: string[]
  screenshot: string
}

async function login(page: Page, email: string): Promise<void> {
  await page.goto('/login')
  await page.locator('input[type="email"]').fill(email)
  await page.locator('input[type="password"]').fill(password as string)
  await page.getByRole('button', { name: 'Iniciar sesión' }).click()
  await expect(page).toHaveURL(/\/(issues|dashboard)/)
}

async function visit(page: Page, route: { name: string; path: string }, viewport: { name: string }, dir: string): Promise<PageReport> {
  const pageErrors: string[] = []
  const consoleErrors: string[] = []
  const onPageError = (error: Error) => pageErrors.push(error.message)
  const onConsole = (message: import('@playwright/test').ConsoleMessage) => {
    if (message.type() === 'error') consoleErrors.push(message.text())
  }
  page.on('pageerror', onPageError)
  page.on('console', onConsole)

  await page.goto(route.path)
  await page.waitForLoadState('domcontentloaded')
  await page.waitForTimeout(1200)

  const headings = page.locator('h1, h2, h3')
  const heading = (await headings.count()) > 0 ? ((await headings.first().textContent({ timeout: 1000 }))?.trim() ?? '') : ''
  const horizontalOverflowPx = await page.evaluate(() => Math.max(0, document.documentElement.scrollWidth - window.innerWidth))
  const screenshot = path.join(dir, `${route.name}-${viewport.name}.png`)
  await page.screenshot({ path: screenshot, fullPage: true })

  page.off('pageerror', onPageError)
  page.off('console', onConsole)
  return {
    route: route.path,
    viewport: viewport.name,
    finalUrl: new URL(page.url()).pathname,
    heading,
    horizontalOverflowPx,
    pageErrors,
    consoleErrors,
    screenshot,
  }
}

for (const account of roles) {
  test(`recorrido completo como ${account.role}`, async ({ browser }) => {
    test.setTimeout(240_000)
    const dir = path.join(outRoot, account.role)
    mkdirSync(dir, { recursive: true })

    const reports: PageReport[] = []
    const navLabels: string[] = []

    for (const viewport of viewports) {
      const context = await browser.newContext({ viewport: { width: viewport.width, height: viewport.height } })
      const page = await context.newPage()
      await login(page, account.email)

      if (viewport.name === 'desktop') {
        await page.locator('aside nav a').first().waitFor({ timeout: 15_000 })
        navLabels.push(...(await page.locator('aside nav a, aside a[href^="/"]').allTextContents()).map((t) => t.trim()).filter(Boolean))
      }

      for (const route of routes) {
        reports.push(await visit(page, route, viewport, dir))
      }
      await context.close()
    }

    writeFileSync(path.join(dir, 'report.json'), JSON.stringify({ role: account.role, navLabels, reports }, null, 2))

    // Solo se afirma lo que el rol debe poder hacer; el resto queda en el informe para revision visual.
    const hasTeamsLink = navLabels.some((label) => label.includes('Equipos'))
    expect(hasTeamsLink, `menu de ${account.role} y la seccion Equipos`).toBe(account.canSeeTeams)

    const crashes = reports.filter((r) => r.pageErrors.length > 0)
    expect(crashes.map((r) => `${r.viewport} ${r.route}: ${r.pageErrors[0]}`), 'errores no capturados en la pagina').toEqual([])
  })
}
