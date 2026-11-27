import { describe, expect, it } from 'vitest';
import { qrModules, qrPath } from './qr-code';

describe('qr-code', () => {
  it('builds a square matrix with the three finder patterns', () => {
    const modules = qrModules(
      'otpauth://totp/CoreLog:ana@example.com?secret=JBSWY3DPEHPK3PXP&issuer=CoreLog',
    );
    const size = modules.length;
    expect(size).toBeGreaterThanOrEqual(21);
    expect(modules.every((row) => row.length === size)).toBe(true);
    // Top-left, top-right and bottom-left 7x7 finder patterns start with a dark corner module.
    expect(modules[0][0]).toBe(true);
    expect(modules[0][size - 1]).toBe(true);
    expect(modules[size - 1][0]).toBe(true);
    // The centre of a finder pattern (3x3 block) is dark and its ring of 5x5 is light.
    expect(modules[3][3]).toBe(true);
    expect(modules[1][1]).toBe(false);
  });

  it('is deterministic for the same text', () => {
    expect(qrModules('hola')).toEqual(qrModules('hola'));
  });

  it('turns the matrix into a path with one rectangle per dark module', () => {
    const modules = [
      [true, false],
      [false, true],
    ];
    expect(qrPath(modules, 1)).toBe('M1 1h1v1h-1zM2 2h1v1h-1z');
  });
});
