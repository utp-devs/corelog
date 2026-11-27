import { ISSUES_PATH, issuePath } from './issue-paths';

describe('issue paths', () => {
  it('builds router paths without the application base', () => {
    expect(ISSUES_PATH).toBe('/issues');
    expect(issuePath('42')).toBe('/issues/42');
  });

  it('never repeats the workspace base', () => {
    expect(issuePath('42').startsWith('/workspace')).toBe(false);
  });
});
