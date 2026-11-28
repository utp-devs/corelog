import { expect, test } from '../../fixtures/test.ts';
import { appPath } from '../../lib/paths.ts';

const OTHER_SESSION = '0d9c6e0a-2f3b-4c55-8d5e-1a2b3c4d5e6f';
const NEW_LOGIN_ID = '6b1f0c2e-1d1e-4a35-9a54-0f6f4f7a2c11';
const ISSUE_NOTICE_ID = '11111111-2222-4333-8444-555555555555';
const MESSAGE = 'Se inició sesión desde Chrome en Windows (IP 203.0.113.x) el 28/11/2026 10:00 UTC';

function notificationFrame(dto: Record<string, unknown>): string {
  return `id: ${String(dto['id'])}\nevent: notification\ndata: ${JSON.stringify(dto)}\n\n`;
}

function listBody(): unknown[] {
  return streamBody()
    .split('\n\n')
    .filter((frame) => frame.includes('event: notification'))
    .map((frame) => JSON.parse(frame.slice(frame.indexOf('data: ') + 6)) as unknown)
    .reverse();
}

/** Body the mocked stream delivers: a plain notice, the security notice and the session event. */
function streamBody(): string {
  const securityData = {
    title: 'Nuevo inicio de sesión',
    session_id: OTHER_SESSION,
    device: 'Chrome en Windows',
    actions: [
      { label: 'No fui yo', method: 'DELETE', path: `/users/me/sessions/${OTHER_SESSION}` },
      { label: 'Ver mis sesiones', link: '/profile/security' },
    ],
  };
  return [
    'retry: 3000\n\n',
    notificationFrame({
      id: ISSUE_NOTICE_ID,
      kind: 'issue.assigned',
      message: 'Te asignaron una incidencia',
      link: '/projects',
      read: false,
      created_at: '2026-11-28T10:00:00Z',
    }),
    notificationFrame({
      id: NEW_LOGIN_ID,
      kind: 'security.new_login',
      message: MESSAGE,
      link: '/profile/security',
      read: false,
      created_at: '2026-11-28T10:00:01Z',
      data: securityData,
    }),
    `event: session\ndata: ${JSON.stringify({
      type: 'new_login',
      session_id: OTHER_SESSION,
      notification_id: NEW_LOGIN_ID,
      message: MESSAGE,
      at: '2026-11-28T10:00:01Z',
      data: securityData,
    })}\n\n`,
  ].join('');
}

test.describe('avisos en vivo', () => {
  test('el contador sube al llegar un aviso y el inicio de sesión nuevo se puede rechazar', async ({
    as,
  }) => {
    const page = await as('owner');

    let unread = 0;
    let opened = 0;
    let release: () => void = () => undefined;
    const gate = new Promise<void>((resolve) => (release = resolve));
    await page.route('**/notifications/unread-count', (route) =>
      route.fulfill({ json: { data: { count: unread } } }),
    );
    // Opening the panel reads the list again: the server returns what it pushed.
    await page.route('**/notifications', (route) => route.fulfill({ json: { data: listBody() } }));
    await page.route('**/notifications/stream', async (route) => {
      opened += 1;
      if (opened > 1) {
        // The mocked stream ends after one delivery: do not let the client loop.
        await route.fulfill({ status: 403, json: { error: { code: 'x', message: 'x' } } });
        return;
      }
      await gate;
      await route.fulfill({
        status: 200,
        headers: { 'Content-Type': 'text/event-stream', 'Cache-Control': 'no-store' },
        body: streamBody(),
      });
    });
    let revoked = 0;
    await page.route(`**/users/me/sessions/${OTHER_SESSION}`, (route) => {
      revoked += 1;
      return route.fulfill({ status: 204 });
    });

    await page.goto(appPath('/projects'));
    const bell = page.getByRole('button', { name: /^Notificaciones/ });
    await expect(bell).toHaveAttribute('aria-label', 'Notificaciones');

    // The server now holds two unread notices and pushes them: the badge follows without polling.
    unread = 2;
    release();
    await expect(bell).toHaveAttribute('aria-label', 'Notificaciones, 2 sin leer');

    const alert = page.getByTestId('security-alert');
    await expect(alert).toBeVisible();
    await expect(alert).toHaveAttribute('role', 'alert');
    await expect(alert).toContainText('Nuevo inicio de sesión');
    await expect(alert).toContainText('Chrome en Windows');
    await expect(alert.getByRole('button', { name: 'No fui yo' })).toBeVisible();
    await expect(alert.getByRole('button', { name: 'Ver mis sesiones' })).toBeVisible();

    // The notification list shows the same alert with its two actions.
    await bell.click();
    const entry = page.getByTestId('notification-security');
    await expect(entry).toContainText('Chrome en Windows');
    await expect(entry.getByRole('button', { name: 'No fui yo' })).toBeVisible();
    await expect(entry.getByRole('button', { name: 'Ver mis sesiones' })).toBeVisible();
    await page.keyboard.press('Escape');

    await alert.getByRole('button', { name: 'No fui yo' }).click();
    await expect.poll(() => revoked).toBe(1);
    await expect(alert).toContainText('Cerramos la sesión desconocida');
    await alert.getByRole('button', { name: 'Cambiar contraseña' }).click();
    await expect(page).toHaveURL(/\/workspace\/profile\/security/);
    await expect(page.getByTestId('security-alert')).toHaveCount(0);
  });

  test('"Ver mis sesiones" abre la seguridad de la cuenta', async ({ as }) => {
    const page = await as('owner');
    let opened = 0;
    await page.route('**/notifications/stream', async (route) => {
      opened += 1;
      if (opened > 1) {
        await route.fulfill({ status: 403, json: { error: { code: 'x', message: 'x' } } });
        return;
      }
      await route.fulfill({
        status: 200,
        headers: { 'Content-Type': 'text/event-stream' },
        body: streamBody(),
      });
    });
    await page.goto(appPath('/projects'));
    await page
      .getByTestId('security-alert')
      .getByRole('button', { name: 'Ver mis sesiones' })
      .click();
    await expect(page).toHaveURL(/\/workspace\/profile\/security/);
  });
});
