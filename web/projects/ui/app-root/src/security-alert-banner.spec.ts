import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { NotificationStream, SecurityAlertActions, type SecurityAlert } from '@corelog/core';
import { vi } from 'vitest';

import { SecurityAlertBanner } from './security-alert-banner';

const ALERT: SecurityAlert = {
  notificationId: 'n1',
  sessionId: 's-new',
  title: 'Nuevo inicio de sesión',
  message: 'Se inició sesión desde Chrome',
  actions: [],
};

function setup(notMe = true) {
  const alert = signal<SecurityAlert | null>(ALERT);
  const stream = { securityAlert: alert, dismissAlert: vi.fn(() => alert.set(null)) };
  const actions = {
    actionsOf: () => [
      { label: 'No fui yo', method: 'DELETE' },
      { label: 'Ver mis sesiones', link: '/profile/security' },
    ],
    isNotMe: (a: { method?: string }) => a.method === 'DELETE',
    notMe: vi.fn(() => Promise.resolve(notMe)),
    open: vi.fn(),
  };
  TestBed.configureTestingModule({
    providers: [
      { provide: NotificationStream, useValue: stream },
      { provide: SecurityAlertActions, useValue: actions },
    ],
  });
  const fixture = TestBed.createComponent(SecurityAlertBanner);
  fixture.detectChanges();
  return { fixture, stream, actions, el: fixture.nativeElement as HTMLElement };
}

function button(el: HTMLElement, testId: string): HTMLButtonElement {
  return el.querySelector<HTMLButtonElement>(`[data-testid="${testId}"]`)!;
}

describe('SecurityAlertBanner', () => {
  it('is an alert with the message and both actions', () => {
    const { el } = setup();
    expect(el.querySelector('[role="alert"]')?.textContent).toContain('Chrome');
    expect(button(el, 'security-alert-not-me').textContent).toContain('No fui yo');
    expect(button(el, 'security-alert-view').textContent).toContain('Ver mis sesiones');
  });

  it('closes the new session and then offers the password change', async () => {
    const { fixture, actions, stream, el } = setup();
    button(el, 'security-alert-not-me').click();
    await fixture.whenStable();
    fixture.detectChanges();
    expect(actions.notMe).toHaveBeenCalledWith('s-new');
    expect(el.textContent).toContain('Cerramos la sesión desconocida');
    button(el, 'security-alert-change-password').click();
    expect(stream.dismissAlert).toHaveBeenCalled();
    expect(actions.open).toHaveBeenCalledWith();
  });

  it('stays as it was when the session cannot be closed', async () => {
    const { fixture, el } = setup(false);
    button(el, 'security-alert-not-me').click();
    await fixture.whenStable();
    fixture.detectChanges();
    expect(el.textContent).not.toContain('Cerramos la sesión desconocida');
    expect(button(el, 'security-alert-not-me').disabled).toBe(false);
  });

  it('opens the sessions page and dismisses the alert', () => {
    const { actions, stream, el } = setup();
    button(el, 'security-alert-view').click();
    expect(stream.dismissAlert).toHaveBeenCalled();
    expect(actions.open).toHaveBeenCalledWith('/profile/security');
  });
});
