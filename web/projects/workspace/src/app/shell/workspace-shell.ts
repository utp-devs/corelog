import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { AuthStore, OrganizationStore } from '@corelog/core';
import { AppShell, AppShellOrgSwitcher } from '@corelog/ui/app-shell';
import { OrgSwitcher } from '@corelog/ui/org-switcher';

import { WORKSPACE_NAV } from '../nav.config';

/** Workspace shell: the shared shell with the workspace navigation and the organization switcher. */
@Component({
  selector: 'app-workspace-shell',
  imports: [AppShell, AppShellOrgSwitcher, OrgSwitcher],
  template: `
    <app-shell [navConfig]="navConfig" [canCreate]="canCreate()" [org]="org()">
      <app-org-switcher appShellOrgSwitcher />
    </app-shell>
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class WorkspaceShell {
  protected readonly navConfig = WORKSPACE_NAV;
  protected readonly canCreate = inject(AuthStore).isAuthenticated;
  protected readonly org = inject(OrganizationStore).currentOrg;
}
