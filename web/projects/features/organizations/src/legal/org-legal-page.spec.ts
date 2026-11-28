import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import {
  LegalApiService,
  LegalStore,
  NotificationService,
  type ConsentRecord,
  type LegalDocument,
} from '@corelog/core';
import { of } from 'rxjs';

import { OrgLegalPage } from './org-legal-page';

function doc(slug: string, title: string, version: number, required: boolean): LegalDocument {
  return {
    slug,
    title,
    version,
    summary: 'Resumen',
    required,
    effectiveFrom: '2026-03-04T12:00:00Z',
    bodyMarkdown: '',
    publishedAt: null,
  };
}

const VERSIONS: LegalDocument[] = [
  doc('privacy_policy', 'Política de privacidad', 1, true),
  doc('privacy_policy', 'Política de privacidad', 2, true),
  doc('communications', 'Comunicaciones', 1, false),
];

const RECORDS: ConsentRecord[] = [
  {
    id: 'c1',
    userId: 'user-1',
    slug: 'privacy_policy',
    version: 2,
    acceptedAt: '2026-03-04T12:00:00Z',
    source: 'login_reprompt',
    revokedAt: null,
  },
];

function setup() {
  const adminPublish = vi.fn(() => of(doc('privacy_policy', 'Nueva', 3, true)));
  const adminConsents = vi.fn(() => of(RECORDS));
  const success = vi.fn();
  const load = vi.fn().mockResolvedValue(undefined);
  TestBed.configureTestingModule({
    imports: [OrgLegalPage],
    providers: [
      provideRouter([]),
      {
        provide: LegalApiService,
        useValue: { adminListDocuments: () => of(VERSIONS), adminConsents, adminPublish },
      },
      {
        provide: LegalStore,
        useValue: {
          load,
          read: () =>
            Promise.resolve({
              ...VERSIONS[1],
              summary: 'Resumen vigente',
              bodyMarkdown: '# Texto vigente',
            }),
        },
      },
      { provide: NotificationService, useValue: { success, error: vi.fn() } },
    ],
  });
  const fixture = TestBed.createComponent(OrgLegalPage);
  fixture.detectChanges();
  return {
    fixture,
    el: fixture.nativeElement as HTMLElement,
    adminPublish,
    adminConsents,
    success,
    load,
  };
}

async function settle(fixture: {
  whenStable(): Promise<unknown>;
  detectChanges(): void;
}): Promise<void> {
  await fixture.whenStable();
  fixture.detectChanges();
  await fixture.whenStable();
  fixture.detectChanges();
}

function button(el: HTMLElement, text: string): HTMLButtonElement | undefined {
  return Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find((b) =>
    b.textContent?.trim().startsWith(text),
  );
}

describe('OrgLegalPage', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('lists the current version of each document with its history count', async () => {
    const { fixture, el } = setup();
    await settle(fixture);
    const text = el.textContent ?? '';
    expect(text).toContain('Política de privacidad');
    expect(text).toContain('Comunicaciones');
    const rows = Array.from(el.querySelectorAll('tbody tr')).map((r) => r.textContent ?? '');
    expect(rows[0]).toMatch(/Política de privacidad\s*2\s/);
    expect(rows[0]).toContain('2');
  });

  it('shows the consent audit and filters it by user', async () => {
    const { fixture, el, adminConsents } = setup();
    await settle(fixture);
    expect(el.textContent).toContain('user-1');
    expect(el.textContent).toContain('Nueva versión');

    const input = el.querySelector<HTMLInputElement>(
      'input[placeholder^="0000"]',
    ) as HTMLInputElement;
    input.value = ' user-1 ';
    input.dispatchEvent(new Event('input'));
    button(el, 'Filtrar')?.click();
    await settle(fixture);
    expect(adminConsents).toHaveBeenLastCalledWith('user-1');
  });

  it('opens the publisher with the current text and asks for confirmation before publishing', async () => {
    const { fixture, el, adminPublish, success, load } = setup();
    await settle(fixture);
    button(el, 'Publicar nueva versión')?.click();
    await settle(fixture);
    const dialog = el.querySelector('[role="dialog"]');
    expect(dialog?.textContent).toContain('Será la versión 3');
    const textarea = el.querySelector<HTMLTextAreaElement>('textarea') as HTMLTextAreaElement;
    expect(textarea.value).toBe('# Texto vigente');

    // Edit title and body, then publish.
    const title = Array.from(
      el.querySelectorAll<HTMLInputElement>('[role="dialog"] input[type="text"]'),
    )[0];
    title.value = 'Nuevo título';
    title.dispatchEvent(new Event('input'));
    textarea.value = '# Texto nuevo';
    textarea.dispatchEvent(new Event('input'));
    await settle(fixture);
    Array.from(el.querySelectorAll<HTMLButtonElement>('button'))
      .find((b) => b.textContent?.trim() === 'Publicar')
      ?.click();
    await settle(fixture);
    expect(adminPublish).not.toHaveBeenCalled();
    expect(el.textContent).toContain('Todas las personas tendrán que aceptarla de nuevo');

    const confirm = Array.from(el.querySelectorAll<HTMLButtonElement>('button')).filter(
      (b) => b.textContent?.trim() === 'Publicar',
    );
    confirm[confirm.length - 1].click();
    await settle(fixture);
    expect(adminPublish).toHaveBeenCalledWith({
      slug: 'privacy_policy',
      title: 'Nuevo título',
      summary: 'Resumen vigente',
      bodyMarkdown: '# Texto nuevo',
      required: true,
    });
    expect(success).toHaveBeenCalledWith('Versión 3 publicada');
    expect(load).toHaveBeenCalledWith(true);
  });

  it('renders the Markdown in the preview tab', async () => {
    const { fixture, el } = setup();
    await settle(fixture);
    button(el, 'Publicar nueva versión')?.click();
    await settle(fixture);
    const textarea = el.querySelector<HTMLTextAreaElement>('textarea') as HTMLTextAreaElement;
    textarea.value = '## Subtítulo\n\nTexto **fuerte**';
    textarea.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    Array.from(el.querySelectorAll<HTMLElement>('[role="radio"], .p-togglebutton, button'))
      .find((b) => b.textContent?.trim() === 'Vista previa')
      ?.click();
    await settle(fixture);
    const preview = el.querySelector('[data-testid="legal-preview"]');
    expect(preview?.querySelector('h2')?.textContent).toBe('Subtítulo');
    expect(preview?.querySelector('strong')?.textContent).toBe('fuerte');
  });
});
