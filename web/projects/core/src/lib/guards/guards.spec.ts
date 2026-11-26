import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting, recordLocation } from '../../testing';
import { SessionService } from '../services/session.service';
import { authGuard } from './auth.guard';
import { guestGuard } from './guest.guard';

// The guards do not read their arguments, so the route and state are not needed.
const noArgs = undefined as never;

describe('route guards', () => {
  beforeEach(() => localStorage.clear());

  it('authGuard lets a signed-in user through', () => {
    const recorder = recordLocation();
    TestBed.configureTestingModule({ providers: [provideCorelogTesting(recorder)] });
    TestBed.inject(SessionService).startSession('token-1');
    const result = TestBed.runInInjectionContext(() => authGuard(noArgs, noArgs));
    expect(result).toBe(true);
    expect(recorder.urls).toEqual([]);
  });

  it('authGuard sends a signed-out user to login and blocks the route', () => {
    const recorder = recordLocation();
    TestBed.configureTestingModule({ providers: [provideCorelogTesting(recorder)] });
    const result = TestBed.runInInjectionContext(() => authGuard(noArgs, noArgs));
    expect(result).toBe(false);
    expect(recorder.urls).toHaveLength(1);
    expect(recorder.urls[0]).toMatch(/^\/login\?returnUrl=/);
  });

  it('guestGuard lets a signed-out user into the login page', () => {
    const recorder = recordLocation();
    TestBed.configureTestingModule({ providers: [provideCorelogTesting(recorder)] });
    expect(TestBed.runInInjectionContext(() => guestGuard(noArgs, noArgs))).toBe(true);
  });

  it('guestGuard sends a signed-in user to the workspace home', () => {
    const recorder = recordLocation();
    TestBed.configureTestingModule({ providers: [provideCorelogTesting(recorder)] });
    TestBed.inject(SessionService).startSession('token-1');
    expect(TestBed.runInInjectionContext(() => guestGuard(noArgs, noArgs))).toBe(false);
    expect(recorder.urls).toEqual(['/workspace/']);
  });
});
