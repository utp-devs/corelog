import { NO_CURRENT_ORGANIZATION_CODE } from './org-id-provider';
import { getOrgId, setOrgIdProvider, withOrg } from './org-context';

describe('withOrg', () => {
  let activeOrgId: string | null;

  beforeEach(() => {
    activeOrgId = null;
    setOrgIdProvider(() => activeOrgId);
  });

  afterEach(() => {
    setOrgIdProvider(() => null);
  });

  it('adds the active organization id and keeps the given params', () => {
    activeOrgId = 'org-1';
    expect(withOrg({ status: 'open' })).toEqual({ status: 'open', organization_id: 'org-1' });
  });

  it('works without params', () => {
    activeOrgId = 'org-1';
    expect(withOrg()).toEqual({ organization_id: 'org-1' });
  });

  it('reads the id through the registered provider on every call', () => {
    activeOrgId = 'org-1';
    expect(withOrg()).toEqual({ organization_id: 'org-1' });
    activeOrgId = 'org-2';
    expect(withOrg()).toEqual({ organization_id: 'org-2' });
  });

  it('throws a typed org.no_current error when no organization is active', () => {
    let caught: unknown;
    try {
      withOrg();
    } catch (error) {
      caught = error;
    }
    expect(caught).toMatchObject({ status: 0, code: NO_CURRENT_ORGANIZATION_CODE, details: [] });
    expect(NO_CURRENT_ORGANIZATION_CODE).toBe('org.no_current');
  });

  it('reports no active organization when no provider is registered', () => {
    setOrgIdProvider(() => null);
    expect(getOrgId()).toBeNull();
  });
});
