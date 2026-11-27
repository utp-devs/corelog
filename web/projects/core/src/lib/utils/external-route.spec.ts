import { TestBed } from '@angular/core/testing';
import { Router, provideRouter } from '@angular/router';
import { describe, expect, it, vi } from 'vitest';

import { LOCATION_ASSIGN } from '../config/navigation';
import { externalPath, externalRoute, sameUrl } from './external-route';

function setup(): { assign: ReturnType<typeof vi.fn>; router: Router } {
  const assign = vi.fn();
  TestBed.configureTestingModule({
    providers: [provideRouter([]), { provide: LOCATION_ASSIGN, useValue: assign }],
  });
  return { assign, router: TestBed.inject(Router) };
}

describe('externalRoute', () => {
  it('hands a fixed absolute path to the browser and cancels the navigation', () => {
    const { assign } = setup();
    const guard = externalRoute('/workspace/projects');
    const result = TestBed.runInInjectionContext(() =>
      guard({} as never, { url: '/projects' } as never),
    );

    expect(assign).toHaveBeenCalledWith('/workspace/projects');
    expect(result).toBe(false);
  });

  it('builds the target from the requested URL when given a function', () => {
    const { assign } = setup();
    const guard = externalRoute((url) => `/admin${url}`);
    TestBed.runInInjectionContext(() => guard({} as never, { url: '/organization' } as never));

    expect(assign).toHaveBeenCalledWith('/admin/organization');
  });
});

describe('externalPath', () => {
  it('creates a full-match leaf route that never renders', () => {
    setup();
    const route = externalPath('settings', '/admin/settings');

    expect(route.path).toBe('settings');
    expect(route.pathMatch).toBe('full');
    expect(route.children).toEqual([]);
    expect(route.canActivate).toHaveLength(1);
  });

  it('matches wildcard paths by prefix', () => {
    setup();
    expect(externalPath('legacy/**', sameUrl).pathMatch).toBeUndefined();
  });
});

describe('sameUrl', () => {
  it('returns the requested URL unchanged', () => {
    expect(sameUrl('/a/b?c=1')).toBe('/a/b?c=1');
  });
});
