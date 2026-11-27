import { describe, expect, it } from 'vitest';

import { PROJECT_TEMPLATES, templateInfo } from './project-templates';
import { isValidKey, projectColorCss, suggestKey } from './project-wizard.utils';

describe('suggestKey', () => {
  it('uses the initials of several words', () => {
    expect(suggestKey('Plataforma de Pagos')).toBe('PDP');
  });

  it('uses the first letters of a single word and drops accents', () => {
    expect(suggestKey('Seguridad')).toBe('SEGU');
    expect(suggestKey('Ágora')).toBe('AGOR');
  });

  it('never starts with a digit and tolerates empty names', () => {
    expect(suggestKey('2024')).toBe('');
    expect(suggestKey('   ')).toBe('');
  });
});

describe('isValidKey', () => {
  it('accepts 2 to 10 upper-case characters starting with a letter', () => {
    expect(isValidKey('AB')).toBe(true);
    expect(isValidKey('A1B2C3D4E5')).toBe(true);
    expect(isValidKey('A')).toBe(false);
    expect(isValidKey('1AB')).toBe(false);
    expect(isValidKey('abc')).toBe(false);
    expect(isValidKey('ABCDEFGHIJK')).toBe(false);
  });
});

describe('projectColorCss', () => {
  it('maps palette names and hex values', () => {
    expect(projectColorCss('palette-3')).toBe('var(--cl-palette-3-bg)');
    expect(projectColorCss('#aabbcc')).toBe('#aabbcc');
    expect(projectColorCss('red')).toBeNull();
    expect(projectColorCss(null)).toBeNull();
  });
});

describe('templates', () => {
  it('offers the four templates and only Scrum has sprints', () => {
    expect(PROJECT_TEMPLATES.map((item) => item.value)).toEqual([
      'scrum',
      'kanban',
      'task_tracking',
      'bug_tracking',
    ]);
    expect(PROJECT_TEMPLATES.filter((item) => item.sprints).map((item) => item.value)).toEqual([
      'scrum',
    ]);
    expect(templateInfo('task_tracking').statuses.length).toBe(3);
  });
});
