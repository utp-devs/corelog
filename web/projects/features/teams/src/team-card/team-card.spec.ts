import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { Team } from '@corelog/core';
import { guardConsole } from '@corelog/core/testing';
import { beforeEach, describe, expect, it } from 'vitest';

import { TeamCard } from './team-card';

const base: Team = {
  id: 't-1',
  organizationId: 'org-1',
  name: 'Plataforma',
  description: 'Infra',
  avatarColor: null,
  createdBy: null,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
};

function render(team: Team): HTMLElement {
  const fixture = TestBed.createComponent(TeamCard);
  fixture.componentRef.setInput('team', team);
  fixture.detectChanges();
  return fixture.nativeElement as HTMLElement;
}

describe('TeamCard', () => {
  const consoleGuard = guardConsole();

  beforeEach(() => {
    TestBed.configureTestingModule({ providers: [provideRouter([])] });
  });

  it('shows the member count with the plural form', () => {
    expect(render({ ...base, memberCount: 4 }).textContent).toContain('4 miembros');
    consoleGuard.expectNoConsoleOutput();
  });

  it('uses the singular for one member and zero when the count is unknown', () => {
    expect(render({ ...base, memberCount: 1 }).textContent).toContain('1 miembro');
    expect(render(base).textContent).toContain('0 miembros');
  });

  it('presents no lead: the caller role is not shown as Responsable on the card', () => {
    expect(render({ ...base, myRole: 'maintainer' }).textContent).not.toContain('Responsable');
    expect(render({ ...base, myRole: 'member' }).textContent).not.toContain('Responsable');
  });

  it('links the whole card to the team page', () => {
    const link = render({ ...base, memberCount: 2 }).querySelector('a[data-testid="team-card"]');
    expect(link?.getAttribute('href')).toBe('/t-1');
    expect(link?.textContent).toContain('Plataforma');
  });
});
