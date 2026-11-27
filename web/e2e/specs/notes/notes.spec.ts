import type { APIRequestContext, Page } from '@playwright/test';

import { expect, test, type DataHelpers } from '../../fixtures/test.ts';
import { orgIdBySlug } from '../../lib/api.ts';
import { appPath } from '../../lib/paths.ts';

interface ApiNote {
  readonly id: string;
  readonly title: string;
  readonly pinned: boolean;
  readonly tags: string[];
  readonly issue_id: string | null;
}

/** Creates a note through the API and registers it to be deleted when the test ends. */
async function createNote(
  data: DataHelpers,
  body: Record<string, unknown>,
  role: 'member' | 'viewer' = 'member',
): Promise<ApiNote> {
  const note = await data.create<ApiNote>(role, 'notes', body);
  data.track(role, `notes/${note.id}`);
  return note;
}

async function notesOf(client: APIRequestContext, query: string): Promise<ApiNote[]> {
  const response = await client.get('notes', { params: { q: query, limit: 100 } });
  return ((await response.json()) as { data: ApiNote[] }).data;
}

/** Deletes every note whose title starts with the marker (notes made through the UI are not tracked). */
async function purge(client: APIRequestContext, marker: string): Promise<void> {
  for (const note of await notesOf(client, marker)) await client.delete(`notes/${note.id}`);
}

const cardOf = (page: Page, title: string) =>
  page.getByTestId('note-card').filter({ hasText: title });

