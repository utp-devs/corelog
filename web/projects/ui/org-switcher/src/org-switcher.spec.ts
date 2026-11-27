import { ComponentFixture, TestBed } from '@angular/core/testing';
import { Router } from '@angular/router';
import { provideCorelogTesting } from '@corelog/core/testing';
import {
  CURRENT_ORG_STORAGE_KEY,
  Organization,
  OrganizationApiService,
  OrganizationStore,
  OrgRole,
} from '@corelog/core';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { of } from 'rxjs';

import { OrgSwitcher } from './org-switcher';

function org(id: string, name: string, role: OrgRole): Organization {
  return {
    id,
    slug: id,
    name,
    description: '',
    hasLogo: false,
    settings: {
      whoCanCreateProjects: 'members',
      whoCanCreateTeams: 'members',
      defaultProjectVisibility: 'open',
      require2fa: false,
      maxUploadBytes: 1,
      hoursPerDay: 8,
      daysPerWeek: 5,
      timezone: 'UTC',
    },
    role,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  };
}

let orgs: Organization[] = [];

function setup(currentId: string): {
  fixture: ComponentFixture<OrgSwitcher>;
  store: OrganizationStore;
  navigate: ReturnType<typeof vi.fn>;
} {
  localStorage.clear();
  TestBed.configureTestingModule({
    providers: [
      provideCorelogTesting(),
      {
        provide: OrganizationApiService,
        useValue: { list: () => of(orgs), fetchLogoBlob: () => of(null) },
      },
    ],
  });
  const store = TestBed.inject(OrganizationStore);
  TestBed.flushEffects();
  store.fetchOrgs();
  store.setCurrentOrg(currentId);
  const router = TestBed.inject(Router);
  const navigate = vi.spyOn(router, 'navigate').mockResolvedValue(true);
  const fixture = TestBed.createComponent(OrgSwitcher);
  fixture.detectChanges();
  return { fixture, store, navigate };
}

function openMenu(fixture: ComponentFixture<OrgSwitcher>): HTMLElement {
  const trigger = fixture.nativeElement.querySelector('button.trigger') as HTMLButtonElement;
  trigger.click();
  fixture.detectChanges();
  return document.querySelector('[role="menu"]') as HTMLElement;
}

function clickItem(item: HTMLElement): void {
  (item.querySelector<HTMLElement>('.p-menu-item-content') ?? item).click();
}

function menuItem(menu: HTMLElement, name: RegExp): HTMLElement {
  const item = Array.from(menu.querySelectorAll<HTMLElement>('[role="menuitem"]')).find((el) =>
    name.test(el.textContent ?? ''),
  );
  if (!item) throw new Error(`menu item ${name} not found`);
  return item;
}

describe('OrgSwitcher', () => {
  beforeEach(() => {
    orgs = [org('a', 'Acme', 'owner'), org('b', 'Beta', 'member')];
  });

  it('shows the current organization name on the trigger', () => {
    const { fixture } = setup('a');
    const trigger = fixture.nativeElement.querySelector('button.trigger') as HTMLButtonElement;
    expect(trigger.getAttribute('aria-label')).toBe('Organización actual: Acme');
    expect(trigger.textContent).toContain('Acme');
  });

  it('lists every organization and marks the current one', () => {
    const { fixture } = setup('a');
    const menu = openMenu(fixture);
    expect(
      menuItem(menu, /Acme/).querySelector('[aria-label="Organización actual"]'),
    ).not.toBeNull();
    expect(menuItem(menu, /Beta/).querySelector('[aria-label="Organización actual"]')).toBeNull();
  });

  it('switches the organization and goes to projects', () => {
    const { fixture, store, navigate } = setup('a');
    const menu = openMenu(fixture);
    clickItem(menuItem(menu, /Beta/));
    expect(store.currentOrgId()).toBe('b');
    expect(localStorage.getItem(CURRENT_ORG_STORAGE_KEY)).toBe('b');
    expect(navigate).toHaveBeenCalledWith(['/projects']);
  });

  it('opens the wizard at the organization step from "Crear organización"', () => {
    const { fixture, navigate } = setup('a');
    const menu = openMenu(fixture);
    clickItem(menuItem(menu, /Crear organización/));
    expect(navigate).toHaveBeenCalledWith(['/onboarding'], { queryParams: { new: '1' } });
  });

  it('shows organization settings to an owner or admin', () => {
    const { fixture, navigate } = setup('a');
    const menu = openMenu(fixture);
    clickItem(menuItem(menu, /Configuración de la organización/));
    expect(navigate).toHaveBeenCalledWith(['/organization/settings']);
  });

  it('hides organization settings from a member', () => {
    const { fixture } = setup('b');
    const menu = openMenu(fixture);
    expect(
      Array.from(menu.querySelectorAll('[role="menuitem"]')).some((el) =>
        /Configuración de la organización/.test(el.textContent ?? ''),
      ),
    ).toBe(false);
    expect(menuItem(menu, /Crear organización/)).toBeTruthy();
  });
});
