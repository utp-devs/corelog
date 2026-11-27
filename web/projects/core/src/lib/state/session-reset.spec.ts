import { provideHttpClient } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../testing';
import { provideCorelogDomain } from '../config/corelog-providers';
import { SessionService } from '../services/session.service';
import { getOrgId } from '../utils/org-context';
import { ORGANIZATION_ID_PROVIDER } from '../utils/org-id-provider';
import { OrganizationStore } from './organization.store';
import { ProjectStore } from './project.store';
import { TeamStore } from './team.store';
import { TicketStore } from './ticket.store';

describe('session reset', () => {
  beforeEach(() => localStorage.clear());

  it('clears the active organization and the stores that depend on it when the session ends', () => {
    TestBed.configureTestingModule({
      providers: [provideCorelogTesting(), provideHttpClient(), provideCorelogDomain()],
    });
    const session = TestBed.inject(SessionService);
    session.startSession('token-1');
    const org = TestBed.inject(OrganizationStore);
    const projects = TestBed.inject(ProjectStore);
    const teams = TestBed.inject(TeamStore);
    const tickets = TestBed.inject(TicketStore);
    TestBed.tick();

    org.setCurrentOrg('o1');
    expect(getOrgId()).toBe('o1');

    session.clear();
    TestBed.tick();

    expect(org.currentOrgId()).toBeNull();
    expect(getOrgId()).toBeNull();
    expect(projects.projects()).toEqual([]);
    expect(teams.teams()).toEqual([]);
    expect(tickets.list.value()).toBeUndefined();
  });

  it('exposes the active organization through the organization id provider token', () => {
    TestBed.configureTestingModule({
      providers: [provideCorelogTesting(), provideHttpClient(), provideCorelogDomain()],
    });
    TestBed.inject(SessionService).startSession('token-1');
    const org = TestBed.inject(OrganizationStore);
    org.setCurrentOrg('o9');
    expect(TestBed.inject(ORGANIZATION_ID_PROVIDER)()).toBe('o9');
  });
});
