import { HttpErrorResponse } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';
import { of, throwError } from 'rxjs';

import { provideCorelogTesting } from '../../testing';
import type { NotificationScheme } from '../models/notification-scheme.model';
import { NotificationSchemeApiService } from '../services/api/notification-scheme-api.service';
import { NotificationSchemeStore } from './notification-scheme.store';

const SCHEME: NotificationScheme = {
  scope: 'project',
  organizationId: 'o-1',
  projectId: 'p-1',
  isDefault: true,
  updatedAt: null,
  rules: [{ event: 'issue_created', recipients: [], channels: ['in_app'], overridden: false }],
};

function setup(api: Partial<NotificationSchemeApiService>): NotificationSchemeStore {
  TestBed.configureTestingModule({
    providers: [
      provideCorelogTesting(),
      NotificationSchemeStore,
      { provide: NotificationSchemeApiService, useValue: api },
    ],
  });
  return TestBed.inject(NotificationSchemeStore);
}

describe('NotificationSchemeStore', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('loads the scheme of a scope', async () => {
    const store = setup({ get: vi.fn(() => of(SCHEME)) });
    await store.load({ kind: 'project', id: 'p-1' });
    expect(store.status()).toBe('ready');
    expect(store.scheme()).toEqual(SCHEME);
  });

  it('keeps the failure of a load and flags a forbidden answer', async () => {
    const store = setup({
      get: vi.fn(() => throwError(() => new HttpErrorResponse({ status: 403 }))),
    });
    await store.load({ kind: 'project', id: 'p-1' });
    expect(store.status()).toBe('error');
    expect(store.forbidden()).toBe(true);
  });

  it('saves the rules and replaces the scheme with the answer', async () => {
    const saved = { ...SCHEME, isDefault: false };
    const save = vi.fn(() => of(saved));
    const store = setup({ get: vi.fn(() => of(SCHEME)), save });
    await store.load({ kind: 'project', id: 'p-1' });

    expect(await store.save([])).toBe(true);
    expect(save).toHaveBeenCalledWith({ kind: 'project', id: 'p-1' }, []);
    expect(store.scheme()).toEqual(saved);
    expect(store.saving()).toBe(false);
  });

  it('reports a failed save with the normalized error and keeps the previous scheme', async () => {
    const store = setup({
      get: vi.fn(() => of(SCHEME)),
      save: vi.fn(() =>
        throwError(
          () =>
            new HttpErrorResponse({
              status: 422,
              error: {
                error: { code: 'notification.scheme_invalid', message: 'Esquema inválido' },
              },
            }),
        ),
      ),
    });
    await store.load({ kind: 'project', id: 'p-1' });

    expect(await store.save([])).toBe(false);
    expect(store.error()?.code).toBe('notification.scheme_invalid');
    expect(store.scheme()).toEqual(SCHEME);
  });

  it('resets the scheme', async () => {
    const reset = vi.fn(() => of(SCHEME));
    const store = setup({ get: vi.fn(() => of(SCHEME)), reset });
    await store.load({ kind: 'organization', id: 'o-1' });
    expect(await store.reset()).toBe(true);
    expect(reset).toHaveBeenCalledWith({ kind: 'organization', id: 'o-1' });
  });

  it('does nothing when no scope was loaded', async () => {
    const store = setup({});
    expect(await store.save([])).toBe(false);
  });
});
