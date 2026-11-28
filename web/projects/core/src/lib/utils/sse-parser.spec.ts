import { SseParser } from './sse-parser';

describe('SseParser', () => {
  it('parses a frame with event, id and data', () => {
    const parser = new SseParser();
    expect(parser.push('id: 7\nevent: notification\ndata: {"a":1}\n\n')).toEqual([
      { event: 'notification', data: '{"a":1}', id: '7' },
    ]);
  });

  it('defaults the event name to message', () => {
    expect(new SseParser().push('data: x\n\n')).toEqual([
      { event: 'message', data: 'x', id: null },
    ]);
  });

  it('joins several data lines with a line feed', () => {
    expect(new SseParser().push('data: one\ndata: two\n\n')[0].data).toBe('one\ntwo');
  });

  it('ignores comments and frames without data', () => {
    const parser = new SseParser();
    expect(parser.push(': ping\n\n')).toEqual([]);
    expect(parser.push('event: x\n\n')).toEqual([]);
    expect(parser.push(': c\ndata: y\n\n')).toHaveLength(1);
  });

  it('reports retry values', () => {
    const retries: number[] = [];
    new SseParser((ms) => retries.push(ms)).push('retry: 3000\n\nretry: abc\n\n');
    expect(retries).toEqual([3000]);
  });

  it('handles frames split at any boundary', () => {
    const text = 'retry: 3000\n\nid: 1\nevent: notification\ndata: {"k":"v"}\n\n: ping\n\n';
    for (let cut = 1; cut < text.length; cut++) {
      const parser = new SseParser();
      const events = [...parser.push(text.slice(0, cut)), ...parser.push(text.slice(cut))];
      expect(events).toEqual([{ event: 'notification', data: '{"k":"v"}', id: '1' }]);
    }
  });

  it('accepts CRLF and CR line endings, also split between chunks', () => {
    const parser = new SseParser();
    expect(parser.push('event: a\r\ndata: 1\r\n\r\n')).toHaveLength(1);
    expect(parser.push('data: 2\r')).toEqual([]);
    expect(parser.push('\n\r\n')).toEqual([{ event: 'message', data: '2', id: null }]);
    expect(parser.push('data: 3\r\r')).toEqual([{ event: 'message', data: '3', id: null }]);
  });

  it('does not leak fields into the next frame', () => {
    const events = new SseParser().push('id: 1\nevent: a\ndata: x\n\ndata: y\n\n');
    expect(events[1]).toEqual({ event: 'message', data: 'y', id: null });
  });
});
