import { TestBed } from '@angular/core/testing';
import { ApiTokenApiService, NotificationService, type ApiToken } from '@corelog/core';
import { of, throwError } from 'rxjs';
import { vi } from 'vitest';

import { ApiTokensSection } from './api-tokens-section';

const TOKEN: ApiToken = {
  id: 't1',
  name: 'CI',
  prefix: 'cl_pat_ab12',
  scopes: ['read'],
  expiresAt: null,
  lastUsedAt: null,
  revokedAt: null,
  createdAt: '2026-01-01T00:00:00Z',
};

function setup(tokens: readonly ApiToken[] = [TOKEN], api: Partial<ApiTokenApiService> = {}) {
  const service = {
    list: vi.fn(() => of(tokens)),
    create: vi.fn(() =>
      of({ token: { ...TOKEN, id: 't2', name: 'Nuevo' }, secret: 'cl_pat_secretvalue' }),
    ),
    revoke: vi.fn(() => of(undefined)),
    ...api,
  };
  TestBed.configureTestingModule({
    providers: [
      { provide: ApiTokenApiService, useValue: service },
      { provide: NotificationService, useValue: { success: vi.fn(), error: vi.fn() } },
    ],
  });
  const fixture = TestBed.createComponent(ApiTokensSection);
  fixture.detectChanges();
  return { fixture, service, el: fixture.nativeElement as HTMLElement };
}

async function settle(fixture: { detectChanges(): void; whenStable(): Promise<unknown> }) {
  fixture.detectChanges();
  await fixture.whenStable();
  fixture.detectChanges();
}

describe('ApiTokensSection', () => {
  it('lists the tokens with prefix and state', () => {
    const { el } = setup();
    const item = el.querySelector('[data-testid="token-item"]');
    expect(item?.textContent).toContain('CI');
    expect(item?.textContent).toContain('cl_pat_ab12');
    expect(item?.textContent).toContain('Activo');
    expect(item?.textContent).toContain('Nunca');
  });

  it('shows the empty state without tokens', () => {
    const { el } = setup([]);
    expect(el.querySelector('[data-testid="token-empty"]')).not.toBeNull();
  });

  it('shows an error with retry when the list fails', async () => {
    const { fixture, el } = setup([], { list: vi.fn(() => throwError(() => new Error('x'))) });
    await settle(fixture);
    expect(el.textContent).toContain('No se pudieron cargar tus tokens');
  });

  it('reveals the new token once and forgets it when dismissed', async () => {
    const { fixture, service, el } = setup();
    el.querySelector<HTMLButtonElement>('[data-testid="token-new"] button')?.click();
    fixture.detectChanges();
    const name = document.querySelector<HTMLInputElement>('[data-testid="token-name"]');
    name!.value = 'Nuevo';
    name!.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    document.querySelector<HTMLButtonElement>('[data-testid="token-create"] button')?.click();
    await settle(fixture);

    expect(service.create).toHaveBeenCalledWith(
      expect.objectContaining({ name: 'Nuevo', scopes: ['read'] }),
    );
    const field = document.querySelector<HTMLInputElement>('[data-testid="secret-reveal-value"]');
    expect(field?.value).toBe('cl_pat_secretvalue');

    document.querySelector<HTMLInputElement>('[data-testid="secret-reveal-ack"]')?.click();
    fixture.detectChanges();
    document
      .querySelector<HTMLButtonElement>('[data-testid="secret-reveal-close"] button')
      ?.click();
    await settle(fixture);

    expect(document.body.textContent).not.toContain('cl_pat_secretvalue');
    expect(el.querySelectorAll('[data-testid="token-item"]')).toHaveLength(2);
  });

  it('revokes after confirming', async () => {
    const { fixture, service, el } = setup();
    el.querySelector<HTMLButtonElement>('[data-testid="token-revoke"] button')?.click();
    fixture.detectChanges();
    const confirm = Array.from(document.querySelectorAll<HTMLButtonElement>('button')).find((b) =>
      b.textContent?.includes('Revocar token'),
    );
    confirm?.click();
    await settle(fixture);
    expect(service.revoke).toHaveBeenCalledWith('t1');
    expect(el.querySelectorAll('[data-testid="token-item"]')).toHaveLength(0);
    expect(el.querySelector('[data-testid="token-empty"]')).not.toBeNull();
  });

  it('requests the revoked tokens only when the toggle is on and shows them dimmed', async () => {
    const revoked: ApiToken = {
      ...TOKEN,
      id: 't9',
      name: 'Viejo',
      revokedAt: '2026-02-03T10:00:00Z',
    };
    const list = vi.fn((include?: boolean) => of(include ? [TOKEN, revoked] : [TOKEN]));
    const { fixture, el } = setup([TOKEN], { list });
    expect(list).toHaveBeenLastCalledWith(false);
    expect(el.querySelectorAll('[data-testid="token-item"]')).toHaveLength(1);

    const toggle = el.querySelector<HTMLInputElement>('[data-testid="token-show-revoked"]')!;
    expect(toggle.checked).toBe(false);
    toggle.click();
    await settle(fixture);

    expect(list).toHaveBeenLastCalledWith(true);
    const items = el.querySelectorAll('[data-testid="token-item"]');
    expect(items).toHaveLength(2);
    expect(items[1].classList).toContain('api-tokens__item--revoked');
    expect(items[1].textContent).toContain('Revocado');
    expect(items[1].querySelector('[data-testid="token-revoked-at"]')?.textContent).toContain(
      '03/02/2026',
    );
  });
});
