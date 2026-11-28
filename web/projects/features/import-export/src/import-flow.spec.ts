import {
  MAX_IMPORT_BYTES,
  checkImportFile,
  formatBytes,
  importIssuesQuery,
  isFinished,
  pollDelay,
  progressPercent,
} from './import-flow';

describe('checkImportFile', () => {
  it('accepts a normal CSV', () => {
    expect(checkImportFile({ name: 'datos.csv', size: 100 })).toBeNull();
  });

  it('rejects other extensions, empty files and files over 10 MiB', () => {
    expect(checkImportFile({ name: 'datos.xlsx', size: 100 })).toContain('CSV');
    expect(checkImportFile({ name: 'a.csv', size: 0 })).toContain('vacío');
    expect(checkImportFile({ name: 'a.csv', size: MAX_IMPORT_BYTES + 1 })).toContain('10 MiB');
    expect(checkImportFile({ name: 'a.csv', size: MAX_IMPORT_BYTES })).toBeNull();
  });
});

describe('polling', () => {
  it('starts at 2 seconds and slows down to a cap', () => {
    expect(pollDelay(0)).toBe(2000);
    expect(pollDelay(4)).toBe(2000);
    expect(pollDelay(6)).toBe(3000);
    expect(pollDelay(100)).toBe(8000);
  });

  it('knows when a job finished', () => {
    expect(isFinished({ status: 'running' })).toBe(false);
    expect(isFinished({ status: 'done' })).toBe(true);
    expect(isFinished({ status: 'failed' })).toBe(true);
  });

  it('computes progress', () => {
    expect(progressPercent({ status: 'running', processedRows: 25, totalRows: 100 })).toBe(25);
    expect(progressPercent({ status: 'running', processedRows: 0, totalRows: 0 })).toBe(0);
    expect(progressPercent({ status: 'done', processedRows: 3, totalRows: 100 })).toBe(100);
  });
});

describe('helpers', () => {
  it('formats sizes', () => {
    expect(formatBytes(512)).toBe('512 B');
    expect(formatBytes(2048)).toBe('2 KiB');
    expect(formatBytes(5 * 1024 * 1024)).toBe('5 MiB');
  });

  it('builds a relative search for the created issues', () => {
    const now = new Date('2026-01-01T10:10:00Z');
    expect(importIssuesQuery('KAN', '2026-01-01T10:00:00Z', now)).toBe(
      'project = KAN AND created >= -11m ORDER BY created DESC',
    );
  });
});
