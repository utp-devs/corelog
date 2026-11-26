export type TriggerKind = 'user' | 'issue'

export type Trigger = {
  kind: TriggerKind
  /** Index where the trigger starts (the @ or # or the first letter of the key). */
  start: number
  /** Caret position at detection; the replaced range is [start, end). */
  end: number
  query: string
}

const USER_TRIGGER_RE = /(?:^|\s)@([^\s@[\]()]{0,40})$/
const ISSUE_TRIGGER_RE = /(?:^|\s)(?:#([A-Za-z0-9-]{0,20})|([A-Z][A-Z0-9]{1,9}-\d{0,9}))$/

/** Detects an active @ or # / key trigger right before the caret. */
export function detectTrigger(before: string): Trigger | null {
  const user = USER_TRIGGER_RE.exec(before)
  if (user) {
    const start = before.length - user[0].length + (/^\s/.test(user[0]) ? 1 : 0)
    return { kind: 'user', start, end: before.length, query: user[1] }
  }
  const issue = ISSUE_TRIGGER_RE.exec(before)
  if (issue) {
    const start = before.length - issue[0].length + (/^\s/.test(issue[0]) ? 1 : 0)
    const query = issue[1] !== undefined ? issue[1] : (issue[2] ?? '')
    return { kind: 'issue', start, end: before.length, query }
  }
  return null
}
