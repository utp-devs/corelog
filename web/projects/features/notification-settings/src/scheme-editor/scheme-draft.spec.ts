import type { ApiError, NotificationRule } from '@corelog/core';

import {
  EVENT_GROUPS,
  addRecipient,
  customize,
  isDirty,
  overrideCount,
  recipientLabel,
  removeRecipient,
  rowErrors,
  setChannel,
  toDraft,
  toInputs,
} from './scheme-draft';

const SAVED: readonly NotificationRule[] = [
  {
    event: 'issue_created',
    recipients: [{ type: 'assignee' }],
    channels: ['in_app'],
    overridden: false,
  },
  {
    event: 'issue_updated',
    recipients: [{ type: 'watchers' }],
    channels: ['in_app', 'email'],
    overridden: true,
  },
];

describe('scheme draft', () => {
  it('groups the 13 events once each', () => {
    const events = EVENT_GROUPS.flatMap((group) => group.events.map((e) => e.event));
    expect(events).toHaveLength(13);
    expect(new Set(events).size).toBe(13);
  });

  it('is not dirty until something changes', () => {
    expect(isDirty(toDraft(SAVED), SAVED)).toBe(false);
  });

  it('customizing an inherited row makes the draft dirty and the row own', () => {
    const draft = customize(toDraft(SAVED), 'issue_created');
    expect(draft[0].overridden).toBe(true);
    expect(isDirty(draft, SAVED)).toBe(true);
    expect(overrideCount(draft)).toBe(2);
  });

  it('editing a channel customizes the row and keeps the other values', () => {
    const draft = setChannel(toDraft(SAVED), 'issue_created', 'email', true);
    expect(draft[0]).toEqual({
      event: 'issue_created',
      recipients: [{ type: 'assignee' }],
      channels: ['in_app', 'email'],
      overridden: true,
    });
    expect(setChannel(draft, 'issue_created', 'email', false)[0].channels).toEqual(['in_app']);
  });

  it('adds a recipient once and removes it again', () => {
    let draft = addRecipient(toDraft(SAVED), 'issue_updated', { type: 'user', id: 'u-1' });
    draft = addRecipient(draft, 'issue_updated', { type: 'user', id: 'u-1' });
    expect(draft[1].recipients).toEqual([{ type: 'watchers' }, { type: 'user', id: 'u-1' }]);

    draft = removeRecipient(draft, 'issue_updated', { type: 'user', id: 'u-1' });
    expect(draft[1].recipients).toEqual([{ type: 'watchers' }]);
    expect(isDirty(draft, SAVED)).toBe(false);
  });

  it('tells recipients of the same type apart by id and role', () => {
    const draft = addRecipient(toDraft(SAVED), 'issue_updated', { type: 'role', role: 'lead' });
    const more = addRecipient(draft, 'issue_updated', { type: 'role', role: 'viewer' });
    expect(more[1].recipients).toHaveLength(3);
  });

  it('ignores the order of channels and recipients when comparing', () => {
    const reordered = toDraft(SAVED).map((rule) => ({
      ...rule,
      channels: [...rule.channels].reverse(),
    }));
    expect(isDirty(reordered, SAVED)).toBe(false);
  });

  it('sends only the rows the scope owns, without the overridden flag', () => {
    expect(toInputs(toDraft(SAVED))).toEqual([
      {
        event: 'issue_updated',
        recipients: [{ type: 'watchers' }],
        channels: ['in_app', 'email'],
      },
    ]);
  });

  it('maps validation details to the rows that were sent', () => {
    const sent = toInputs(customize(toDraft(SAVED), 'issue_created'));
    const failure: ApiError = {
      status: 422,
      code: 'notification.scheme_invalid',
      message: 'Esquema inválido',
      details: [{ field: 'rules[1].recipients', message: 'Demasiados destinatarios' }],
    };
    const mapped = rowErrors(failure, sent);
    expect(mapped.byEvent.get('issue_updated')).toBe('Demasiados destinatarios');
    expect(mapped.general).toBeNull();
  });

  it('keeps a general message when no detail points at a row', () => {
    const mapped = rowErrors(
      { status: 422, code: 'x', message: 'Esquema inválido', details: [] },
      [],
    );
    expect(mapped.general).toBe('Esquema inválido');
  });

  it('labels recipients in plain Spanish', () => {
    const names = { person: () => 'Ana', team: () => 'Soporte' };
    expect(recipientLabel({ type: 'assignee' }, names)).toBe('Responsable');
    expect(recipientLabel({ type: 'user', id: 'u' }, names)).toBe('Ana');
    expect(recipientLabel({ type: 'team', id: 't' }, names)).toBe('Equipo Soporte');
    expect(recipientLabel({ type: 'role', role: 'lead' }, names)).toBe('Líderes del proyecto');
  });
});
