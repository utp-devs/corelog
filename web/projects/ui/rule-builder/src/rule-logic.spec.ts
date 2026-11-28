import type { AutomationRuleDraft } from '@corelog/core';

import {
  actionOfType,
  addItem,
  duplicateDraft,
  emptyRule,
  moveItem,
  parseList,
  removeAt,
  smartValueProblem,
  summarizeRule,
  triggerOfType,
  validateRule,
} from './rule-logic';

const rule = (patch: Partial<AutomationRuleDraft> = {}): AutomationRuleDraft => ({
  ...emptyRule(),
  name: 'Regla',
  actions: [{ type: 'add_comment', text: 'Hola' }],
  ...patch,
});

describe('list operations', () => {
  it('moves an item up and down and ignores the edges', () => {
    expect(moveItem(['a', 'b', 'c'], 1, -1)).toEqual(['b', 'a', 'c']);
    expect(moveItem(['a', 'b', 'c'], 1, 1)).toEqual(['a', 'c', 'b']);
    const list = ['a', 'b'];
    expect(moveItem(list, 0, -1)).toBe(list);
    expect(moveItem(list, 1, 1)).toBe(list);
  });

  it('adds up to the maximum and removes by index', () => {
    expect(addItem([1, 2], 3, 3)).toEqual([1, 2, 3]);
    expect(addItem([1, 2, 3], 4, 3)).toEqual([1, 2, 3]);
    expect(removeAt([1, 2, 3], 1)).toEqual([1, 3]);
  });

  it('parses comma lists', () => {
    expect(parseList(' Error, Tarea ,, ')).toEqual(['Error', 'Tarea']);
  });
});

describe('summarizeRule', () => {
  it('reads the rule as a sentence', () => {
    const text = summarizeRule(
      rule({
        trigger: { type: 'issue_transitioned', to: 'Hecho' },
        conditions: [{ type: 'field_equals', field: 'priority', value: 'Alta' }],
        actions: [{ type: 'add_comment', text: 'Listo {{issue.key}}' }],
      }),
    );
    expect(text).toBe(
      'Cuando una incidencia pase a «Hecho», si prioridad es «Alta», entonces añadir el comentario «Listo {{issue.key}}».',
    );
  });

  it('joins several actions and omits an empty condition list', () => {
    const text = summarizeRule(
      rule({
        actions: [
          { type: 'add_label', label: 'urgente' },
          { type: 'assign_issue', who: 'lead' },
        ],
      }),
    );
    expect(text).toBe(
      'Cuando se cree una incidencia, entonces añadir la etiqueta «urgente» y asignarla a la persona líder del proyecto.',
    );
  });

  it('shows placeholders for empty values and names people', () => {
    expect(summarizeRule(rule({ actions: [{ type: 'transition_issue', status: '' }] }))).toContain(
      '…',
    );
    const text = summarizeRule(
      rule({ actions: [{ type: 'assign_issue', who: 'user', userId: 'u1' }] }),
      (id) => (id === 'u1' ? 'Ana' : '?'),
    );
    expect(text).toContain('Ana');
  });

  it('describes a scheduled trigger', () => {
    expect(
      summarizeRule(rule({ trigger: { type: 'scheduled', everyMinutes: 60, query: 'a = b' } })),
    ).toMatch(/^Cada 60 minutos, para las incidencias que cumplan «a = b»/);
  });
});

describe('validateRule', () => {
  it('accepts a complete rule', () => {
    expect(validateRule(rule())).toEqual([]);
  });

  it('requires a name and at least one action', () => {
    const paths = validateRule(rule({ name: ' ', actions: [] })).map((p) => p.path);
    expect(paths).toEqual(['name', 'actions']);
  });

  it('checks the typed fields of each action', () => {
    const problems = validateRule(
      rule({
        actions: [
          { type: 'add_comment', text: '' },
          { type: 'assign_issue', who: 'user' },
          { type: 'send_webhook', url: 'ftp://x' },
          { type: 'add_label', label: '' },
        ],
      }),
    );
    expect(problems.map((p) => p.path)).toEqual([
      'actions[0]',
      'actions[1]',
      'actions[2]',
      'actions[3]',
    ]);
  });

  it('validates the interval and query of a scheduled rule', () => {
    const problems = validateRule(
      rule({ trigger: { ...triggerOfType('scheduled'), everyMinutes: 1, query: '' } }),
    );
    expect(problems.filter((p) => p.path === 'trigger')).toHaveLength(2);
  });

  it('rejects unknown smart values and unclosed braces', () => {
    expect(smartValueProblem('Hola {{issue.key}}')).toBeNull();
    expect(smartValueProblem('Hola {{issue.nope}}')).not.toBeNull();
    expect(smartValueProblem('Hola {{issue.key')).not.toBeNull();
    expect(validateRule(rule({ actions: [{ type: 'add_comment', text: '{{x}}' }] }))).toHaveLength(
      1,
    );
  });

  it('requires values in list conditions', () => {
    const problems = validateRule(
      rule({
        conditions: [
          { type: 'field_in', field: 'status', values: [] },
          { type: 'query', query: '' },
        ],
      }),
    );
    expect(problems.map((p) => p.path)).toEqual(['conditions[0]', 'conditions[1]']);
  });
});

describe('duplicateDraft', () => {
  it('copies the definition disabled with a new name', () => {
    const original = rule({ enabled: true, actions: [actionOfType('add_label')] });
    const copy = duplicateDraft(original);
    expect(copy.name).toBe('Regla (copia)');
    expect(copy.enabled).toBe(false);
    expect(copy.actions[0]).not.toBe(original.actions[0]);
  });
});
