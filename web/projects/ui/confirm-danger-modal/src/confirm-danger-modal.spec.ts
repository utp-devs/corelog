import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import { ConfirmDangerModal } from './confirm-danger-modal';
import { isConfirmationMatch } from './confirmation';

function buttonNamed(name: string): HTMLButtonElement | undefined {
  return Array.from(document.querySelectorAll('button')).find(
    (button) => button.textContent?.trim() === name,
  );
}

function fieldLabelled(pattern: RegExp): HTMLInputElement | null {
  const label = Array.from(document.querySelectorAll('label')).find((node) =>
    pattern.test(node.textContent ?? ''),
  );
  const id = label?.getAttribute('for');
  return id ? (document.getElementById(id) as HTMLInputElement) : null;
}

@Component({
  imports: [ConfirmDangerModal],
  template: `
    <app-confirm-danger-modal
      [open]="true"
      [title]="title()"
      [confirmation]="confirmation()"
      (confirmed)="confirmed = confirmed + 1"
      (cancelled)="cancelled = cancelled + 1"
    />
  `,
})
class Host {
  readonly title = signal('Eliminar proyecto');
  readonly confirmation = signal('Proyecto Alfa');
  confirmed = 0;
  cancelled = 0;
}

function create(
  confirmation = 'Proyecto Alfa',
  title = 'Eliminar proyecto',
): {
  fixture: import('@angular/core/testing').ComponentFixture<Host>;
  host: Host;
} {
  TestBed.configureTestingModule({ imports: [Host] });
  const fixture = TestBed.createComponent(Host);
  fixture.componentInstance.confirmation.set(confirmation);
  fixture.componentInstance.title.set(title);
  fixture.detectChanges();
  return { fixture, host: fixture.componentInstance };
}

describe('isConfirmationMatch', () => {
  it('requires an exact, non-empty match', () => {
    expect(isConfirmationMatch('CoreLog', 'CoreLog')).toBe(true);
    expect(isConfirmationMatch('corelog', 'CoreLog')).toBe(false);
    expect(isConfirmationMatch('CoreLog ', 'CoreLog')).toBe(false);
    expect(isConfirmationMatch('', '')).toBe(false);
  });
});

describe('ConfirmDangerModal', () => {
  afterEach(() => {
    TestBed.resetTestingModule();
  });

  it('keeps the danger button disabled until the exact name is typed', async () => {
    const { fixture } = create();
    await fixture.whenStable();
    const confirm = buttonNamed('Eliminar');
    expect(confirm?.disabled).toBe(true);

    const input = fieldLabelled(/para confirmar/);
    expect(input).not.toBeNull();
    input!.value = 'proyecto alfa';
    input!.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    expect(buttonNamed('Eliminar')?.disabled).toBe(true);

    input!.value = 'Proyecto Alfa';
    input!.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    expect(buttonNamed('Eliminar')?.disabled).toBe(false);
  });

  it('calls onConfirm only once the name matches', async () => {
    const { fixture, host } = create('Acme', 'Eliminar organización');
    await fixture.whenStable();
    buttonNamed('Eliminar')?.click();
    fixture.detectChanges();
    expect(host.confirmed).toBe(0);

    const input = fieldLabelled(/para confirmar/)!;
    input.value = 'Acme';
    input.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    buttonNamed('Eliminar')?.click();
    fixture.detectChanges();
    expect(host.confirmed).toBe(1);
  });

  it('calls onCancel from the cancel button', async () => {
    const { fixture, host } = create('X', 'Eliminar');
    await fixture.whenStable();
    buttonNamed('Cancelar')?.click();
    fixture.detectChanges();
    expect(host.cancelled).toBe(1);
  });

  it('focuses the confirmation field when it opens', async () => {
    const { fixture } = create();
    await fixture.whenStable();
    expect(document.activeElement).toBe(fieldLabelled(/para confirmar/));
  });

  it('treats Escape as cancel', async () => {
    const { fixture, host } = create();
    await fixture.whenStable();
    document
      .querySelector('[role="dialog"]')!
      .dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    expect(host.cancelled).toBe(1);
  });
});
