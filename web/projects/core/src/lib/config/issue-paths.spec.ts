import { BROWSE_PATH, ISSUES_PATH, issuePath, normalizeIssueLink } from './issue-paths';

describe('issue paths', () => {
  it('builds router paths without the application base', () => {
    expect(ISSUES_PATH).toBe('/issues');
    expect(BROWSE_PATH).toBe('/browse');
    expect(issuePath('CORE-12')).toBe('/browse/CORE-12');
    expect(issuePath('42')).toBe('/browse/42');
  });

  it('never repeats the workspace base', () => {
    expect(issuePath('42').startsWith('/workspace')).toBe(false);
  });
});

describe('normalizeIssueLink', () => {
  it('rewrites the legacy detail path to the browse path', () => {
    expect(normalizeIssueLink('/issues/abc-123')).toBe('/browse/abc-123');
    expect(normalizeIssueLink('/issues/abc-123#comments')).toBe('/browse/abc-123#comments');
  });

  it('keeps the list, the new-issue form and other paths unchanged', () => {
    expect(normalizeIssueLink('/issues')).toBe('/issues');
    expect(normalizeIssueLink('/issues/new')).toBe('/issues/new');
    expect(normalizeIssueLink('/projects/p/board')).toBe('/projects/p/board');
  });
});
