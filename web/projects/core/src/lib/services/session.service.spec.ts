import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting, recordLocation } from '../../testing';
import { AUTH_TOKEN_KEY, SessionService } from './session.service';

describe('SessionService', () => {
  beforeEach(() => localStorage.clear());

  it('stores the token under the key shared with the React app', () => {
    TestBed.configureTestingModule({ providers: [provideCorelogTesting()] });
    const session = TestBed.inject(SessionService);
    session.startSession('token-1');
    expect(localStorage.getItem('corelog.auth.token')).toBe('token-1');
    expect(session.isAuthenticated()).toBe(true);
    expect(session.token()).toBe('token-1');
  });

  it('expire clears the token and redirects to login with returnUrl', () => {
    const recorder = recordLocation();
    TestBed.configureTestingModule({ providers: [provideCorelogTesting(recorder)] });
    const session = TestBed.inject(SessionService);
    session.startSession('token-1');
    session.expire();
    expect(localStorage.getItem(AUTH_TOKEN_KEY)).toBeNull();
    expect(session.isAuthenticated()).toBe(false);
    expect(recorder.urls[0]).toMatch(/^\/login\?returnUrl=%2F/);
  });
});
