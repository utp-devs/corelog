import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import { ConfirmModal } from './confirm-modal';

function buttonNamed(name: string): HTMLButtonElement | undefined {
  return Array.from(document.querySelectorAll('button')).find(
    (button) => button.textContent?.trim() === name,
  );
}

@Component({
  imports: [ConfirmModal],
  template: `
    <app-confirm-modal
      [open]="open()"
      [title]="title()"
      [description]="description()"
      [confirmText]="confirmText()"
      (confirmed)="confirmed = confirmed + 1"
      (cancelled)="cancelled = cancelled + 1"
    />
  `,
})
class Host {
  readonly open = signal(true);
  readonly title = signal('Confirmar acción');
  readonly description = signal<string | undefined>(undefined);
  readonly confirmText = signal('Confirmar');
  confirmed = 0;
  cancelled = 0;
}

function create(): { fixture: import('@angular/core/testing').ComponentFixture<Host>; host: Host } {
  TestBed.configureTestingModule({ imports: [Host] });
  const fixture = TestBed.createComponent(Host);
  fixture.detectChanges();
  return { fixture, host: fixture.componentInstance };
}

describe('ConfirmModal', () => {
  afterEach(() => {
    TestBed.resetTestingModule();
  });

  it('shows the title and the consequence description when open', async () => {
    const { fixture, host } = create();
    host.title.set('Eliminar proyecto');
    host.description.set('Se borrarán todas sus incidencias.');
    fixture.detectChanges();
    await fixture.whenStable();

    expect(document.querySelector('[role="dialog"]')?.textContent).toContain('Eliminar proyecto');
    expect(document.body.textContent).toContain('Se borrarán todas sus incidencias.');
  });

  it('calls onConfirm when the confirm button is pressed', async () => {
    const { fixture, host } = create();
    host.confirmText.set('Sí, eliminar');
    fixture.detectChanges();
    await fixture.whenStable();

    buttonNamed('Sí, eliminar')?.click();
    fixture.detectChanges();
    expect(host.confirmed).toBe(1);
  });

  it('calls onCancel when the cancel button is pressed', async () => {
    const { fixture, host } = create();
    await fixture.whenStable();

    buttonNamed('Cancelar')?.click();
    fixture.detectChanges();
    expect(host.cancelled).toBe(1);
  });

  it('treats Escape and the close button as cancel', async () => {
    const { fixture, host } = create();
    await fixture.whenStable();

    const dialog = document.querySelector<HTMLElement>('[role="dialog"]')!;
    dialog.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    expect(host.cancelled).toBe(1);

    document.querySelector<HTMLButtonElement>('button[aria-label="Cerrar"]')?.click();
    expect(host.cancelled).toBe(2);
    expect(host.confirmed).toBe(0);
  });

  it('starts focus on the cancel button so the action is never confirmed by accident', async () => {
    const { fixture } = create();
    await fixture.whenStable();
    expect(document.activeElement).toBe(buttonNamed('Cancelar'));
  });

  it('does not use the PrimeNG dialog', async () => {
    const { fixture } = create();
    await fixture.whenStable();
    expect(document.querySelector('.p-dialog')).toBeNull();
  });
});
