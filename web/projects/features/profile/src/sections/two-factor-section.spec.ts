import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { NotificationService, TwoFactorStore, type TwoFactorStatus } from '@corelog/core';
import { vi } from 'vitest';

import { ProfileTwoFactorSection } from './two-factor-section';

function setup(status: TwoFactorStatus) {
  const recoveryCodes = signal<readonly string[] | null>(null);
  const enrollment = signal<{ secret: string; otpauthUri: string } | null>(null);
  const store = {
    status: {
      value: signal<TwoFactorStatus | undefined>(status),
      loading: signal(false),
      error: signal(null),
    },
    recoveryCodes,
    enrollment,
    busy: signal(false),
    load: vi.fn(),
    beginEnrollment: vi.fn(async () => {
      const next = {
        secret: 'JBSWY3DPEHPK3PXP',
        otpauthUri: 'otpauth://totp/CoreLog:a?secret=JBSWY3DPEHPK3PXP',
      };
      enrollment.set(next);
      return next;
    }),
    confirmEnrollment: vi.fn(async () => {
      recoveryCodes.set(Array.from({ length: 10 }, (_, i) => `COD-${i}`));
    }),
    clearRecoveryCodes: vi.fn(() => recoveryCodes.set(null)),
    cancelEnrollment: vi.fn(),
    disable: vi.fn(async () => undefined),
    regenerateRecoveryCodes: vi.fn(),
  };
  TestBed.configureTestingModule({
    providers: [
      { provide: TwoFactorStore, useValue: store },
      { provide: NotificationService, useValue: { success: vi.fn(), error: vi.fn() } },
    ],
  });
  const fixture = TestBed.createComponent(ProfileTwoFactorSection);
  fixture.detectChanges();
  return { fixture, store, root: fixture.nativeElement as HTMLElement };
}

const OFF: TwoFactorStatus = { enabled: false, enabledAt: null, recoveryCodesRemaining: 0 };
const ON: TwoFactorStatus = {
  enabled: true,
  enabledAt: '2026-01-01T00:00:00Z',
  recoveryCodesRemaining: 8,
};

async function settle(fixture: {
  detectChanges(): void;
  whenStable(): Promise<unknown>;
}): Promise<void> {
  fixture.detectChanges();
  await fixture.whenStable();
  fixture.detectChanges();
}

describe('ProfileTwoFactorSection', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('offers to turn it on and shows an accessible QR with the secret as text', async () => {
    const { fixture, root } = setup(OFF);
    root.querySelector<HTMLButtonElement>('[data-testid="two-factor-enable"] button')?.click();
    await settle(fixture);
    const qr = document.querySelector('[data-testid="two-factor-qr"]');
    expect(qr?.getAttribute('role')).toBe('img');
    expect(qr?.getAttribute('aria-label')).toContain('Código QR');
    expect(document.querySelector('[data-testid="two-factor-secret"]')?.textContent).toContain(
      'JBSW Y3DP EHPK 3PXP',
    );
    expect(qr?.querySelector('path')?.getAttribute('d')?.length).toBeGreaterThan(100);
  });

  it('shows the recovery codes once and forgets them when the dialog is closed', async () => {
    const { fixture, store } = setup(OFF);
    await store.confirmEnrollment();
    await settle(fixture);
    expect(document.querySelectorAll('[data-testid="recovery-codes"] li')).toHaveLength(10);
    expect(
      document.querySelector<HTMLButtonElement>('[data-testid="recovery-done"] button')?.disabled,
    ).toBe(true);
    document.querySelector<HTMLInputElement>('.two-factor__ack input')?.click();
    await settle(fixture);
    document.querySelector<HTMLButtonElement>('[data-testid="recovery-done"] button')?.click();
    expect(store.clearRecoveryCodes).toHaveBeenCalled();
  });

  it('lists the actions for an account that already has it', async () => {
    const { fixture, root } = setup(ON);
    await settle(fixture);
    expect(root.textContent).toContain('Activada');
    expect(root.textContent).toContain('8 códigos de recuperación');
    expect(root.querySelector('[data-testid="two-factor-disable"]')).not.toBeNull();
  });
});
