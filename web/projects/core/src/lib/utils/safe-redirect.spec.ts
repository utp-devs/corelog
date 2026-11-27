import { DEFAULT_AUTH_LANDING, authPathWithRedirect, safeRedirect } from './safe-redirect';

describe('safeRedirect', () => {
  it('keeps in-app paths, query and hash', () => {
    expect(safeRedirect('/invite/abc')).toBe('/invite/abc');
    expect(safeRedirect('/projects?x=1#top')).toBe('/projects?x=1#top');
  });

  it('falls back to the landing page for missing or external targets', () => {
    expect(safeRedirect(null)).toBe(DEFAULT_AUTH_LANDING);
    expect(safeRedirect('')).toBe(DEFAULT_AUTH_LANDING);
    expect(safeRedirect('https://evil.example')).toBe(DEFAULT_AUTH_LANDING);
    expect(safeRedirect('//evil.example')).toBe(DEFAULT_AUTH_LANDING);
    expect(safeRedirect('/\\evil.example')).toBe(DEFAULT_AUTH_LANDING);
    expect(safeRedirect('projects')).toBe(DEFAULT_AUTH_LANDING);
  });

  it('uses the given fallback when the target is rejected', () => {
    expect(safeRedirect('//evil.example', '/inicio')).toBe('/inicio');
  });
});

describe('authPathWithRedirect', () => {
  it('encodes the target into the returnUrl param', () => {
    expect(authPathWithRedirect('/login', '/invite/tok?a=1')).toBe(
      '/login?returnUrl=%2Finvite%2Ftok%3Fa%3D1',
    );
  });

  it('returns the bare path without a target', () => {
    expect(authPathWithRedirect('/register', null)).toBe('/register');
  });
});
