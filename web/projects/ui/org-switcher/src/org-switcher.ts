import { ChangeDetectionStrategy, Component, computed, inject } from '@angular/core';
import { LOCATION_ASSIGN, OrganizationStore, canManageOrg } from '@corelog/core';
import { DropdownMenu, ShellIcon } from '@corelog/ui/app-shell';
import { OrgLogo } from '@corelog/ui/org-logo';
import { AppTooltip } from '@corelog/ui/tooltip';

/**
 * Top bar dropdown: the caller's organizations (current one checked), create, and organization
 * settings for owners and admins. Renders nothing until an organization is active. With a single
 * organization the list has one checked row, so the menu still shows where the user is.
 */
@Component({
  selector: 'app-org-switcher',
  imports: [AppTooltip, DropdownMenu, OrgLogo, ShellIcon],
  templateUrl: './org-switcher.html',
  styleUrl: './org-switcher.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class OrgSwitcher {
  private readonly store = inject(OrganizationStore);
  /** The organization pages live in other applications: each move is a full-page navigation. */
  private readonly assign = inject(LOCATION_ASSIGN);

  protected readonly org = this.store.currentOrg;
  protected readonly orgs = this.store.orgs;
  protected readonly canManage = computed(() => canManageOrg(this.store.currentRole()));

  protected readonly logoRevision = computed(() => {
    const id = this.store.currentOrgId();
    return id ? (this.store.logoRevision()[id] ?? 0) : 0;
  });

  protected revisionOf(id: string): number {
    return this.store.logoRevision()[id] ?? 0;
  }

  protected switchTo(id: string): void {
    if (id === this.store.currentOrgId()) {
      return;
    }
    this.store.setCurrentOrg(id);
    this.assign('/workspace/projects');
  }

  protected createOrganization(): void {
    this.assign('/onboarding?new=1');
  }

  protected openSettings(): void {
    this.assign('/admin/organization/settings');
  }
}
