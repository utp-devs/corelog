import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { RouterStateSnapshot } from '@angular/router';
import { describe, expect, it, vi } from 'vitest';

import {
  UNSAVED_CHANGES_PROMPT,
  UnsavedChangesAware,
  unsavedChangesGuard,
} from './unsaved-changes.guard';

const state = (url: string): RouterStateSnapshot => ({ url }) as RouterStateSnapshot;

function run(component: UnsavedChangesAware | null, current: string, next: string): unknown {
  return TestBed.runInInjectionContext(() =>
    unsavedChangesGuard(component, {} as never, state(current), state(next)),
  );
}

describe('unsavedChangesGuard', () => {
  it('lets navigation through when the routed component is null', () => {
    const prompt = vi.fn(() => Promise.resolve(false));
    TestBed.configureTestingModule({
      providers: [{ provide: UNSAVED_CHANGES_PROMPT, useValue: prompt }],
    });

    expect(run(null, '/a', '/b')).toBe(true);
    expect(prompt).not.toHaveBeenCalled();
  });

  it('lets navigation through when the form is clean', () => {
    TestBed.configureTestingModule({});
    expect(run({ dirty: signal(false) }, '/a', '/b')).toBe(true);
  });

  it('asks before leaving a dirty form and returns the answer', async () => {
    const prompt = vi.fn(() => Promise.resolve(false));
    TestBed.configureTestingModule({
      providers: [{ provide: UNSAVED_CHANGES_PROMPT, useValue: prompt }],
    });

    const result = run({ dirty: signal(true) }, '/a', '/b');

    expect(prompt).toHaveBeenCalledTimes(1);
    await expect(result).resolves.toBe(false);
  });

  it('does not block a change of query only', () => {
    const prompt = vi.fn(() => Promise.resolve(false));
    TestBed.configureTestingModule({
      providers: [{ provide: UNSAVED_CHANGES_PROMPT, useValue: prompt }],
    });

    expect(run({ dirty: signal(true) }, '/a', '/a?tab=2')).toBe(true);
    expect(prompt).not.toHaveBeenCalled();
  });
});
