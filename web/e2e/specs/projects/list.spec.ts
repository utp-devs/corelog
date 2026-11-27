import { orgIdBySlug } from '../../lib/api.ts';
import { appPath } from '../../lib/paths.ts';
import { createProjectViaApi } from '../../lib/projects-api.ts';
import { expect, test } from '../../fixtures/test.ts';

const PROJECTS_PATH = '/projects';

test.describe('lista de proyectos', () => {
  test('buscar, marcar favorito y filtrar por favoritos', async ({ as, api, data }) => {
    const member = await api('member');
    const acme = await orgIdBySlug(member, 'acme');
    const token = data.uniqueName('lst');
    const starred = await createProjectViaApi(member, acme, `${token} alfa`);
    data.trackProject('member', starred);
    const other = await createProjectViaApi(member, acme, `${token} beta`);
    data.trackProject('member', other);

    const page = await as('member');
    await page.goto(appPath(PROJECTS_PATH));

    await page.getByRole('textbox', { name: 'Buscar proyectos' }).fill(token);
    await expect(page.getByTestId(`project-row-${starred.key}`)).toBeVisible();
    await expect(page.getByTestId(`project-row-${other.key}`)).toBeVisible();

    await page.getByRole('button', { name: `Marcar como favorito ${starred.name}` }).click();
    await expect(
      page.getByRole('button', { name: `Quitar de favoritos ${starred.name}` }),
    ).toHaveAttribute('aria-pressed', 'true');

    await page.getByTestId('projects-starred-filter').click();
    await expect(page.getByTestId(`project-row-${starred.key}`)).toBeVisible();
    await expect(page.getByTestId(`project-row-${other.key}`)).toHaveCount(0);

    await page.getByTestId('projects-starred-filter').click();
    await page.getByRole('textbox', { name: 'Buscar proyectos' }).fill(`${token} beta`);
    await expect(page.getByTestId(`project-row-${other.key}`)).toBeVisible();
    await expect(page.getByTestId(`project-row-${starred.key}`)).toHaveCount(0);

    await page.getByRole('textbox', { name: 'Buscar proyectos' }).fill(`${token} sin resultados`);
    await expect(page.getByText('Ningún proyecto coincide')).toBeVisible();
    await page.getByTestId('projects-clear-filters').click();
    await expect(page.getByRole('textbox', { name: 'Buscar proyectos' })).toHaveValue('');
  });

  test('abrir un proyecto desde la lista lleva a su espacio', async ({ as, api, data }) => {
    const member = await api('member');
    const acme = await orgIdBySlug(member, 'acme');
    const project = await createProjectViaApi(member, acme, data.uniqueName('opn'));
    data.trackProject('member', project);

    const page = await as('member');
    await page.goto(appPath(PROJECTS_PATH));
    await page.getByRole('textbox', { name: 'Buscar proyectos' }).fill(project.name);
    await page.getByRole('link', { name: project.name }).click();
    await expect(page).toHaveURL(new RegExp(`/projects/${project.key}(/|$)`));
  });

  test('member ve el boton de crear y lleva al asistente', async ({ as }) => {
    const page = await as('member');
    await page.goto(appPath(PROJECTS_PATH));
    await page.getByTestId('projects-create').click();
    await expect(page).toHaveURL(/\/projects\/new$/);
  });

  test('guest ve la lista sin boton de crear', async ({ as }) => {
    const page = await as('guest');
    await page.goto(appPath(PROJECTS_PATH));
    await expect(page.getByRole('heading', { name: 'Proyectos' }).first()).toBeVisible();
    await expect(page.getByTestId('projects-create')).toHaveCount(0);
    await expect(page.getByTestId('projects-empty-create')).toHaveCount(0);
  });

  test('la lista pasa la auditoria de maquetacion', async ({ as, audit }) => {
    const page = await as('member');
    await page.goto(appPath(PROJECTS_PATH));
    await expect(page.locator('[data-page="projects"][data-ready="true"]')).toBeVisible();
    await audit(page, { template: 'wide', name: 'projects-list' });
  });
});
