import { Location } from '@angular/common';
import { TestBed } from '@angular/core/testing';
import { NavigationError } from '@angular/router';

import {
  CHUNK_RECOVERY_ENV,
  CHUNK_RELOAD_KEY,
  CHUNK_RELOAD_WINDOW_MS,
  ChunkRecovery,
  isChunkLoadError,
  mayReload,
} from './chunk-recovery';

describe('isChunkLoadError', () => {
  it('detects the browser wordings and the bundler name', () => {
    expect(
      isChunkLoadError(new TypeError('Failed to fetch dynamically imported module: /x.js')),
    ).toBe(true);
    expect(isChunkLoadError(new Error('error loading dynamically imported module'))).toBe(true);
    expect(isChunkLoadError(new Error('Importing a module script failed.'))).toBe(true);
    expect(isChunkLoadError({ name: 'ChunkLoadError', message: 'x' })).toBe(true);
  });

  it('ignores other failures', () => {
    expect(isChunkLoadError(new Error('boom'))).toBe(false);
    expect(isChunkLoadError(null)).toBe(false);
    expect(isChunkLoadError(undefined)).toBe(false);
  });
});

describe('mayReload', () => {
  it('allows the first reload and blocks a repeat inside the window', () => {
    expect(mayReload(null, 1000)).toBe(true);
    expect(mayReload('1000', 1000 + CHUNK_RELOAD_WINDOW_MS - 1)).toBe(false);
    expect(mayReload('1000', 1000 + CHUNK_RELOAD_WINDOW_MS + 1)).toBe(true);
  });

  it('treats a garbage or future value as no previous reload', () => {
    expect(mayReload('abc', 1000)).toBe(true);
    expect(mayReload('5000', 1000)).toBe(true);
  });
});

describe('ChunkRecovery', () => {
  function setup(stored: string | null) {
    let value = stored;
    const reload = vi.fn();
    TestBed.configureTestingModule({
      providers: [
        {
          provide: CHUNK_RECOVERY_ENV,
          useValue: {
            now: () => 100_000,
            read: () => value,
            write: (v: string) => (value = v),
            reload,
          },
        },
        {
          provide: Location,
          useValue: { prepareExternalUrl: (url: string) => `/workspace${url}` },
        },
      ],
    });
    return { service: TestBed.inject(ChunkRecovery), reload, stored: () => value };
  }
  const chunkError = () =>
    new NavigationError(
      1,
      '/projects/CL/reports',
      new TypeError('Failed to fetch dynamically imported module'),
    );

  it('reloads once on the target url and remembers it', () => {
    const { service, reload, stored } = setup(null);
    expect(service.handle(chunkError())).toBe(true);
    expect(reload).toHaveBeenCalledWith('/workspace/projects/CL/reports');
    expect(stored()).toBe('100000');
    expect(service.stale()).toBe(false);
    expect(CHUNK_RELOAD_KEY).toBeTruthy();
  });

  it('shows the notice instead of looping when it already reloaded', () => {
    const { service, reload } = setup('99000');
    expect(service.handle(chunkError())).toBe(true);
    expect(reload).not.toHaveBeenCalled();
    expect(service.stale()).toBe(true);
  });

  it('leaves other navigation errors alone', () => {
    const { service, reload } = setup(null);
    expect(service.handle(new NavigationError(1, '/x', new Error('nope')))).toBe(false);
    expect(reload).not.toHaveBeenCalled();
    expect(service.stale()).toBe(false);
  });
});
