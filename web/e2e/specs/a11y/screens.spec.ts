import type { Page } from '@playwright/test';

import { expect, test } from '../../fixtures/test.ts';
import { orgIdBySlug } from '../../lib/api.ts';
import { axeSummary } from '../../lib/axe.ts';
import { readE2eEnv } from '../../lib/env.ts';
import { appPath } from '../../lib/paths.ts';
import { THEME_STORAGE_KEY } from '../../lib/storage-state.ts';
import { waitForImages, waitForReady } from '../../lib/wait.ts';
import { createIssueViaApi } from '../../lib/workflow-api.ts';

// Axe (WCAG 2.1 A/AA + best practices, which includes rendered color contrast) on every screen of the
// three apps. The suite runs on each viewport and theme project. All findings of a group are collected
// and reported together so one run lists everything that is left to fix.

interface Screen {
  readonly name: string;
  readonly path: string;
}

/** Settles a screen: ready marker, images, no running animation. */
async function settle(page: Page): Promise<void> {
  await waitForReady(page);
  await waitForImages(page);
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.waitForTimeout(300);
}

async function scanAll(page: Page, screens: readonly Screen[]): Promise<string> {
  const failures: string[] = [];
  for (const screen of screens) {
    await page.goto(appPath(screen.path));
    await settle(page);
    const summary = await axeSummary(page, { name: `${screen.name} (${screen.path})` });
    if (summary) failures.push(summary);
  }
  return failures.join('\n');
}

test.describe('accesibilidad: pantallas', () => {
  test('pantallas publicas del portal', async ({ browser }, testInfo) => {
    const context = await browser.newContext({
      baseURL: readE2eEnv().baseURL,
      viewport: testInfo.project.use.viewport ?? null,
      colorScheme: testInfo.project.use.colorScheme,
    });
    if (testInfo.project.name.endsWith('-dark')) {
      await context.addInitScript(
        (key: string) => window.localStorage.setItem(key, 'dark'),
        THEME_STORAGE_KEY,
      );
    }
    const page = await context.newPage();
    const failures = await scanAll(page, [
      { name: 'login', path: '/login' },
      { name: 'registro', path: '/register' },
      { name: 'recuperar contrasena', path: '/forgot-password' },
      { name: 'restablecer contrasena', path: '/reset-password?token=e2e-invalid-token' },
      { name: 'verificar correo', path: '/verify-email?token=e2e-invalid-token' },
      { name: 'invitacion', path: '/invite/e2e-invalid-token' },
      { name: 'no encontrada', path: '/e2e-missing-page' },
    ]);
    await context.close();
    expect(failures, failures).toBe('');
  });

  test('onboarding sin organizacion', async ({ as }) => {
    const page = await as('newbie');
    const failures = await scanAll(page, [{ name: 'onboarding', path: '/onboarding' }]);
    expect(failures, failures).toBe('');
  });

  test('pantallas del espacio de trabajo', async ({ as, api, data }) => {
    test.setTimeout(300_000);
    const admin = await api('admin');
    const org = await orgIdBySlug(admin, 'corelog');
    const project = await data.createProject('admin', org, 'axe');
    const issue = await createIssueViaApi(admin, project.id, 'Incidencia de accesibilidad');
    const me = (await (await admin.get('users/me')).json()) as { data: { id: string } };
    const team = await data.createTeam('admin', org, 'axe');
    const page = await as('admin');
    const key = project.key;
    const settings = [
      'details',
      'access',
      'roles',
      'permissions',
      'issue-types',
      'versions',
      'components',
      'fields',
      'workflow',
      'notifications',
      'automation',
      'import',
      'export',
      'danger',
    ];
    const profile = ['general', 'security', 'notifications', 'appearance', 'privacy', 'tokens'];
    const screens: Screen[] = [
      { name: 'panel', path: '/dashboard' },
      { name: 'paneles', path: '/dashboards' },
      { name: 'incidencias', path: '/issues' },
      { name: 'incidencia nueva', path: '/issues/new' },
      { name: 'incidencia', path: `/browse/${issue.key}` },
      { name: 'filtros', path: '/filters' },
      { name: 'proyectos', path: '/projects' },
      { name: 'proyecto nuevo', path: '/projects/new' },
      { name: 'tablero', path: `/projects/${key}/board` },
      { name: 'backlog', path: `/projects/${key}/backlog` },
      { name: 'sprints', path: `/projects/${key}/sprints` },
      { name: 'calendario de proyecto', path: `/projects/${key}/calendar` },
      { name: 'informes de proyecto', path: `/projects/${key}/reports` },
      { name: 'informe burndown', path: `/projects/${key}/reports/burndown` },
      { name: 'hoja de ruta', path: `/projects/${key}/roadmap` },
      ...settings.map((s) => ({ name: `ajustes ${s}`, path: `/projects/${key}/settings/${s}` })),
      { name: 'equipos', path: '/teams' },
      { name: 'equipo', path: `/teams/${team.id}` },
      { name: 'notas', path: '/notes' },
      { name: 'calendario', path: '/calendar' },
      { name: 'horas', path: '/timesheet' },
      { name: 'personas', path: '/people' },
      { name: 'persona', path: `/people/${me.data.id}` },
      ...profile.map((s) => ({ name: `perfil ${s}`, path: `/profile/${s}` })),
    ];
    const failures = await scanAll(page, screens);
    expect(failures, failures).toBe('');
  });

  test('pantallas de administracion de la organizacion', async ({ as }) => {
    test.setTimeout(240_000);
    const page = await as('admin');
    const sections = [
      'overview',
      'settings',
      'members',
      'catalogs',
      'custom-fields',
      'statuses',
      'invitations',
      'legal',
      'notifications',
      'audit',
      'webhooks',
      'platform-audit',
      'email-outbox',
      'jobs',
      '403',
    ];
    const failures = await scanAll(
      page,
      sections.map((s) => ({ name: `org ${s}`, path: `/organization/${s}` })),
    );
    expect(failures, failures).toBe('');
  });
});
