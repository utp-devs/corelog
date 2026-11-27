import type { Page } from '@playwright/test';

import { expect, test, type DataHelpers } from '../../fixtures/test.ts';
import { orgIdBySlug } from '../../lib/api.ts';
import { appPath } from '../../lib/paths.ts';

/** The sidebar of the project in view is an aside named after the project. */
const sidebarOf = (page: Page, projectName: string) =>
  page.getByRole('complementary', { name: projectName });

const projectsMenu = (page: Page) =>
  page.getByRole('navigation', { name: 'Principal' }).getByRole('button', { name: 'Proyectos' });

interface Pair {
  readonly scrum: { id: string; key: string; name: string };
  readonly kanban: { id: string; key: string; name: string };
}

/** One scrum and one kanban project of Acme, both open. */
async function createPair(data: DataHelpers, organizationId: string): Promise<Pair> {
  const suffix = Math.random().toString(36).slice(2, 6).toUpperCase();
  const make = async (template: 'scrum' | 'kanban', prefix: string) => {
    const project = await data.create<{ id: string; key: string; name: string }>(
      'owner',
      'projects',
      {
        organization_id: organizationId,
        key: `${template === 'scrum' ? 'S' : 'K'}${suffix}`,
        name: data.uniqueName(prefix),
        template,
        visibility: 'open',
      },
    );
    data.trackProject('owner', project);
    return project;
  };
  return { scrum: await make('scrum', 'ctx-scrum'), kanban: await make('kanban', 'ctx-kanban') };
}

