import { Component, signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { LegalStore, type ConsentRef, type LegalDocument } from '@corelog/core';

import { LegalDocumentDialog } from './legal-document-dialog';

const DOC: LegalDocument = {
  slug: 'privacy_policy',
  title: 'Política de privacidad',
  version: 3,
  summary: '',
  required: true,
  effectiveFrom: '2026-02-03T12:00:00Z',
  bodyMarkdown: '# Cuerpo\n\nTexto **importante**.',
  publishedAt: null,
};

@Component({
  imports: [LegalDocumentDialog],
  template: `<app-legal-document-dialog
    [slug]="slug()"
    [acceptable]="acceptable()"
    (closed)="closes = closes + 1"
    (accepted)="accepted = $event"
  />`,
})
class Host {
  readonly slug = signal<string | null>(null);
  readonly acceptable = signal(true);
  closes = 0;
  accepted: ConsentRef | null = null;
}

function setup(read: (slug: string) => Promise<LegalDocument>): {
  fixture: ComponentFixture<Host>;
  host: Host;
  el: HTMLElement;
} {
  TestBed.configureTestingModule({
    imports: [Host],
    providers: [provideRouter([]), { provide: LegalStore, useValue: { read } }],
  });
  const fixture = TestBed.createComponent(Host);
  fixture.detectChanges();
  return { fixture, host: fixture.componentInstance, el: fixture.nativeElement as HTMLElement };
}

async function settle(fixture: ComponentFixture<Host>): Promise<void> {
  await fixture.whenStable();
  fixture.detectChanges();
  await fixture.whenStable();
  fixture.detectChanges();
}

function button(el: HTMLElement, text: string): HTMLButtonElement | undefined {
  return Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find(
    (b) => b.textContent?.trim() === text,
  );
}

describe('LegalDocumentDialog', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('stays closed without a slug', () => {
    const { el } = setup(() => Promise.resolve(DOC));
    expect(el.querySelector('[role="dialog"]')).toBeNull();
  });

  it('shows a loading state, then the title, version, date and the sanitized body', async () => {
    let release: (doc: LegalDocument) => void = () => undefined;
    const { fixture, host, el } = setup(() => new Promise((resolve) => (release = resolve)));
    host.slug.set('privacy_policy');
    fixture.detectChanges();
    await Promise.resolve();
    fixture.detectChanges();
    expect(el.querySelector('[role="status"]')?.textContent).toContain('Cargando');

    release({ ...DOC, bodyMarkdown: 'Hola <script>alert(1)</script> **mundo**' });
    await settle(fixture);
    expect(el.querySelector('[role="dialog"]')?.getAttribute('aria-labelledby')).toBeTruthy();
    expect(el.textContent).toContain('Política de privacidad');
    expect(el.textContent).toContain('Versión 3');
    expect(el.textContent).toContain('03/02/2026');
    expect(el.querySelector('strong')?.textContent).toBe('mundo');
    expect(el.querySelector('script')).toBeNull();
  });

  it('shows an error with a retry that loads the document', async () => {
    let fail = true;
    const read = () =>
      fail
        ? Promise.reject({ status: 500, code: 'x', message: 'boom', details: [] })
        : Promise.resolve(DOC);
    const { fixture, host, el } = setup(read);
    host.slug.set('privacy_policy');
    await settle(fixture);
    expect(el.querySelector('[role="alert"]')?.textContent).toContain('No se pudo cargar');

    fail = false;
    button(el, 'Reintentar')?.click();
    await settle(fixture);
    expect(el.querySelector('[role="alert"]')).toBeNull();
    expect(el.textContent).toContain('Versión 3');
  });

  it('emits the shown version when "Aceptar y cerrar" is pressed', async () => {
    const { fixture, host, el } = setup(() => Promise.resolve(DOC));
    host.slug.set('privacy_policy');
    await settle(fixture);
    button(el, 'Aceptar y cerrar')?.click();
    expect(host.accepted).toEqual({ slug: 'privacy_policy', version: 3 });
  });

  it('offers only "Cerrar" when it is read only, and closing emits closed', async () => {
    const { fixture, host, el } = setup(() => Promise.resolve(DOC));
    host.acceptable.set(false);
    host.slug.set('privacy_policy');
    await settle(fixture);
    expect(button(el, 'Aceptar y cerrar')).toBeUndefined();
    button(el, 'Cerrar')?.click();
    expect(host.closes).toBe(1);
  });

  it('asks to close with Escape', async () => {
    const { fixture, host, el } = setup(() => Promise.resolve(DOC));
    host.slug.set('privacy_policy');
    await settle(fixture);
    el.querySelector('[role="dialog"]')?.dispatchEvent(
      new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }),
    );
    expect(host.closes).toBe(1);
  });
});
