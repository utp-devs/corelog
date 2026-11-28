import { expect, test } from '../../fixtures/test.ts';
import { appPath } from '../../lib/paths.ts';
import { projectWithIssue } from './issue-fixtures.ts';

test.describe('comentarios de la incidencia', () => {
  test('un miembro comenta mencionando a alguien, reacciona, responde y edita', async ({
    as,
    api,
    data,
  }) => {
    const { issue } = await projectWithIssue(api, data, 'cmt');
    const page = await as('member');
    await page.goto(appPath(`/browse/${issue.key}`));

    await page.getByRole('tab', { name: /Comentarios/ }).click();
    await page.getByTestId('comment-open').click();
    const editor = page.getByRole('textbox', { name: 'Nuevo comentario' });
    await editor.click();
    await page.keyboard.type('Revisa esto @lead');
    const suggestion = page.getByRole('option').filter({ hasText: /lead/i }).first();
    await expect(suggestion).toBeVisible();
    await suggestion.click();
    await page.keyboard.type(' por favor');
    await page.getByTestId('comment-submit').click();

    const comment = page.getByTestId('comment').filter({ hasText: 'Revisa esto' }).first();
    await expect(comment).toBeVisible();
    await expect(comment).toContainText('por favor');

    // Reaction toggles on and off.
    await comment.getByRole('button', { name: 'Añadir reacción' }).click();
    await page.getByRole('button', { name: 'Me gusta', exact: true }).click();
    const reaction = comment.getByRole('button', { name: /Me gusta: 1/ });
    await expect(reaction).toHaveAttribute('aria-pressed', 'true');
    await reaction.click();
    await expect(comment.getByRole('button', { name: /Me gusta: / })).toHaveCount(0);

    // Reply in the thread.
    await comment.getByRole('button', { name: 'Responder' }).click();
    await page.getByRole('textbox', { name: 'Responder al comentario' }).click();
    await page.keyboard.type('Respuesta e2e');
    await page.getByRole('button', { name: 'Responder', exact: true }).last().click();
    await expect(page.getByTestId('comment').filter({ hasText: 'Respuesta e2e' })).toBeVisible();

    // Edit leaves the marker.
    await comment.getByRole('button', { name: 'Editar' }).first().click();
    await page.getByRole('textbox', { name: 'Editar comentario' }).click();
    await page.keyboard.type(' (editado)');
    await page.getByRole('button', { name: 'Guardar' }).click();
    await expect(
      comment.getByRole('button', { name: 'Ver el historial de ediciones' }),
    ).toBeVisible();
  });

  test('el historial agrupa los cambios de una actualización', async ({ as, api, data }) => {
    const { owner, issue } = await projectWithIssue(api, data, 'hist');
    const changed = await owner.patch(`issues/${issue.id}`, {
      data: { story_points: 3, summary: 'Título nuevo' },
    });
    expect(changed.ok()).toBe(true);
    const page = await as('viewer');
    await page.goto(appPath(`/browse/${issue.key}`));
    await page.getByRole('tab', { name: 'Historial' }).click();
    const entry = page.getByTestId('issue-conversation').locator('li[data-kind="history"]').first();
    await expect(entry).toContainText('Puntos de historia');
    await expect(entry).toContainText('Resumen');
  });

  test('quien observa no ve el compositor de comentarios', async ({ as, api, data }) => {
    const { issue } = await projectWithIssue(api, data, 'cmtro');
    const page = await as('viewer');
    await page.goto(appPath(`/browse/${issue.key}`));
    await expect(page.getByTestId('issue-conversation')).toBeVisible();
    await expect(page.getByTestId('comment-open')).toHaveCount(0);
  });
});