test.describe('mis apuntes', () => {
  test('crea una nota desde el compositor con etiqueta y color', async ({ as, api, data }) => {
    const marker = data.uniqueName('nota');
    const member = await api('member');
    try {
      const page = await as('member');
      await page.goto(appPath('/notes'));
      await expect(page.getByText('Solo tú puedes ver tus notas.').first()).toBeVisible();

      await page.getByRole('button', { name: 'Escribe una nota…' }).click();
      await page.getByLabel('Título de la nota (opcional)').fill(marker);
      await page.getByLabel('Texto de la nota').fill('Revisar **el plan** del sprint');
      await page.getByLabel('Etiquetas', { exact: true }).fill('E2E-Plan');
      await page.getByLabel('Etiquetas', { exact: true }).press('Enter');
      await page.getByRole('radio', { name: 'Verde' }).click();
      await page.getByRole('button', { name: 'Guardar nota' }).click();

      const card = cardOf(page, marker);
      await expect(card).toBeVisible();
      await expect(card.locator('strong')).toHaveText('el plan');
      await expect(card).toHaveAttribute('data-color', 'green');

      const [saved] = await notesOf(member, marker);
      expect(saved.tags).toEqual(['e2e-plan']);
    } finally {
      await purge(member, marker);
    }
  });

  test('una nota vacía no se guarda y pide un título o texto', async ({ as }) => {
    const page = await as('member');
    await page.goto(appPath('/notes'));
    await page.getByRole('button', { name: 'Escribe una nota…' }).click();
    await page.getByRole('button', { name: 'Guardar nota' }).click();
    await expect(page.getByRole('alert').filter({ hasText: 'Escribe un título' })).toBeVisible();
  });

  test('fija una nota y pasa a la sección Fijadas', async ({ as, api, data }) => {
    const title = data.uniqueName('fijar');
    const note = await createNote(data, { title, body: 'x' });
    const page = await as('member');
    await page.goto(appPath('/notes'));

    await cardOf(page, title).getByRole('button', { name: 'Fijar la nota' }).click();
    await expect(
      page.getByTestId('notes-pinned').getByTestId('note-card').filter({ hasText: title }),
    ).toBeVisible();
    await expect
      .poll(
        async () =>
          (await notesOf(await api('member'), title)).find((n) => n.id === note.id)?.pinned,
      )
      .toBe(true);
  });

  test('filtra por etiqueta desde la barra lateral', async ({ as, data }) => {
    const tag = `e2e-${Math.random().toString(36).slice(2, 7)}`;
    const tagged = data.uniqueName('con-etiqueta');
    const other = data.uniqueName('sin-etiqueta');
    await createNote(data, { title: tagged, tags: [tag] });
    await createNote(data, { title: other });
    const page = await as('member');
    await page.goto(appPath('/notes'));
    await expect(cardOf(page, other)).toBeVisible();

    await page
      .getByRole('navigation', { name: 'Filtrar notas' })
      .getByRole('button', { name: new RegExp(tag) })
      .click();
    await expect(cardOf(page, tagged)).toBeVisible();
    await expect(cardOf(page, other)).toHaveCount(0);

    await page.getByRole('button', { name: 'Todas' }).click();
    await expect(cardOf(page, other)).toBeVisible();
  });

  test('busca en las notas por texto', async ({ as, data }) => {
    const hit = data.uniqueName('busqueda');
    const miss = data.uniqueName('ajena');
    await createNote(data, { title: hit });
    await createNote(data, { title: miss });
    const page = await as('member');
    await page.goto(appPath('/notes'));
    await page.getByLabel('Buscar en mis notas').fill(hit);
    await expect(cardOf(page, hit)).toBeVisible();
    await expect(cardOf(page, miss)).toHaveCount(0);
  });

  test('elimina una nota y la recupera con Deshacer', async ({ as, api, data }) => {
    const title = data.uniqueName('borrar');
    const note = await createNote(data, { title });
    const page = await as('member');
    await page.goto(appPath('/notes'));

    await cardOf(page, title).getByRole('button', { name: 'Eliminar la nota' }).click();
    await expect(cardOf(page, title)).toHaveCount(0);
    await page.getByRole('button', { name: 'Deshacer' }).click();
    await expect(cardOf(page, title)).toBeVisible();

    const client = await api('member');
    expect((await client.get(`notes/${note.id}`)).status()).toBe(200);
  });

  test('vincula una nota a una incidencia y aparece en Mis apuntes de la incidencia', async ({
    as,
    api,
    data,
  }) => {
    const member = await api('member');
    const project = await data.createProject('member', await orgIdBySlug(member, 'acme'), 'notas');
    const summary = data.uniqueName('incidencia');
    const issue = await data.create<{ id: string; key: string }>('member', 'issues', {
      summary,
      description: 'Incidencia de prueba para apuntes.',
      project_id: project.id,
    });
    const title = data.uniqueName('vinculada');
    const note = await createNote(data, { title });

    const page = await as('member');
    await page.goto(appPath('/notes'));
    await cardOf(page, title)
      .getByRole('button', { name: `Abrir la nota ${title}` })
      .click();

    const editor = page.getByRole('dialog', { name: title });
    await expect(editor).toBeVisible();
    await editor.getByRole('combobox').fill(summary);
    await page.getByRole('option', { name: new RegExp(summary) }).click();
    await expect(editor.getByRole('link', { name: new RegExp(summary) })).toBeVisible();
    await expect(editor.getByTestId('note-save-state')).toContainText('Guardado');
    await editor.getByRole('button', { name: 'Listo' }).click();

    await expect
      .poll(async () => (await (await member.get(`notes/${note.id}`)).json()).data.issue_id)
      .toBe(issue.id);

    await page.goto(appPath(`/browse/${issue.key}`));
    const section = page.getByTestId('issue-notes');
    await expect(section.getByText('Solo tú puedes ver tus notas.')).toBeVisible();
    await expect(section.getByRole('link', { name: new RegExp(title) })).toBeVisible();
  });

  test('"Añadir a mis apuntes" crea una nota vinculada a la incidencia', async ({
    as,
    api,
    data,
  }) => {
    const member = await api('member');
    const project = await data.createProject('member', await orgIdBySlug(member, 'acme'), 'notas');
    const issue = await data.create<{ id: string; key: string }>('member', 'issues', {
      summary: data.uniqueName('incidencia'),
      description: 'Incidencia de prueba para apuntes.',
      project_id: project.id,
    });
    const marker = data.uniqueName('apunte');
    try {
      const page = await as('member');
      await page.goto(appPath(`/browse/${issue.key}`));
      await page.getByLabel('Nuevo apunte sobre esta incidencia').fill(marker);
      await page.getByRole('button', { name: 'Añadir a mis apuntes' }).click();
      await expect(page.getByTestId('issue-notes').getByText(marker)).toBeVisible();

      const [saved] = await notesOf(member, marker);
      expect(saved.issue_id).toBe(issue.id);
    } finally {
      await purge(member, marker);
    }
  });

  test('las notas son privadas: otro rol no las ve en la interfaz ni por la API', async ({
    as,
    api,
    data,
  }) => {
    const title = data.uniqueName('privada');
    const note = await createNote(data, { title, body: 'secreto' });

    const viewer = await api('viewer');
    expect((await viewer.get(`notes/${note.id}`)).status()).toBe(404);
    expect((await viewer.patch(`notes/${note.id}`, { data: { title: 'x' } })).status()).toBe(404);
    expect((await viewer.delete(`notes/${note.id}`)).status()).toBe(404);
    expect(await notesOf(viewer, title)).toEqual([]);

    const page = await as('viewer');
    await page.goto(appPath('/notes'));
    await expect(
      page.getByTestId('notes-others').or(page.getByText('Aún no tienes notas')),
    ).toBeVisible();
    await expect(page.getByText(title)).toHaveCount(0);

    // The owner still has it.
    expect((await (await api('member')).get(`notes/${note.id}`)).status()).toBe(200);
  });

  test('el borrador rápido se abre con n, guarda y se cierra con Escape', async ({
    as,
    api,
    data,
  }) => {
    const marker = data.uniqueName('rapida');
    const member = await api('member');
    try {
      const page = await as('member');
      await page.goto(appPath('/dashboard'));
      await expect(page.getByRole('button', { name: 'Mis apuntes' })).toBeVisible();

      await page.keyboard.press('n');
      const drawer = page.getByRole('dialog', { name: 'Mis apuntes' });
      await expect(drawer).toBeVisible();
      await expect(drawer.getByText('Solo tú puedes ver tus notas.')).toBeVisible();

      await drawer.getByLabel('Nota rápida').fill(marker);
      await drawer.getByRole('button', { name: 'Guardar nota' }).click();
      await expect(drawer.getByText(marker).first()).toBeVisible();

      await page.keyboard.press('Escape');
      await expect(drawer).toBeHidden();

      // The pencil button opens it too, and "Ver todas" goes to the notes page.
      await page.getByRole('button', { name: 'Mis apuntes' }).click();
      await page
        .getByRole('dialog', { name: 'Mis apuntes' })
        .getByRole('button', { name: 'Ver todas' })
        .click();
      await expect(page).toHaveURL(/\/workspace\/notes/);
      await expect(cardOf(page, marker)).toBeVisible();
    } finally {
      await purge(member, marker);
    }
  });

  test('n no abre el borrador mientras se escribe en un campo', async ({ as }) => {
    const page = await as('member');
    await page.goto(appPath('/notes'));
    await page.getByRole('button', { name: 'Escribe una nota…' }).click();
    await page.getByLabel('Título de la nota (opcional)').press('n');
    await expect(page.getByRole('dialog', { name: 'Mis apuntes' })).toHaveCount(0);
  });

  test('diseño de la página de notas y del borrador rápido', async ({ as, data, audit }) => {
    for (let index = 0; index < 4; index++) {
      await createNote(data, {
        title: data.uniqueName(`diseno${index}`),
        body:
          index % 2 === 0
            ? 'Texto corto'
            : 'Texto\n\n- uno\n- dos\n- tres\n\nMás texto de la nota.',
        color: ['yellow', 'blue', 'green', 'pink'][index],
        tags: ['e2e-diseno'],
      });
    }
    const page = await as('member');
    await page.goto(appPath('/notes'));
    await expect(page.getByTestId('note-card').first()).toBeVisible();
    await audit(page, { template: 'wide', name: 'notes-page' });

    await page.keyboard.press('n');
    await expect(page.getByRole('dialog', { name: 'Mis apuntes' })).toBeVisible();
    await audit(page, { template: 'wide', name: 'notes-scratchpad' });
  });
});
