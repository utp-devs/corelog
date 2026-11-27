import { TestBed } from '@angular/core/testing';
import { Observable, Subject, isObservable, of, throwError } from 'rxjs';

import { LocationRecorder, provideCorelogTesting, recordLocation } from '../../testing';
import { ApiError } from '../models/api-error.model';
import { Organization } from '../models/organization.model';
import { OrganizationApiService } from '../services/api/organization-api.service';
import { SessionService } from '../services/session.service';
import { createFakeOrganizationApi, orgWith } from '../state/organization-fixtures';
import { OrganizationStore } from '../state/organization.store';
import { requireOrgGuard } from './require-org.guard';

// The guard does not read its arguments, so the route and state are not needed.
const noArgs = undefined as never;

describe('requireOrgGuard', () => {
  let recorder: LocationRecorder;
  let api: ReturnType<typeof createFakeOrganizationApi>;

  function setup(overrides: Readonly<Record<string, (...args: never[]) => unknown>> = {}): void {
    recorder = recordLocation();
    api = createFakeOrganizationApi(overrides);
    TestBed.configureTestingModule({
      providers: [
        provideCorelogTesting(recorder),
        { provide: OrganizationApiService, useValue: api },
      ],
    });
    TestBed.inject(SessionService).startSession('token-1');
  }

  /** Runs the guard and collects its single answer (the guard may wait for the request). */
  function run(): { result?: unknown; error?: ApiError } {
    const outcome: { result?: unknown; error?: ApiError } = {};
    const answer = TestBed.runInInjectionContext(() => requireOrgGuard(noArgs, noArgs));
    expect(isObservable(answer)).toBe(true);
    (answer as Observable<unknown>).subscribe({
      next: (value) => (outcome.result = value),
      error: (error: ApiError) => (outcome.error = error),
    });
    return outcome;
  }

  beforeEach(() => {
    localStorage.clear();
  });

  it('waits for the organizations to load, then lets the route through', () => {
    const pending = new Subject<Organization[]>();
    setup({ list: () => pending.asObservable() });
    const outcome = run();
    expect(outcome.result).toBeUndefined();
    pending.next([orgWith('o1')]);
    pending.complete();
    expect(outcome.result).toBe(true);
    expect(TestBed.inject(OrganizationStore).currentOrgId()).toBe('o1');
  });

  it('redirects a user without organizations to the onboarding page', () => {
    setup({ list: () => of([]) });
    const outcome = run();
    expect(outcome.result).toBe(false);
    expect(recorder.urls).toEqual(['/onboarding']);
  });

  it('fetches the organizations only once when they are already loaded', () => {
    setup({ list: () => of([orgWith('o1')]) });
    TestBed.inject(OrganizationStore).fetchOrgs();
    expect(api.list).toHaveBeenCalledTimes(1);
    expect(run().result).toBe(true);
    expect(api.list).toHaveBeenCalledTimes(1);
  });

  it('blocks the route when the list fails and keeps the error for a retry', () => {
    const failure: ApiError = {
      status: 500,
      code: 'http.500',
      message: 'Error del servidor.',
      details: [],
    };
    let calls = 0;
    setup({
      list: () => {
        calls += 1;
        return calls === 1 ? throwError(() => failure) : of([orgWith('o1')]);
      },
    });
    const outcome = run();
    expect(outcome.result).toBe(false);
    expect(recorder.urls).toEqual([]);
    expect(TestBed.inject(OrganizationStore).error()?.message).toBe('Error del servidor.');

    TestBed.inject(OrganizationStore).fetchOrgs();
    expect(TestBed.inject(OrganizationStore).loaded()).toBe(true);
    expect(run().result).toBe(true);
  });
});
