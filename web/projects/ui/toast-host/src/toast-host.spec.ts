import { TestBed } from '@angular/core/testing';
import { NotificationService } from '@corelog/core';
import { MessageService } from 'primeng/api';

import { ToastHost } from './toast-host';

describe('ToastHost', () => {
  function setup() {
    TestBed.configureTestingModule({ providers: [MessageService] });
    const fixture = TestBed.createComponent(ToastHost);
    fixture.detectChanges();
    return { fixture, notify: TestBed.inject(NotificationService) };
  }

  afterEach(() => TestBed.resetTestingModule());

  it('shows a toast sent through the notification service with its title and message', async () => {
    const { fixture, notify } = setup();
    notify.success('Proyecto creado');
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    const text = document.body.textContent ?? '';
    expect(text).toContain('Listo');
    expect(text).toContain('Proyecto creado');
  });

  it('shows an identical message only once', async () => {
    const { fixture, notify } = setup();
    notify.error('No se pudo guardar el equipo.');
    notify.error('No se pudo guardar el equipo.');
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    const matches = Array.from(document.querySelectorAll('.p-toast-message')).filter((node) =>
      node.textContent?.includes('No se pudo guardar el equipo.'),
    );
    expect(matches).toHaveLength(1);
  });

  it('positions the outlet with the toast class', async () => {
    const { fixture } = setup();
    fixture.detectChanges();
    expect(document.querySelector('.p-toast.toast-host')).not.toBeNull();
  });
});