test.describe('contexto de proyecto en la navegacion', () => {
  test.beforeEach(({}, testInfo) => {
    test.skip(
      (testInfo.project.use.viewport?.width ?? 0) < 1024,
      'the project sidebar and the top menus live in the drawer on narrow viewports',
    );
  });

  test('cambiar de proyecto actualiza la cabecera y las secciones segun la plantilla', async ({
    as,
    api,
    data,
  }) => {
    const owner = await api('owner');
    const { scrum, kanban } = await createPair(data, await orgIdBySlug(owner, 'acme'));
    const page = await as('owner');

    await page.goto(appPath(`/projects/${scrum.key}/board`));
    const scrumSidebar = sidebarOf(page, scrum.name);
    await expect(scrumSidebar.getByTestId('sidebar-project-name')).toHaveText(scrum.name);
    await expect(scrumSidebar.getByTestId('sidebar-project-key')).toHaveText(scrum.key);
    for (const label of ['Tablero', 'Backlog', 'Sprints', 'Calendario']) {
      await expect(scrumSidebar.getByRole('link', { name: label })).toBeVisible();
    }
    await expect(scrumSidebar.getByRole('link', { name: 'Tablero' })).toHaveAttribute(
      'aria-current',
      'page',
    );

    await page.goto(appPath(`/projects/${kanban.key}/board`));
    const kanbanSidebar = sidebarOf(page, kanban.name);
    await expect(kanbanSidebar.getByTestId('sidebar-project-name')).toHaveText(kanban.name);
    await expect(kanbanSidebar.getByTestId('sidebar-project-key')).toHaveText(kanban.key);
    await expect(kanbanSidebar.getByRole('link', { name: 'Tablero' })).toBeVisible();
    await expect(kanbanSidebar.getByRole('link', { name: 'Backlog' })).toHaveCount(0);
    await expect(kanbanSidebar.getByRole('link', { name: 'Sprints' })).toHaveCount(0);
  });

  test('una seccion del sidebar abre su pagina y queda marcada como activa', async ({
    as,
    api,
    data,
  }) => {
    const owner = await api('owner');
    const { scrum } = await createPair(data, await orgIdBySlug(owner, 'acme'));
    const page = await as('owner');
    await page.goto(appPath(`/projects/${scrum.key}/board`));
    const sidebar = sidebarOf(page, scrum.name);
    await sidebar.getByRole('link', { name: 'Calendario' }).click();
    await expect(page).toHaveURL(new RegExp(`/projects/${scrum.key}/calendar`));
    await expect(sidebar.getByRole('link', { name: 'Calendario' })).toHaveAttribute(
      'aria-current',
      'page',
    );
    await expect(sidebar.getByRole('link', { name: 'Tablero' })).not.toHaveAttribute(
      'aria-current',
      'page',
    );
  });

  test('destacar un proyecto lo muestra en Destacados y los visitados en Recientes', async ({
    as,
    api,
    data,
  }) => {
    const owner = await api('owner');
    const { scrum, kanban } = await createPair(data, await orgIdBySlug(owner, 'acme'));
    const page = await as('owner');

    // Visit both: the backend records the visit when the project is read.
    await page.goto(appPath(`/projects/${kanban.key}/board`));
    await expect(sidebarOf(page, kanban.name)).toBeVisible();
    await page.goto(appPath(`/projects/${scrum.key}/board`));
    const sidebar = sidebarOf(page, scrum.name);
    await expect(sidebar).toBeVisible();

    await sidebar.getByTestId('sidebar-project-star').click();
    await expect(sidebar.getByTestId('sidebar-project-star')).toHaveAttribute(
      'aria-pressed',
      'true',
    );

    await projectsMenu(page).click();
    const menu = page.getByRole('menu', { name: 'Proyectos' });
    const starred = menu.getByRole('group', { name: 'Destacados' });
    const recent = menu.getByRole('group', { name: 'Recientes' });
    await expect(starred.getByRole('menuitem', { name: new RegExp(scrum.name) })).toBeVisible();
    await expect(recent.getByRole('menuitem', { name: new RegExp(kanban.name) })).toBeVisible();
    // A starred project is not repeated among the recent ones.
    await expect(recent.getByRole('menuitem', { name: new RegExp(scrum.name) })).toHaveCount(0);
    await expect(menu.getByRole('menuitem', { name: 'Ver todos los proyectos' })).toBeVisible();

    // Opening an item goes to the project.
    await recent.getByRole('menuitem', { name: new RegExp(kanban.name) }).click();
    await expect(page).toHaveURL(new RegExp(`/projects/${kanban.key}`));

    // Un-starring removes it from the group.
    await page.goto(appPath(`/projects/${scrum.key}/board`));
    await sidebarOf(page, scrum.name).getByTestId('sidebar-project-star').click();
    await projectsMenu(page).click();
    await expect(
      page
        .getByRole('menu', { name: 'Proyectos' })
        .getByRole('group', { name: 'Destacados' })
        .getByRole('menuitem', { name: new RegExp(scrum.name) }),
    ).toHaveCount(0);
  });

  test('el menu de proyectos ofrece Crear proyecto solo a quien puede crearlos', async ({ as }) => {
    const admin = await as('admin');
    await admin.goto(appPath('/issues'));
    await projectsMenu(admin).click();
    await expect(
      admin.getByRole('menu', { name: 'Proyectos' }).getByRole('menuitem', {
        name: 'Crear proyecto',
      }),
    ).toBeVisible();

    const guest = await as('guest');
    await guest.goto(appPath('/issues'));
    await projectsMenu(guest).click();
    const menu = guest.getByRole('menu', { name: 'Proyectos' });
    await expect(menu.getByRole('menuitem', { name: 'Ver todos los proyectos' })).toBeVisible();
    await expect(menu.getByRole('menuitem', { name: 'Crear proyecto' })).toHaveCount(0);
  });

  test('un lector ve solo las secciones que su rol permite', async ({ as, api, data }) => {
    const owner = await api('owner');
    const { scrum } = await createPair(data, await orgIdBySlug(owner, 'acme'));
    const page = await as('viewer');
    await page.goto(appPath(`/projects/${scrum.key}/board`));
    const sidebar = sidebarOf(page, scrum.name);
    await expect(sidebar.getByRole('link', { name: 'Tablero' })).toBeVisible();
    // Project settings are for people who can administer the project.
    await expect(sidebar.getByRole('link', { name: 'Configuración' })).toHaveCount(0);
  });
});
