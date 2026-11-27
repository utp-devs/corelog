import { LEAD_LOADING_LABEL, NO_LEAD_LABEL, ProjectLeadMembers, leadNameOf } from './project-lead';

const members: ProjectLeadMembers = new Map([
  ['u1', { name: 'Ana Pérez' }],
  ['u2', { name: '   ' }],
]);

describe('leadNameOf', () => {
  it('returns the name of the lead found in the organization members', () => {
    expect(leadNameOf('u1', members, 'ready')).toBe('Ana Pérez');
  });

  it('falls back when the project has no lead', () => {
    expect(leadNameOf('', members, 'ready')).toBe(NO_LEAD_LABEL);
  });

  it('falls back when the lead is not a member of the organization', () => {
    expect(leadNameOf('u9', members, 'ready')).toBe(NO_LEAD_LABEL);
  });

  it('falls back when the lead has a blank name', () => {
    expect(leadNameOf('u2', members, 'ready')).toBe(NO_LEAD_LABEL);
  });

  it('shows the loading label while the members are requested', () => {
    expect(leadNameOf('u1', new Map(), 'loading')).toBe(LEAD_LOADING_LABEL);
  });

  it('falls back when the members request failed', () => {
    expect(leadNameOf('u1', new Map(), 'error')).toBe(NO_LEAD_LABEL);
  });
});
