import { TestBed } from '@angular/core/testing';
import { Router, UrlTree } from '@angular/router';
import { Observable, of } from 'rxjs';

import { provideCorelogTesting } from '../../testing';
import { OrgRole } from '../models/organization.model';
import { OrganizationApiService } from '../services/api/organization-api.service';
import { SessionService } from '../services/session.service';
import { createFakeOrganizationApi, orgWith } from '../state/organization-fixtures';
import { requireOrgAdminGuard } from './require-org-admin.guard';

// The guard does not read its arguments, so the route and state are not needed.
const noArgs = undefined as never;

describe('requireOrgAdminGuard', () => {
  function run(role: OrgRole): unknown {
    const api = createFakeOrganizationApi({ list: () => of([orgWith('o1', role)]) });
    TestBed.configureTestingModule({
      providers: [provideCorelogTesting(), { provide: OrganizationApiService, useValue: api }],
    });
    TestBed.inject(SessionService).startSession('token-1');
    let answer: unknown;
    (
      TestBed.runInInjectionContext(() =>
        requireOrgAdminGuard(noArgs, noArgs),
      ) as Observable<unknown>
    ).subscribe((value) => (answer = value));
    return answer;
  }

  beforeEach(() => {
    localStorage.clear();
  });

  it.each<OrgRole>(['owner', 'admin'])('lets an organization %s through', (role) => {
    expect(run(role)).toBe(true);
  });

  it.each<OrgRole>(['member', 'guest'])('sends an organization %s to the 403 page', (role) => {
    const answer = run(role);
    expect(answer).toBeInstanceOf(UrlTree);
    expect(TestBed.inject(Router).serializeUrl(answer as UrlTree)).toBe('/403');
  });
});
