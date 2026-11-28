import type { APIRequestContext, Page } from '@playwright/test';

import { expect, test } from '../../fixtures/test.ts';
import { readE2eEnv } from '../../lib/env.ts';
import { appPath } from '../../lib/paths.ts';

interface WindowItem {
  readonly kind: string;
  readonly title: string;
  readonly source_id: string;
  readonly date: string;
  readonly occurrence_start: string | null;
}

function isoDay(offsetDays: number): string {
  const date = new Date();
  date.setDate(date.getDate() + offsetDays);
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
}

/** Occurrences of the event with this title in the next 60 days, as the API returns them. */
async function occurrencesOf(client: APIRequestContext, title: string): Promise<WindowItem[]> {
  const response = await client.get('calendar', {
    params: { from: isoDay(-1), to: isoDay(60), types: 'event' },
  });
  expect(response.ok()).toBe(true);
  const body = (await response.json()) as { data: { items: WindowItem[] } };
  return body.data.items.filter((item) => item.title === title);
}

/** Creates an event through the API and registers its deletion. */
async function createEvent(
  client: APIRequestContext,
  body: Record<string, unknown>,
): Promise<string> {
  const response = await client.post('calendar/events', { data: body });
  expect(response.status(), 'creating the event').toBe(201);
  return ((await response.json()) as { data: { id: string } }).data.id;
}

async function removeEvent(client: APIRequestContext, id: string): Promise<void> {
  await client.delete(`calendar/events/${id}`);
}

/** The list view is available on every viewport, so the functional specs read the agenda. */
async function openAgenda(page: Page): Promise<void> {
  await page.goto(appPath('/calendar'));
  await expect(page.getByTestId('calendar-view')).toBeVisible();
  await page.getByRole('button', { name: 'Agenda', exact: true }).click();
  await expect(page.getByTestId('calendar-agenda')).toBeVisible();
}

