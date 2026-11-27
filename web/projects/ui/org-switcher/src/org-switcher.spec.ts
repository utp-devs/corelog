import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideCorelogTesting } from '@corelog/core/testing';
import {
  LOCATION_ASSIGN,
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
  assign: ReturnType<typeof vi.fn>;
} {
  localStorage.clear();
  const assign = vi.fn();
  TestBed.configureTestingModule({
    providers: [
      provideCorelogTesting(),
      {
        provide: OrganizationApiService,
        useValue: { list: () => of(orgs), fetchLogoBlob: () => of(null) },
      },
      { provide: LOCATION_ASSIGN, useValue: assign },
    ],
  });
  const store = TestBed.inject(OrganizationStore);
  TestBed.flushEffects();
  store.fetchOrgs();
  store.setCurrentOrg(currentId);
  const fixture = TestBed.createComponent(OrgSwitcher);
  fixture.detectChanges();
  return { fixture, store, assign };
}

function openMenu(fixture: ComponentFixture<OrgSwitcher>): HTMLElement {
  const trigger = fixture.nativeElement.querySelector(
    'button.org-switcher__trigger',
  ) as HTMLButtonElement;
  trigger.click();
  fixture.detectChanges();
  return document.querySelector('[role="menu"]') as HTMLElement;
}

function clickItem(item: HTMLElement): void {
  item.click();
}

function menuItem(menu: HTMLElement, name: RegExp): HTMLElement {
  const item = Array.from(
    menu.querySelectorAll<HTMLElement>('[role="menuitem"], [role="menuitemradio"]'),
  ).find((el) => name.test(el.textContent ?? ''));
  if (!item) throw new Error(`menu item ${name} not found`);
  return item;
}

describe('OrgSwitcher', () => {
  beforeEach(() => {
    orgs = [org('a', 'Acme', 'owner'), org('b', 'Beta', 'member')];
  });

  it('shows the current organization name on the trigger', () => {
    const { fixture } = setup('a');
    const trigger = fixture.nativeElement.querySelector(
      'button.org-switcher__trigger',
    ) as HTMLButtonElement;
    expect(trigger.getAttribute('aria-label')).toBe('Organización actual: Acme');
    expect(trigger.textContent).toContain('Acme');
  });

  it('lists every organization and marks the current one', () => {
    const { fixture } = setup('a');
    const menu = openMenu(fixture);
    expect(menuItem(menu, /Acme/).getAttribute('aria-checked')).toBe('true');
    expect(menuItem(menu, /Beta/).getAttribute('aria-checked')).toBe('false');
    expect(menu.textContent).toContain('Tus organizaciones');
  });

  it('switches the organization and goes to projects', () => {
    const { fixture, store, assign } = setup('a');
    const menu = openMenu(fixture);
    clickItem(menuItem(menu, /Beta/));
    expect(store.currentOrgId()).toBe('b');
    expect(localStorage.getItem(CURRENT_ORG_STORAGE_KEY)).toBe('b');
    expect(assign).toHaveBeenCalledWith('/workspace/projects');
  });

  it('opens the wizard at the organization step from "Crear organización"', () => {
    const { fixture, assign } = setup('a');
    const menu = openMenu(fixture);
    clickItem(menuItem(menu, /Crear organización/));
    expect(assign).toHaveBeenCalledWith('/onboarding?new=1');
  });

  it('shows organization settings to an owner or admin', () => {
    const { fixture, assign } = setup('a');
    const menu = openMenu(fixture);
    clickItem(menuItem(menu, /Configuración de la organización/));
    expect(assign).toHaveBeenCalledWith('/admin/organization/settings');
  });

  it('works with a single organization: one checked row and the actions', () => {
    orgs = [org('a', 'Acme', 'owner')];
    const { fixture } = setup('a');
    const menu = openMenu(fixture);
    expect(menu.querySelectorAll('[role="menuitemradio"]')).toHaveLength(1);
    expect(menuItem(menu, /Acme/).getAttribute('aria-checked')).toBe('true');
    expect(menuItem(menu, /Crear organización/)).toBeTruthy();
  });

  it('closes on Escape and returns focus to the trigger', () => {
    const { fixture } = setup('a');
    const menu = openMenu(fixture);
    menu.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    fixture.detectChanges();
    expect(document.querySelector('[role="menu"]')).toBeNull();
    expect(document.activeElement).toBe(
      fixture.nativeElement.querySelector('button.org-switcher__trigger'),
    );
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
