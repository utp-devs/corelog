import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { AuthStore, OrganizationStore } from '@corelog/core';
import { AppShell, AppShellOrgSwitcher } from '@corelog/ui/app-shell';
import { OrgSwitcher } from '@corelog/ui/org-switcher';

import { HELP_LINKS, WORKSPACE_NAV } from '../nav.config';

/** Workspace shell: the shared shell with the workspace navigation and the organization switcher. */
@Component({
  selector: 'app-workspace-shell',
  imports: [AppShell, AppShellOrgSwitcher, OrgSwitcher],
  templateUrl: './workspace-shell.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class WorkspaceShell {
  protected readonly navConfig = WORKSPACE_NAV;
  protected readonly help = HELP_LINKS;
  protected readonly canCreate = inject(AuthStore).isAuthenticated;
  protected readonly org = inject(OrganizationStore).currentOrg;
}
