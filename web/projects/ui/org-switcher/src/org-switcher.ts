import { ChangeDetectionStrategy, Component, computed, inject, viewChild } from '@angular/core';
import { MenuItem, PrimeTemplate } from 'primeng/api';
import { Menu } from 'primeng/menu';

import { LOCATION_ASSIGN, OrganizationStore, canManageOrg } from '@corelog/core';
import { OrgLogo } from '@corelog/ui/org-logo';

/** Menu entry with a flag for the organization that is active. */
export interface OrgSwitcherItem extends MenuItem {
  readonly current?: boolean;
}

/**
 * Top bar dropdown: the caller's organizations (current one checked), create, and organization
 * settings for owners and admins. Renders nothing until an organization is active.
 */
@Component({
  selector: 'app-org-switcher',
  imports: [Menu, PrimeTemplate, OrgLogo],
  templateUrl: './org-switcher.html',
  styleUrl: './org-switcher.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class OrgSwitcher {
  private readonly store = inject(OrganizationStore);
  /** The organization pages live in other applications: each move is a full-page navigation. */
  private readonly assign = inject(LOCATION_ASSIGN);

  private readonly menu = viewChild.required<Menu>('menu');

  protected readonly org = this.store.currentOrg;
  protected readonly role = this.store.currentRole;

  protected readonly logoRevision = computed(() => {
    const id = this.store.currentOrgId();
    return id ? (this.store.logoRevision()[id] ?? 0) : 0;
  });

  protected readonly items = computed<OrgSwitcherItem[]>(() => {
    const current = this.org();
    if (!current) {
      return [];
    }
    const organizations: OrgSwitcherItem[] = this.store.orgs().map((item) => ({
      id: `org-${item.id}`,
      label: item.name,
      current: item.id === current.id,
      icon: item.id === current.id ? 'pi pi-check' : undefined,
      command: () => this.switchTo(item.id),
    }));
    const entries: OrgSwitcherItem[] = [
      {
        id: 'organizations',
        label: 'Tus organizaciones',
        disabled: true,
        styleClass: 'group-label',
      },
      ...organizations,
      { separator: true },
      {
        id: 'create',
        label: 'Crear organización',
        icon: 'pi pi-plus',
        command: () => this.assign('/onboarding?new=1'),
      },
    ];
    if (canManageOrg(this.role())) {
      entries.push({
        id: 'settings',
        label: 'Configuración de la organización',
        icon: 'pi pi-cog',
        command: () => this.assign('/admin/organization/settings'),
      });
    }
    return entries;
  });

  protected toggle(event: Event): void {
    this.menu().toggle(event);
  }

  private switchTo(id: string): void {
    if (id === this.store.currentOrgId()) {
      return;
    }
    this.store.setCurrentOrg(id);
    this.assign('/workspace/projects');
  }
}
