import { TestBed } from '@angular/core/testing';

import {
  NO_CURRENT_ORGANIZATION_CODE,
  ORGANIZATION_ID_PROVIDER,
  noCurrentOrganizationError,
} from './org-id-provider';

describe('ORGANIZATION_ID_PROVIDER', () => {
  it('defaults to a provider that returns no organization', () => {
    const provider = TestBed.inject(ORGANIZATION_ID_PROVIDER);
    expect(provider()).toBeNull();
  });

  it('describes the missing organization with the org.no_current code', () => {
    expect(noCurrentOrganizationError()).toMatchObject({
      status: 0,
      code: NO_CURRENT_ORGANIZATION_CODE,
    });
    expect(NO_CURRENT_ORGANIZATION_CODE).toBe('org.no_current');
  });
});
