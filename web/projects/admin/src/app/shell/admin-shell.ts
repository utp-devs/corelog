import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { OrganizationStore } from '@corelog/core';
import { AppShell, AppShellOrgSwitcher } from '@corelog/ui/app-shell';
import { OrgSwitcher } from '@corelog/ui/org-switcher';

import { ADMIN_BRAND, ADMIN_NAV } from '../nav.config';

/** Admin shell: the shared shell with the admin navigation, brand and the organization switcher. */
@Component({
  selector: 'app-admin-shell',
  imports: [AppShell, AppShellOrgSwitcher, OrgSwitcher],
  template: `
    <app-shell [navConfig]="navConfig" [brand]="brand" [workPages]="false" [org]="org()">
      <app-org-switcher appShellOrgSwitcher />
    </app-shell>
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class AdminShell {
  protected readonly navConfig = ADMIN_NAV;
  protected readonly brand = ADMIN_BRAND;
  protected readonly org = inject(OrganizationStore).currentOrg;
}
