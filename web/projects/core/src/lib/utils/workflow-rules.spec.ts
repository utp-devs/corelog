import { describe, expect, it } from 'vitest';

import {
  RULE_SPECS,
  describeRule,
  fieldLabel,
  newRule,
  ruleFieldError,
  ruleSpec,
  specsOfKind,
} from './workflow-rules';

describe('workflow rule registry', () => {
  it('covers every condition, validator and post-function the engine knows', () => {
    expect(specsOfKind('condition').map((spec) => spec.type)).toEqual([
      'user_in_role',
      'is_assignee',
      'is_reporter',
      'permission',
      'field_equals',
    ]);
    expect(specsOfKind('validator').map((spec) => spec.type)).toEqual([
      'required_field',
      'comment_required',
      'resolution_required',
      'subtasks_done',
      'assignee_set',
    ]);
    expect(specsOfKind('postFunction').map((spec) => spec.type)).toEqual([
      'set_resolution',
      'clear_resolution',
      'assign',
      'set_field',
      'add_comment',
      'emit',
    ]);
  });

  it('has Spanish labels and unique types', () => {
    expect(new Set(RULE_SPECS.map((spec) => spec.type)).size).toBe(RULE_SPECS.length);
    expect(RULE_SPECS.every((spec) => spec.label !== '' && spec.description !== '')).toBe(true);
  });

  it('builds a new rule with the default value of every parameter', () => {
    expect(newRule('user_in_role').params).toEqual({ roles: ['lead'] });
    expect(newRule('assign').params).toEqual({ to: 'current_user' });
    expect(newRule('set_resolution').params).toEqual({ resolution: 'done' });
    expect(newRule('comment_required').params).toEqual({});
  });

  it('describes a rule with its parameters', () => {
    expect(describeRule({ type: 'user_in_role', params: { roles: ['lead', 'viewer'] } })).toBe(
      'Quien ejecuta tiene un rol: Líder, Observador',
    );
    expect(describeRule({ type: 'required_field', params: { field: 'due_date' } })).toContain(
      'Fecha de vencimiento',
    );
    expect(describeRule({ type: 'comment_required', params: {} })).toBe('Exigir un comentario');
  });

  it('flags the rule forms that miss a value', () => {
    expect(ruleFieldError({ type: 'user_in_role', params: { roles: [] } })).toBe(
      'Elige al menos un rol.',
    );
    expect(ruleFieldError({ type: 'emit', params: { event: '' } })).toContain('nombre del evento');
    expect(ruleFieldError({ type: 'emit', params: { event: 'issue.done' } })).toBeNull();
    expect(ruleFieldError({ type: 'is_assignee', params: {} })).toBeNull();
    expect(ruleFieldError({ type: 'bogus', params: {} })).toBe('Tipo de regla desconocido.');
  });

  it('names standard fields and custom ones', () => {
    expect(fieldLabel('story_points')).toBe('Puntos de historia');
    expect(fieldLabel('cf_0123456789abcdef0123456789abcdef')).toBe('Campo personalizado');
    expect(ruleSpec('emit')?.params[0].key).toBe('event');
  });
});
