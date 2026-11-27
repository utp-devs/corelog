import { IssueActivity } from '@corelog/core';
import { describe, expect, it } from 'vitest';

import { describeActivity } from './activity-text';

function activity(overrides: Partial<IssueActivity>): IssueActivity {
  return {
    id: 'a1',
    issueId: 'i1',
    actorId: 'u1',
    action: 'created',
    fieldName: null,
    oldValue: null,
    newValue: null,
    createdAt: '2026-01-02T10:00:00Z',
    ...overrides,
  };
}

const nameOf = (id: string): string => (id === 'u2' ? 'Luis Pérez' : 'Ana García');

describe('describeActivity', () => {
  it('translates the creation', () => {
    expect(describeActivity(activity({}), nameOf)).toBe('creó la incidencia');
  });

  it('names the old and new status in Spanish', () => {
    const text = describeActivity(
      activity({
        action: 'status_changed',
        fieldName: 'status',
        oldValue: 'open',
        newValue: 'in_progress',
      }),
      nameOf,
    );
    expect(text).toBe('cambió el estado de Abierto a En progreso');
  });

  it('writes priority changes with the priority labels', () => {
    const text = describeActivity(
      activity({
        action: 'priority_changed',
        fieldName: 'priority',
        oldValue: 'medium',
        newValue: 'high',
      }),
      nameOf,
    );
    expect(text).toBe('cambió la prioridad de Media a Alta');
  });

  it('writes assignment, change and removal of the assignee with member names', () => {
    const base = { action: 'assignee_changed', fieldName: 'assignee_id' };
    expect(describeActivity(activity({ ...base, newValue: 'u2' }), nameOf)).toBe(
      'asignó la incidencia a Luis Pérez',
    );
    expect(describeActivity(activity({ ...base, oldValue: 'u2' }), nameOf)).toBe(
      'quitó al responsable (Luis Pérez)',
    );
    expect(describeActivity(activity({ ...base, oldValue: 'u1', newValue: 'u2' }), nameOf)).toBe(
      'cambió el responsable de Ana García a Luis Pérez',
    );
  });

  it('infers the field from the action name when the field is missing', () => {
    const text = describeActivity(
      activity({ action: 'title_changed', oldValue: 'A', newValue: 'B' }),
      nameOf,
    );
    expect(text).toBe('cambió el título de A a B');
  });

  it('never shows raw English for unknown actions', () => {
    expect(describeActivity(activity({ action: 'something_weird' }), nameOf)).toBe(
      'hizo un cambio en la incidencia',
    );
  });
});
