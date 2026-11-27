import { orgIdBySlug } from '../../lib/api.ts';
import { appPath } from '../../lib/paths.ts';
import { createProjectViaApi, findProjectByKey, randomProjectKey } from '../../lib/projects-api.ts';
import { expect, test } from '../../fixtures/test.ts';

const NEW_PROJECT_PATH = '/projects/new';

test.describe('asistente de nuevo proyecto', () => {
  test('member crea un proyecto Scrum y aterriza en el', async ({ as, api, data }) => {
    const member = await api('member');
    const acme = await orgIdBySlug(member, 'acme');
    const name = data.uniqueName('wiz');
    const key = randomProjectKey();

    const page = await as('member');
    try {
      await page.goto(appPath(NEW_PROJECT_PATH));
      await expect(page.getByTestId('template-scrum').locator('input')).toBeChecked();
      await page.getByTestId('wizard-next').click();

      await page.getByLabel('Nombre').fill(name);
      await page.getByLabel('Clave').fill(key);
      await expect(page.getByText('Clave disponible.')).toBeVisible();
      await page.getByTestId('wizard-next').click();

      await page.getByTestId('wizard-next').click();
      await expect(page.getByTestId('wizard-summary')).toContainText(name);
      await expect(page.getByTestId('wizard-summary')).toContainText('Scrum');
      await page.getByTestId('wizard-create').click();

      await expect(page).toHaveURL(new RegExp(`/projects/${key}(/|$)`));
      const created = await findProjectByKey(member, acme, key);
      expect(created, 'the project must exist in the API').not.toBeNull();
    } finally {
      const created = await findProjectByKey(member, acme, key);
      // Created through the UI: the fixture deletes it at the end of the test.
      if (created) data.trackProject('member', created);
    }
  });

  test('una clave repetida muestra el error y bloquea el paso', async ({ as, api, data }) => {
    const member = await api('member');
    const acme = await orgIdBySlug(member, 'acme');
    const existing = await createProjectViaApi(member, acme, data.uniqueName('taken'));
    data.trackProject('member', existing);

    const page = await as('member');
    await page.goto(appPath(NEW_PROJECT_PATH));
    await page.getByTestId('wizard-next').click();
    await page.getByLabel('Nombre').fill(data.uniqueName('wiz'));
    await page.getByLabel('Clave').fill(existing.key);

    await expect(page.getByText('Ya existe un proyecto con esta clave')).toBeVisible();
    await page.getByTestId('wizard-next').click();
    await expect(page.getByTestId('wizard-step-details')).toHaveAttribute('aria-current', 'step');
  });

  test('guest no puede crear proyectos desde el asistente', async ({ as }) => {
    const page = await as('guest');
    await page.goto(appPath(NEW_PROJECT_PATH));
    await expect(page.getByText('No puedes crear proyectos')).toBeVisible();
    await expect(page.getByTestId('wizard-next')).toHaveCount(0);
  });

  test('el asistente se puede recorrer con el teclado', async ({ as }) => {
    const page = await as('member');
    await page.goto(appPath(NEW_PROJECT_PATH));
    await page.getByTestId('template-kanban').locator('input').focus();
    await page.keyboard.press('Space');
    await expect(page.getByTestId('template-kanban').locator('input')).toBeChecked();
    await page.getByTestId('wizard-next').focus();
    await page.keyboard.press('Enter');
    await expect(page.getByRole('heading', { name: 'Detalles del proyecto' })).toBeVisible();
  });

  test('el asistente pasa la auditoria de maquetacion', async ({ as, audit }) => {
    const page = await as('member');
    await page.goto(appPath(NEW_PROJECT_PATH));
    await expect(page.locator('[data-page="project-new"][data-ready="true"]')).toBeVisible();
    await audit(page, { template: 'wide', name: 'project-wizard' });
  });
});
