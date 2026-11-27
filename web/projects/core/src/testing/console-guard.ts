import { MockInstance } from 'vitest';

export interface ConsoleGuard {
  /** Fails when console.warn or console.error was called since the test started. */
  expectNoConsoleOutput: () => void;
}

/**
 * Silences console.warn and console.error for the file and records them.
 * Call once at the top level of a describe block, then `expectNoConsoleOutput()` after rendering.
 */
export function guardConsole(): ConsoleGuard {
  let warn: MockInstance<typeof console.warn> | undefined;
  let error: MockInstance<typeof console.error> | undefined;

  beforeEach(() => {
    warn = vi.spyOn(console, 'warn').mockImplementation(() => undefined);
    error = vi.spyOn(console, 'error').mockImplementation(() => undefined);
  });

  afterEach(() => {
    warn?.mockRestore();
    error?.mockRestore();
  });

  return {
    expectNoConsoleOutput() {
      expect(warn?.mock.calls ?? []).toEqual([]);
      expect(error?.mock.calls ?? []).toEqual([]);
    },
  };
}
