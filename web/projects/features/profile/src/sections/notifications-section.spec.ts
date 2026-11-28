import { ComponentFixture, TestBed } from '@angular/core/testing';
import { NotificationApiService } from '@corelog/core';
import { provideCorelogTesting } from '@corelog/core/testing';
import { NEVER, Observable, of, throwError } from 'rxjs';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { ProfileNotificationsSection } from './notifications-section';

function setup(
  getPreferences: () => Observable<{ inApp: boolean }>,
): ComponentFixture<ProfileNotificationsSection> {
  TestBed.configureTestingModule({
    providers: [
      provideCorelogTesting(),
      {
        provide: NotificationApiService,
        useValue: { getPreferences, updatePreferences: vi.fn(() => of({ inApp: true })) },
      },
    ],
  });
  const fixture = TestBed.createComponent(ProfileNotificationsSection);
  fixture.detectChanges();
  return fixture;
}

function loadingStatus(el: HTMLElement): HTMLElement | null {
  return el.querySelector<HTMLElement>('.notifications-section__loading');
}

describe('ProfileNotificationsSection', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('shows a spinner and a disabled switch while the preferences load', () => {
    const fixture = setup(() => NEVER);
    const el = fixture.nativeElement as HTMLElement;

    expect(loadingStatus(el)?.textContent).toContain('Cargando preferencias');
    expect(loadingStatus(el)?.querySelector('.pi-spinner')).not.toBeNull();
    expect(el.querySelector<HTMLInputElement>('p-toggleswitch input')?.disabled).toBe(true);
  });

  it('hides the spinner and enables the switch once the preferences arrive', () => {
    const fixture = setup(() => of({ inApp: true }));
    const el = fixture.nativeElement as HTMLElement;

    expect(loadingStatus(el)).toBeNull();
    expect(el.querySelector<HTMLInputElement>('p-toggleswitch input')?.disabled).toBe(false);
  });

  it('hides the spinner when the preferences fail to load', () => {
    const fixture = setup(() => new Observable((subscriber) => subscriber.error(new Error('x'))));
    const el = fixture.nativeElement as HTMLElement;

    expect(loadingStatus(el)).toBeNull();
  });
});

describe('ProfileNotificationsSection email preferences', () => {
  afterEach(() => TestBed.resetTestingModule());

  function setupEmail(
    update: ReturnType<typeof vi.fn>,
  ): ComponentFixture<ProfileNotificationsSection> {
    TestBed.configureTestingModule({
      providers: [
        provideCorelogTesting(),
        {
          provide: NotificationApiService,
          useValue: {
            getPreferences: () => of({ inApp: true, email: 'daily', notifyOwnActions: true }),
            updatePreferences: update,
          },
        },
      ],
    });
    const fixture = TestBed.createComponent(ProfileNotificationsSection);
    fixture.detectChanges();
    return fixture;
  }

  it('shows the saved email mode, the own actions switch and the digest time', async () => {
    const fixture = setupEmail(vi.fn(() => of({ inApp: true })));
    await fixture.whenStable();
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect((fixture.componentInstance as unknown as { emailMode(): string }).emailMode()).toBe(
      'daily',
    );
    expect(el.querySelector('[data-testid="notifications-digest-note"]')?.textContent).toContain(
      '08:00 UTC',
    );
    expect(el.querySelectorAll('p-toggleswitch')).toHaveLength(2);
  });

  it('saves the email mode together with the in-app flag', () => {
    const update = vi.fn(() => of({ inApp: true }));
    const fixture = setupEmail(update);
    (fixture.componentInstance as unknown as { changeEmailMode(m: string): void }).changeEmailMode(
      'off',
    );
    expect(update).toHaveBeenCalledWith(true, { email: 'off' });
  });

  it('restores the previous value and shows the error when the save fails', () => {
    const update = vi.fn(() => throwError(() => new Error('x')));
    const fixture = setupEmail(update);
    (
      fixture.componentInstance as unknown as { toggleOwnActions(c: boolean): void }
    ).toggleOwnActions(false);
    fixture.detectChanges();
    expect((fixture.nativeElement as HTMLElement).querySelector('[role="alert"]')).not.toBeNull();
  });
});
