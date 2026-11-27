import { Component, signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';

import { Dialog, type DialogSize } from './dialog';

@Component({
  imports: [Dialog],
  template: `
    <button id="opener" type="button">Abrir</button>
    <app-dialog
      [open]="open()"
      heading="Título de prueba"
      [subtitle]="subtitle()"
      [size]="size()"
      [dismissable]="dismissable()"
      (closeRequested)="closeCount = closeCount + 1"
    >
      <input id="field" type="text" aria-label="Campo" />
      <button dialogFooter id="cancel" type="button">Cancelar</button>
      <button dialogFooter id="save" type="button">Guardar</button>
    </app-dialog>
  `,
})
class Host {
  readonly open = signal(true);
  readonly subtitle = signal<string | undefined>(undefined);
  readonly size = signal<DialogSize>('md');
  readonly dismissable = signal(true);
  closeCount = 0;
}

function create(): { fixture: ComponentFixture<Host>; host: Host; el: HTMLElement } {
  TestBed.configureTestingModule({ imports: [Host] });
  const fixture = TestBed.createComponent(Host);
  fixture.detectChanges();
  return { fixture, host: fixture.componentInstance, el: fixture.nativeElement as HTMLElement };
}

function press(target: Element, key: string, init: KeyboardEventInit = {}): KeyboardEvent {
  const event = new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true, ...init });
  target.dispatchEvent(event);
  return event;
}

describe('Dialog', () => {
  afterEach(() => {
    TestBed.resetTestingModule();
    document.body.style.removeProperty('overflow');
  });

  it('renders an accessible modal with title, subtitle and footer buttons', () => {
    const { fixture, host, el } = create();
    host.subtitle.set('Detalle del diálogo');
    fixture.detectChanges();

    const dialog = el.querySelector<HTMLElement>('[role="dialog"]');
    expect(dialog?.getAttribute('aria-modal')).toBe('true');
    const labelledBy = dialog?.getAttribute('aria-labelledby') ?? '';
    expect(el.querySelector(`#${labelledBy}`)?.textContent).toContain('Título de prueba');
    expect(el.textContent).toContain('Detalle del diálogo');
    expect(el.querySelector('.dialog__footer #save')).not.toBeNull();
    expect(el.querySelector('button[aria-label="Cerrar"]')).not.toBeNull();
  });

  it('renders nothing while closed', () => {
    const { fixture, host, el } = create();
    host.open.set(false);
    fixture.detectChanges();
    expect(el.querySelector('[role="dialog"]')).toBeNull();
  });

  it('applies the size modifier', () => {
    const { fixture, host, el } = create();
    host.size.set('lg');
    fixture.detectChanges();
    expect(el.querySelector('.dialog__panel--lg')).not.toBeNull();
  });

  it('asks to close from the close button, Escape and the backdrop', () => {
    const { fixture, host, el } = create();
    el.querySelector<HTMLButtonElement>('button[aria-label="Cerrar"]')?.click();
    expect(host.closeCount).toBe(1);

    press(el.querySelector('#field')!, 'Escape');
    expect(host.closeCount).toBe(2);

    const backdrop = el.querySelector<HTMLElement>('.dialog')!;
    backdrop.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
    expect(host.closeCount).toBe(3);

    // A press inside the panel is not a backdrop click.
    el.querySelector('.dialog__panel')!.dispatchEvent(
      new MouseEvent('mousedown', { bubbles: true }),
    );
    expect(host.closeCount).toBe(3);
    fixture.detectChanges();
  });

  it('ignores Escape and the backdrop when it is not dismissable, but keeps the close button', () => {
    const { fixture, host, el } = create();
    host.dismissable.set(false);
    fixture.detectChanges();

    press(el.querySelector('#field')!, 'Escape');
    el.querySelector<HTMLElement>('.dialog')!.dispatchEvent(
      new MouseEvent('mousedown', { bubbles: true }),
    );
    expect(host.closeCount).toBe(0);

    el.querySelector<HTMLButtonElement>('button[aria-label="Cerrar"]')?.click();
    expect(host.closeCount).toBe(1);
  });

  it('leaves Escape alone when an inner control already handled it', () => {
    const { host, el } = create();
    const field = el.querySelector('#field')!;
    field.addEventListener('keydown', (event) => event.preventDefault());
    press(field, 'Escape');
    expect(host.closeCount).toBe(0);
  });

  it('moves focus to the first field on open and keeps Tab inside the dialog', async () => {
    const { fixture, el } = create();
    await fixture.whenStable();
    expect(document.activeElement).toBe(el.querySelector('#field'));

    const save = el.querySelector<HTMLElement>('#save')!;
    save.focus();
    const forward = press(save, 'Tab');
    expect(forward.defaultPrevented).toBe(true);
    expect(document.activeElement).toBe(el.querySelector('button[aria-label="Cerrar"]'));

    const close = el.querySelector<HTMLElement>('button[aria-label="Cerrar"]')!;
    const backward = press(close, 'Tab', { shiftKey: true });
    expect(backward.defaultPrevented).toBe(true);
    expect(document.activeElement).toBe(save);
  });

  it('returns focus to the opener and releases the page scroll when it closes', async () => {
    const { fixture, host, el } = create();
    host.open.set(false);
    fixture.detectChanges();
    const opener = el.querySelector<HTMLElement>('#opener')!;
    opener.focus();

    host.open.set(true);
    fixture.detectChanges();
    await fixture.whenStable();
    expect(document.body.style.overflow).toBe('hidden');
    expect(document.activeElement).toBe(el.querySelector('#field'));

    host.open.set(false);
    fixture.detectChanges();
    expect(document.activeElement).toBe(opener);
    expect(document.body.style.overflow).toBe('');
  });
});
