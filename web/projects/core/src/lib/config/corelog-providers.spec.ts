import { ApplicationInitStatus } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { afterEach, describe, expect, it } from 'vitest';

import { AuthStore } from '../state/auth.store';
import { provideCorelog } from './corelog-providers';

describe('provideCorelog', () => {
  afterEach(() => {
    window.localStorage.clear();
    TestBed.resetTestingModule();
  });

  it('runs the session check before the first navigation', async () => {
    TestBed.configureTestingModule({
      providers: [provideCorelog({ app: 'portal', apiUrl: '/api/v1', routes: [] })],
    });

    await TestBed.inject(ApplicationInitStatus).donePromise;

    expect(TestBed.inject(AuthStore).sessionChecked()).toBe(true);
  });
});
