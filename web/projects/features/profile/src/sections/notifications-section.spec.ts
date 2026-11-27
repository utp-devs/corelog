import { ComponentFixture, TestBed } from '@angular/core/testing';
import { NotificationApiService } from '@corelog/core';
import { provideCorelogTesting } from '@corelog/core/testing';
import { NEVER, Observable, of } from 'rxjs';
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
