import { Route } from '@angular/router';
import { describe, expect, it } from 'vitest';

import { ONBOARDING_ROUTES, ORG_ADMIN_ROUTES } from './organizations.routes';
import { OnboardingPage } from './onboarding/onboarding-page';
import { OrgInvitationsPage } from './invitations/org-invitations-page';
import { OrgMembersPage } from './members/org-members-page';
import { OrgSettingsPage } from './settings/org-settings-page';

async function loaded(route: Route): Promise<unknown> {
  return route.loadComponent?.();
}

describe('ORG_ADMIN_ROUTES', () => {
  it('mounts settings, members and invitations with Spanish titles', () => {
    expect(ORG_ADMIN_ROUTES.map((route) => route.path)).toEqual([
      'settings',
      'members',
      'invitations',
    ]);
    expect(ORG_ADMIN_ROUTES.map((route) => route.title)).toEqual([
      'Configuración de la organización',
      'Miembros',
      'Invitaciones',
    ]);
  });

  it('lazy-loads each page component', async () => {
    expect(await loaded(ORG_ADMIN_ROUTES[0])).toBe(OrgSettingsPage);
    expect(await loaded(ORG_ADMIN_ROUTES[1])).toBe(OrgMembersPage);
    expect(await loaded(ORG_ADMIN_ROUTES[2])).toBe(OrgInvitationsPage);
  });

  it('guards unsaved settings edits on leave', () => {
    expect(ORG_ADMIN_ROUTES[0].canDeactivate).toHaveLength(1);
  });
});

describe('ONBOARDING_ROUTES', () => {
  it('serves the wizard at the empty path, lazy-loaded', async () => {
    expect(ONBOARDING_ROUTES[0].path).toBe('');
    expect(ONBOARDING_ROUTES[0].pathMatch).toBe('full');
    expect(ONBOARDING_ROUTES[0].title).toBe('Asistente de alta');
    expect(await loaded(ONBOARDING_ROUTES[0])).toBe(OnboardingPage);
  });
});
