import { isValidOrgSlug, parseInvitationToken, slugFromName } from './organization.model';

describe('organization model helpers', () => {
  it('validates the slug rule of the API', () => {
    expect(isValidOrgSlug('acme')).toBe(true);
    expect(isValidOrgSlug('acme-2026')).toBe(true);
    expect(isValidOrgSlug('ac')).toBe(false);
    expect(isValidOrgSlug('-acme')).toBe(false);
    expect(isValidOrgSlug('acme-')).toBe(false);
    expect(isValidOrgSlug('Acme')).toBe(false);
    expect(isValidOrgSlug('a'.repeat(41))).toBe(false);
  });

  it('derives a slug from a name', () => {
    expect(slugFromName('Acme Corp')).toBe('acme-corp');
    expect(slugFromName('Organización Ñandú!')).toBe('organizacion-nandu');
    expect(slugFromName('  --  ')).toBe('');
    expect(slugFromName('x'.repeat(60))).toHaveLength(40);
  });

  it('extracts the token from an accept link or a bare token', () => {
    const token = 'abcdefghijklmnopqrstu-_09';
    expect(parseInvitationToken(`http://localhost:5173/invite/${token}`)).toBe(token);
    expect(parseInvitationToken(`  http://localhost/invite/${token}/  `)).toBe(token);
    expect(parseInvitationToken(token)).toBe(token);
  });

  it('rejects input that is neither a link nor a token', () => {
    expect(parseInvitationToken('')).toBeNull();
    expect(parseInvitationToken('hola')).toBeNull();
    expect(parseInvitationToken('https://example.com/other/abcdefghijklmnopqrstu')).toBeNull();
  });
});
