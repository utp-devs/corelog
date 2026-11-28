import { sessionIdOfToken } from './session-token';

function token(payload: object): string {
  const part = btoa(JSON.stringify(payload))
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=+$/, '');
  return `h.${part}.s`;
}

describe('sessionIdOfToken', () => {
  it('reads the jti claim', () => {
    expect(sessionIdOfToken(token({ jti: 'abc-123', sub: 'u' }))).toBe('abc-123');
  });

  it('returns null without a token, claim or valid payload', () => {
    expect(sessionIdOfToken(null)).toBeNull();
    expect(sessionIdOfToken('token-1')).toBeNull();
    expect(sessionIdOfToken(token({ sub: 'u' }))).toBeNull();
    expect(sessionIdOfToken('a.%%%.c')).toBeNull();
  });
});
