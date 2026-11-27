import {
  ISSUE_PRIORITY_LABELS,
  ISSUE_STATUS_META,
  ISSUE_TYPE_LABELS,
  issueKeyOf,
  shortIssueKeyOf,
} from './ticket-labels';

describe('ticket labels', () => {
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

  it('derives the visible key from the first eight characters of the id', () => {
    expect(issueKeyOf('11111111-1111-4111-8111-111111111111')).toBe('INC-11111111');
    expect(issueKeyOf('ab12')).toBe('INC-AB12');
  });

  it('shortens the key to the prefix and six characters of the identifier', () => {
    expect(shortIssueKeyOf('11111111-1111-4111-8111-111111111111')).toBe('INC-111111');
    expect(shortIssueKeyOf('ab12')).toBe('INC-AB12');
  });
});
