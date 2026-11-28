import {
  ALL_EVENTS,
  EVENT_GROUPS,
  canRedeliver,
  checkWebhookUrl,
  hostOf,
  isEventSelected,
  prettyPayload,
  toggleAll,
  toggleEvent,
} from './webhook-rules';

describe('checkWebhookUrl', () => {
  it('accepts public http and https addresses', () => {
    expect(checkWebhookUrl('https://example.com/hook').valid).toBe(true);
    expect(checkWebhookUrl('http://hooks.example.org:8080/x').valid).toBe(true);
  });

  it('does not complain about an empty field but keeps it invalid', () => {
    expect(checkWebhookUrl('  ')).toEqual({ valid: false, message: null });
  });

  it('rejects other protocols, bad text and credentials', () => {
    expect(checkWebhookUrl('ftp://example.com').valid).toBe(false);
    expect(checkWebhookUrl('example.com/hook').message).toContain('dirección completa');
    expect(checkWebhookUrl('https://user:pw@example.com').valid).toBe(false);
  });

  it('rejects private and local hosts', () => {
    for (const host of [
      'localhost',
      '127.0.0.1',
      '10.0.0.5',
      '192.168.1.9',
      '172.20.1.1',
      'db.internal',
      '[::1]',
    ]) {
      expect(checkWebhookUrl(`http://${host}/x`).message).toContain('privadas');
    }
    expect(checkWebhookUrl('http://172.32.0.1/x').valid).toBe(true);
  });

  it('extracts the host', () => {
    expect(hostOf('https://example.com:8443/a')).toBe('example.com:8443');
    expect(hostOf('no es')).toBe('no es');
  });
});

describe('event selection', () => {
  it('toggles single events', () => {
    expect(toggleEvent([], 'issue.created')).toEqual(['issue.created']);
    expect(toggleEvent(['issue.created'], 'issue.created')).toEqual([]);
  });

  it('collapses to the wildcard when every event is chosen', () => {
    const all = EVENT_GROUPS.flatMap((g) => g.events.map((e) => e.value));
    const almost = all.slice(1);
    expect(toggleEvent(almost, all[0])).toEqual([ALL_EVENTS]);
  });

  it('expands the wildcard when one event is unchecked', () => {
    const next = toggleEvent([ALL_EVENTS], 'issue.created');
    expect(next).not.toContain('issue.created');
    expect(next).not.toContain(ALL_EVENTS);
    expect(next.length).toBeGreaterThan(5);
  });

  it('toggles all', () => {
    expect(toggleAll([])).toEqual([ALL_EVENTS]);
    expect(toggleAll([ALL_EVENTS])).toEqual([]);
    expect(isEventSelected([ALL_EVENTS], 'sprint.started')).toBe(true);
  });
});

describe('deliveries', () => {
  it('only failed and dead deliveries can be redelivered', () => {
    expect(canRedeliver('failed')).toBe(true);
    expect(canRedeliver('dead')).toBe(true);
    expect(canRedeliver('success')).toBe(false);
    expect(canRedeliver('pending')).toBe(false);
  });

  it('pretty prints payloads as text', () => {
    expect(prettyPayload({ a: 1 })).toBe('{\n  "a": 1\n}');
    expect(prettyPayload(undefined)).toBe('');
  });
});
