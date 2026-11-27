import { Component, signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';

import { SidePanel } from './side-panel';

@Component({
  imports: [SidePanel],
  template: `
    <button id="opener" type="button">Abrir</button>
    <app-side-panel
      [open]="open()"
      heading="Panel de prueba"
      (closeRequested)="closeCount = closeCount + 1"
    >
      <input id="field" type="text" aria-label="Campo" />
      <button panelFooter id="save" type="button">Guardar</button>
    </app-side-panel>
  `,
})
class Host {
  readonly open = signal(true);
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

describe('SidePanel', () => {
  afterEach(() => {
    TestBed.resetTestingModule();
    document.body.style.removeProperty('overflow');
  });

  it('is a labelled modal dialog while open and renders nothing when closed', async () => {
    const { fixture, host, el } = create();
    const dialog = el.querySelector('[role="dialog"]');
    expect(dialog?.getAttribute('aria-modal')).toBe('true');
    const label = dialog?.getAttribute('aria-labelledby') ?? '';
    expect(el.querySelector(`#${label}`)?.textContent).toContain('Panel de prueba');

    host.open.set(false);
    fixture.detectChanges();
    await fixture.whenStable();
    expect(el.querySelector('[role="dialog"]')).toBeNull();
  });

  it('asks to close with Escape, the close button and the backdrop', () => {
    const { fixture, host, el } = create();
    const panel = el.querySelector<HTMLElement>('[role="dialog"]') as HTMLElement;

    expect(press(panel, 'Escape').defaultPrevented).toBe(true);
    el.querySelector<HTMLButtonElement>('.side-panel__close')?.click();
    el.querySelector('.side-panel')?.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
    fixture.detectChanges();
    expect(host.closeCount).toBe(3);
  });

  it('keeps Tab inside the panel', () => {
    const { el } = create();
    const panel = el.querySelector<HTMLElement>('[role="dialog"]') as HTMLElement;
    const save = el.querySelector<HTMLButtonElement>('#save') as HTMLButtonElement;
    const field = el.querySelector<HTMLInputElement>('#field') as HTMLInputElement;
    const close = el.querySelector<HTMLButtonElement>('.side-panel__close') as HTMLButtonElement;

    save.focus();
    expect(press(save, 'Tab').defaultPrevented).toBe(true);
    expect(document.activeElement).toBe(close);

    close.focus();
    expect(press(close, 'Tab', { shiftKey: true }).defaultPrevented).toBe(true);
    expect(document.activeElement).toBe(save);

    field.focus();
    expect(press(field, 'Tab').defaultPrevented).toBe(false);
  });

  it('locks the page scroll while open and releases it on close', async () => {
    const { fixture, host } = create();
    expect(document.body.style.overflow).toBe('hidden');
    host.open.set(false);
    fixture.detectChanges();
    await fixture.whenStable();
    expect(document.body.style.overflow).toBe('');
  });
});
