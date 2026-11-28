/** One dispatched Server-Sent Event. */
export interface SseEvent {
  /** Event name; `message` when the frame carries none. */
  readonly event: string;
  readonly data: string;
  /** Last event id announced by the frame, when it had one. */
  readonly id: string | null;
}

/** Content of a `retry:` field, in milliseconds. */
export interface SseRetry {
  readonly retry: number;
}

/**
 * Incremental parser of a `text/event-stream` body. Feed it decoded text in any chunking: lines may
 * be cut anywhere, end in LF, CRLF or CR, and a frame is dispatched at its blank line. Comments
 * (`: ping`) are ignored; several `data:` lines are joined with a line feed.
 */
export class SseParser {
  private buffer = '';
  private skipLeadingLf = false;
  private event = '';
  private data: string[] = [];
  private id: string | null = null;
  private hasData = false;

  /** Reports each `retry:` value found, so the client can adopt the server pace. */
  constructor(private readonly onRetry: (ms: number) => void = () => undefined) {}

  /** Consumes a chunk and returns the events completed by it. */
  push(chunk: string): SseEvent[] {
    let text = chunk;
    if (this.skipLeadingLf && text.startsWith('\n')) {
      text = text.slice(1);
    }
    this.skipLeadingLf = false;
    this.buffer += text;

    const events: SseEvent[] = [];
    let start = 0;
    for (let i = 0; i < this.buffer.length; i++) {
      const char = this.buffer[i];
      if (char !== '\n' && char !== '\r') continue;
      const line = this.buffer.slice(start, i);
      if (char === '\r') {
        if (i + 1 < this.buffer.length) {
          if (this.buffer[i + 1] === '\n') i++;
        } else {
          // A lone CR at the end of the chunk: the LF of a CRLF may open the next one.
          this.skipLeadingLf = true;
        }
      }
      start = i + 1;
      const event = this.line(line);
      if (event) events.push(event);
    }
    this.buffer = this.buffer.slice(start);
    return events;
  }

  private line(line: string): SseEvent | null {
    if (line === '') return this.dispatch();
    if (line.startsWith(':')) return null;
    const colon = line.indexOf(':');
    const field = colon === -1 ? line : line.slice(0, colon);
    let value = colon === -1 ? '' : line.slice(colon + 1);
    if (value.startsWith(' ')) value = value.slice(1);

    switch (field) {
      case 'event':
        this.event = value;
        break;
      case 'data':
        this.data.push(value);
        this.hasData = true;
        break;
      case 'id':
        if (!value.includes('\0')) this.id = value;
        break;
      case 'retry':
        if (/^\d+$/.test(value)) this.onRetry(Number(value));
        break;
      default:
        break;
    }
    return null;
  }

  private dispatch(): SseEvent | null {
    const result = this.hasData
      ? { event: this.event || 'message', data: this.data.join('\n'), id: this.id }
      : null;
    this.event = '';
    this.data = [];
    this.hasData = false;
    this.id = null;
    return result;
  }
}
