import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import { SecretReveal } from './secret-reveal';

@Component({
  imports: [SecretReveal],
  template: `<app-secret-reveal [secret]="secret()" label="Token" (dismissed)="onDismiss()" />`,
})
class Host {
  readonly secret = signal<string | null>('cl_pat_abcdef');
  dismissals = 0;
  onDismiss(): void {
    this.dismissals += 1;
    this.secret.set(null);
  }
}

function setup() {
  TestBed.configureTestingModule({ imports: [Host] });
  const fixture = TestBed.createComponent(Host);
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement, host: fixture.componentInstance };
}

describe('SecretReveal', () => {
  it('shows the value read-only with the one-time warning', () => {
    const { el } = setup();
    const field = el.querySelector<HTMLInputElement>('[data-testid="secret-reveal-value"]');
    expect(field?.value).toBe('cl_pat_abcdef');
    expect(field?.readOnly).toBe(true);
    expect(el.textContent).toContain('No volverás a ver');
  });

  it('cannot be closed before confirming the copy', () => {
    const { fixture, el, host } = setup();
    const close = el.querySelector<HTMLButtonElement>('[data-testid="secret-reveal-close"] button');
    expect(close?.disabled).toBe(true);
    close?.click();
    expect(host.dismissals).toBe(0);
    fixture.detectChanges();
    expect(el.querySelector('[data-testid="secret-reveal-value"]')).not.toBeNull();
  });

  it('is cleared from the page once the copy is confirmed and closed', () => {
    const { fixture, el, host } = setup();
    el.querySelector<HTMLInputElement>('[data-testid="secret-reveal-ack"]')?.click();
    fixture.detectChanges();
    el.querySelector<HTMLButtonElement>('[data-testid="secret-reveal-close"] button')?.click();
    fixture.detectChanges();
    expect(host.dismissals).toBe(1);
    expect(el.querySelector('[data-testid="secret-reveal"]')).toBeNull();
    expect(el.textContent).not.toContain('cl_pat_abcdef');
  });

  it('copies the value to the clipboard', async () => {
    const { fixture, el } = setup();
    const write = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText: write },
      configurable: true,
    });
    el.querySelector<HTMLButtonElement>('[data-testid="secret-reveal-copy"] button')?.click();
    await fixture.whenStable();
    expect(write).toHaveBeenCalledWith('cl_pat_abcdef');
  });
});
