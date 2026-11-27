import {
  ISSUE_PRIORITY_LABELS,
  ISSUE_STATUS_META,
  ISSUE_TYPE_LABELS,
  issueKeyOf,
  shortIssueKeyOf,
} from './issue-labels';

describe('issue labels', () => {
  it('maps every status to a Spanish label and a category', () => {
    expect(ISSUE_STATUS_META.open).toEqual({ label: 'Abierto', category: 'todo' });
    expect(ISSUE_STATUS_META.in_progress).toEqual({ label: 'En progreso', category: 'progress' });
    expect(ISSUE_STATUS_META.resolved).toEqual({ label: 'Resuelto', category: 'done' });
    expect(ISSUE_STATUS_META.closed).toEqual({ label: 'Cerrado', category: 'done' });
  });

  it('maps priorities and issue types to Spanish labels', () => {
    expect(ISSUE_PRIORITY_LABELS.critical).toBe('Crítica');
    expect(ISSUE_TYPE_LABELS.epic).toBe('Épica');
    expect(ISSUE_TYPE_LABELS.subtask).toBe('Subtarea');
  });

  it('uses the server key when the payload carries one', () => {
    expect(issueKeyOf({ id: '11111111-1111-4111-8111-111111111111', key: 'CORE-12' })).toBe(
      'CORE-12',
    );
    expect(shortIssueKeyOf({ id: 'ab12', key: 'CORE-12' })).toBe('CORE-12');
  });

  it('falls back to a prefix of the id when there is no key', () => {
    expect(issueKeyOf({ id: '11111111-1111-4111-8111-111111111111' })).toBe('INC-11111111');
    expect(issueKeyOf({ id: 'ab12', key: null })).toBe('INC-AB12');
    expect(shortIssueKeyOf({ id: '11111111-1111-4111-8111-111111111111' })).toBe('INC-111111');
  });
});