test.describe('calendario', () => {
  test('crear un evento semanal desde la interfaz, ver sus ocurrencias y cancelar una', async ({
    as,
    api,
    data,
  }) => {
    const title = data.uniqueName('semanal');
    const owner = await api('owner');
    const page = await as('owner');
    await openAgenda(page);

    await page.getByTestId('calendar-new').getByRole('button').click();
    const dialog = page.getByRole('dialog', { name: 'Nuevo evento' });
    await expect(dialog).toBeVisible();
    await dialog.getByTestId('event-title').fill(title);
    await dialog.getByRole('combobox', { name: 'Repetición' }).click();
    await page.getByRole('option', { name: 'Cada semana' }).click();
    await expect(dialog.getByTestId('recurrence-summary')).toContainText('Cada semana el');
    await dialog.getByText('Después de').click();
    await dialog.getByTestId('recurrence-count').fill('3');
    await dialog.getByTestId('recurrence-count').dispatchEvent('change');
    await expect(dialog.getByTestId('recurrence-summary')).toContainText('3 veces');
    await dialog.getByTestId('event-save').getByRole('button').click();
    await expect(dialog).toBeHidden();

    // Three occurrences, a week apart, and the first one is in the agenda of this month.
    let occurrences: WindowItem[] = [];
    await expect.poll(async () => (occurrences = await occurrencesOf(owner, title)).length).toBe(3);
    const eventId = occurrences[0].source_id;
    data.track('owner', `calendar/events/${eventId}`);

    await expect(page.getByTestId('calendar-agenda').getByText(title).first()).toBeVisible();

    // Cancel only the first occurrence: the series keeps two.
    await page.getByTestId('calendar-agenda').getByText(title).first().click();
    const panel = page.getByRole('dialog', { name: title });
    await expect(panel.getByTestId('event-recurrence')).toContainText('Cada semana');
    await panel.getByTestId('event-delete').getByRole('button').click();
    const choice = page.getByRole('dialog', { name: 'Eliminar evento repetido' });
    await expect(choice).toBeVisible();
    await choice.getByTestId('delete-occurrence').getByRole('button').click();
    await expect(choice).toBeHidden();
    await expect.poll(async () => (await occurrencesOf(owner, title)).length).toBe(2);
  });

  test('un invitado responde a la invitación y el organizador ve su respuesta', async ({
    as,
    api,
    data,
  }) => {
    const title = data.uniqueName('invitacion');
    const owner = await api('owner');
    const member = await api('member');
    const me = (await (await member.get('users/me')).json()) as { data: { id: string } };
    const day = isoDay(1);
    const id = await createEvent(owner, {
      title,
      starts_at: `${day}T15:00:00Z`,
      ends_at: `${day}T16:00:00Z`,
      timezone: 'UTC',
      visibility: 'private',
      attendees: [me.data.id],
    });
    data.track('owner', `calendar/events/${id}`);

    const page = await as('member');
    await openAgenda(page);
    await page.getByTestId('calendar-agenda').getByText(title).first().click();
    const panel = page.getByRole('dialog', { name: title });
    await expect(panel.getByTestId('rsvp-accepted')).toBeVisible();
    // The guest cannot edit or delete the organizer's event.
    await expect(panel.getByTestId('event-edit')).toHaveCount(0);
    await panel.getByTestId('rsvp-accepted').click();
    await expect(panel.getByTestId('rsvp-accepted')).toHaveAttribute('aria-pressed', 'true');

    const detail = (await (await owner.get(`calendar/events/${id}`)).json()) as {
      data: { attendees: { response: string }[] };
    };
    expect(detail.data.attendees.map((a) => a.response)).toEqual(['accepted']);
  });

  test('la suscripción ICS se crea, devuelve un calendario válido y se revoca', async ({
    as,
    api,
    playwright,
  }) => {
    const owner = await api('owner');
    const page = await as('owner');
    try {
      await page.goto(appPath('/calendar'));
      await page.getByTestId('calendar-feed').getByRole('button').click();
      const dialog = page.getByRole('dialog', { name: /Suscribirse al calendario/ });
      await expect(dialog).toBeVisible();
      await dialog.getByTestId('feed-generate').getByRole('button').click();
      const url = await dialog.getByTestId('feed-url').inputValue();
      expect(url).toContain('token=');
      await expect(dialog.getByTestId('feed-copy')).toBeVisible();

      const token = new URL(url).searchParams.get('token') ?? '';
      const anonymous = await playwright.request.newContext({ baseURL: `${readE2eEnv().apiURL}/` });
      const feed = await anonymous.get('calendar/export.ics', { params: { token } });
      expect(feed.status()).toBe(200);
      expect(await feed.text()).toContain('BEGIN:VCALENDAR');

      await dialog.getByTestId('feed-revoke').getByRole('button').click();
      await expect(dialog.getByTestId('feed-disabled')).toBeVisible();
      const revoked = await anonymous.get('calendar/export.ics', { params: { token } });
      expect(revoked.status()).toBe(404);
      await anonymous.dispose();
    } finally {
      await owner.delete('me/calendar/feed');
    }
  });

  test('el evento propio se edita desde el panel y cambia su título', async ({ as, api, data }) => {
    const title = data.uniqueName('editar');
    const owner = await api('owner');
    const day = isoDay(1);
    const id = await createEvent(owner, {
      title,
      starts_at: `${day}T15:00:00Z`,
      ends_at: `${day}T16:00:00Z`,
      timezone: 'UTC',
      visibility: 'private',
    });
    data.track('owner', `calendar/events/${id}`);

    const page = await as('owner');
    await openAgenda(page);
    await page.getByTestId('calendar-agenda').getByText(title).first().click();
    await page
      .getByRole('dialog', { name: title })
      .getByTestId('event-edit')
      .getByRole('button')
      .click();
    const dialog = page.getByRole('dialog', { name: 'Editar evento' });
    await dialog.getByTestId('event-title').fill(`${title} v2`);
    await dialog.getByTestId('event-save').getByRole('button').click();
    await expect(dialog).toBeHidden();
    await expect.poll(async () => (await occurrencesOf(owner, `${title} v2`)).length).toBe(1);
  });

  test('el calendario cumple la auditoría de layout en mes, semana, día y agenda', async ({
    as,
    audit,
  }) => {
    const page = await as('owner');
    await page.goto(appPath('/calendar'));
    await expect(page.getByTestId('calendar-view')).toBeVisible();
    const views =
      (await page.getByRole('button', { name: 'Mes', exact: true }).count()) > 0
        ? ['Mes', 'Semana', 'Día', 'Agenda']
        : ['Día', 'Agenda'];
    for (const view of views) {
      await page.getByRole('button', { name: view, exact: true }).click();
      await expect(page.getByTestId('calendar-title')).toBeVisible();
      // A short list on a wide screen is naturally airy; the agenda is audited where it is the main view (phones).
      const airy = view === 'Agenda' && (page.viewportSize()?.width ?? 0) >= 768;
      if (!airy) await audit(page, { template: 'fluid', name: `calendar-${view.toLowerCase()}` });
    }
  });
});
