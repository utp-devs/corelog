import { Component, signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { FormControl, ReactiveFormsModule } from '@angular/forms';
import { provideRouter } from '@angular/router';
import { LegalStore, type ConsentRef, type LegalDocumentSummary } from '@corelog/core';

import { LegalConsentGroup } from './legal-consent-group';

const DOCS: LegalDocumentSummary[] = [
  ['privacy_policy', 'Privacidad', true, 2],
  ['data_processing', 'Datos', true, 1],
  ['terms_of_use', 'Términos', true, 5],
  ['communications', 'Comunicaciones', false, 1],
].map(([slug, title, required, version]) => ({
  slug: slug as string,
  title: title as string,
  version: version as number,
  summary: '',
  required: required as boolean,
  effectiveFrom: '',
}));

@Component({
  imports: [LegalConsentGroup, ReactiveFormsModule],
  template: `<app-legal-consent-group
    [formControl]="control"
    [showErrors]="showErrors()"
    [serverMissing]="missing()"
  />`,
})
class Host {
  readonly control = new FormControl<ConsentRef[]>([], { nonNullable: true });
  readonly showErrors = signal(false);
  readonly missing = signal<string[]>([]);
}

function setup(status = 'ready'): { fixture: ComponentFixture<Host>; host: Host; el: HTMLElement } {
  TestBed.configureTestingModule({
    imports: [Host],
    providers: [
      provideRouter([]),
      {
        provide: LegalStore,
        useValue: {
          documents: signal(DOCS),
          status: signal(status),
          find: (slug: string) => DOCS.find((d) => d.slug === slug),
          load: () => Promise.resolve(),
          read: (slug: string) =>
            Promise.resolve({
              ...DOCS.find((d) => d.slug === slug),
              bodyMarkdown: 'x',
              publishedAt: null,
            }),
        },
      },
    ],
  });
  const fixture = TestBed.createComponent(Host);
  fixture.detectChanges();
  return { fixture, host: fixture.componentInstance, el: fixture.nativeElement as HTMLElement };
}

function boxes(el: HTMLElement): HTMLInputElement[] {
  return Array.from(el.querySelectorAll<HTMLInputElement>('input[type="checkbox"]'));
}

function tick(box: HTMLInputElement, value = true): void {
  box.checked = value;
  box.dispatchEvent(new Event('change'));
}

describe('LegalConsentGroup', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('draws the three required boxes and the optional one, unticked', () => {
    const { el } = setup();
    expect(boxes(el)).toHaveLength(4);
    expect(boxes(el).some((b) => b.checked)).toBe(false);
    expect(el.textContent).toContain('He leído y acepto la');
    expect(el.textContent).toContain('Política de privacidad');
    expect(el.textContent).toContain('tratamiento de mis datos personales');
    expect(el.textContent).toContain('Términos de uso');
    expect(el.textContent).toContain('(opcional)');
  });

  it('is invalid until the three required documents are ticked, whatever the optional one', () => {
    const { fixture, host, el } = setup();
    expect(host.control.errors?.['consentsMissing']).toHaveLength(3);
    tick(boxes(el)[3]);
    fixture.detectChanges();
    expect(host.control.invalid).toBe(true);
    tick(boxes(el)[0]);
    tick(boxes(el)[1]);
    expect(host.control.invalid).toBe(true);
    tick(boxes(el)[2]);
    expect(host.control.valid).toBe(true);
  });

  it('emits the slug and the version that is shown for each ticked box', () => {
    const { host, el } = setup();
    tick(boxes(el)[0]);
    tick(boxes(el)[2]);
    tick(boxes(el)[3]);
    expect(host.control.value).toEqual([
      { slug: 'privacy_policy', version: 2 },
      { slug: 'terms_of_use', version: 5 },
      { slug: 'communications', version: 1 },
    ]);
    tick(boxes(el)[0], false);
    expect(host.control.value.map((c) => c.slug)).not.toContain('privacy_policy');
  });

  it('shows the message under an unticked required box after a submit attempt', () => {
    const { fixture, host, el } = setup();
    host.showErrors.set(true);
    fixture.detectChanges();
    expect(el.querySelectorAll('[role="alert"]')).toHaveLength(3);
    tick(boxes(el)[0]);
    fixture.detectChanges();
    expect(el.querySelectorAll('[role="alert"]')).toHaveLength(2);
    expect(boxes(el)[1].getAttribute('aria-invalid')).toBe('true');
  });

  it('flags the boxes the server reported as missing', () => {
    const { fixture, host, el } = setup();
    host.missing.set(['terms_of_use']);
    fixture.detectChanges();
    const alerts = el.querySelectorAll('[role="alert"]');
    expect(alerts).toHaveLength(1);
    expect(alerts[0].textContent).toContain('términos de uso');
  });

  it('opens the document from the link and ticks the box when accepted in the modal', async () => {
    const { fixture, host, el } = setup();
    Array.from(el.querySelectorAll<HTMLButtonElement>('button'))
      .find((b) => b.textContent?.trim() === 'Términos de uso')
      ?.click();
    await fixture.whenStable();
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    expect(el.querySelector('[role="dialog"]')).not.toBeNull();
    // The link does not toggle the box by itself.
    expect(boxes(el)[2].checked).toBe(false);

    Array.from(el.querySelectorAll<HTMLButtonElement>('[role="dialog"] button'))
      .find((b) => b.textContent?.includes('Aceptar y cerrar'))
      ?.click();
    await fixture.whenStable();
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    expect(boxes(el)[2].checked).toBe(true);
    expect(host.control.value).toEqual([{ slug: 'terms_of_use', version: 5 }]);
    expect(el.querySelector('[role="dialog"]')).toBeNull();
  });

  it('offers a retry when the documents cannot be loaded', () => {
    const { el } = setup('error');
    expect(el.querySelector('[role="alert"]')?.textContent).toContain('No se pudieron cargar');
    expect(boxes(el)).toHaveLength(0);
  });
});
