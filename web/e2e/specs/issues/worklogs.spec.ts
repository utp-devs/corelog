import { expect, test } from '../../fixtures/test.ts';
import { appPath } from '../../lib/paths.ts';
import { projectWithIssue } from './issue-fixtures.ts';

test.describe('registro de tiempo', () => {
  test('un miembro registra tiempo, lo ve en la lista y en su hoja de horas, y lo elimina', async ({
    as,
    api,
    data,
  }) => {
    const { issue } = await projectWithIssue(api, data, 'wl');
    const page = await as('member');
    await page.goto(appPath(`/browse/${issue.key}`));

    await page.getByTestId('log-work-open').click();
    await page.getByRole('textbox', { name: /Tiempo dedicado/ }).fill('1h 30m');
    await expect(page.getByText('= 1h 30m')).toBeVisible();
    await page.getByRole('textbox', { name: 'Descripción' }).fill('Trabajo de prueba');
    await page.getByTestId('log-work-submit').click();
    await expect(page.getByText('Tiempo registrado').first()).toBeVisible();

    const item = page.getByTestId('worklog-item').first();
    await expect(item).toContainText('1h 30m');
    await expect(item).toContainText('Trabajo de prueba');
    await expect(page.getByTestId('time-spent')).toHaveText('1h 30m');

    // The timesheet of the week shows the issue with its time.
    await page.goto(appPath('/timesheet'));
    const row = page.getByTestId('timesheet-row').filter({ hasText: issue.key });
    await expect(row).toBeVisible();
    await expect(row).toContainText('1h 30m');
    await expect(page.getByTestId('timesheet-total')).toContainText('1h 30m');

    // Delete from the issue.
    await page.goto(appPath(`/browse/${issue.key}`));
    await page.getByRole('button', { name: 'Eliminar registro' }).first().click();
    await page.getByRole('button', { name: 'Eliminar', exact: true }).click();
    await expect(page.getByTestId('worklog-item')).toHaveCount(0);
  });

  test('un texto de duración inválido se explica y no se envía', async ({ as, api, data }) => {
    const { issue } = await projectWithIssue(api, data, 'wlbad');
    const page = await as('member');
    await page.goto(appPath(`/browse/${issue.key}`));
    await page.getByTestId('log-work-open').click();
    await page.getByRole('textbox', { name: /Tiempo dedicado/ }).fill('mucho');
    await expect(page.getByRole('alert').filter({ hasText: 'semanas, días, horas' })).toBeVisible();
    await page.getByTestId('log-work-submit').click();
    await expect(page.getByTestId('worklog-error')).toBeVisible();
  });

  test('quien lidera edita la estimación original y se actualiza el progreso', async ({
    as,
    api,
    data,
  }) => {
    const { issue } = await projectWithIssue(api, data, 'wlest');
    const page = await as('lead');
    await page.goto(appPath(`/browse/${issue.key}`));
    await page.getByRole('button', { name: /Estimación original/ }).click();
    await page.getByRole('textbox', { name: /Estimación original/ }).fill('2d');
    await page.keyboard.press('Enter');
    await expect(page.getByText('Estimación actualizada').first()).toBeVisible();
    await expect(page.getByTestId('issue-worklogs')).toContainText('2d');
  });
});
