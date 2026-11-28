import { ChangeDetectionStrategy, Component, computed, inject } from '@angular/core';
import {
  ORGANIZATION_ID_PROVIDER,
  OrganizationStore,
  type NotificationSchemeScope,
} from '@corelog/core';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader } from '@corelog/ui/page-header';

import { SchemeEditor } from '../scheme-editor/scheme-editor';

/** Default notification scheme of the organization: what projects inherit until they customize it. */
@Component({
  selector: 'app-org-notifications-page',
  imports: [PageContainer, PageHeader, SchemeEditor],
  templateUrl: './org-notifications-page.html',
  styleUrl: './org-notifications-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class OrgNotificationsPage {
  private readonly organizationId = inject(ORGANIZATION_ID_PROVIDER);
  private readonly organizations = inject(OrganizationStore);

  protected readonly scope = computed<NotificationSchemeScope | null>(() => {
    const id = this.organizationId();
    return id ? { kind: 'organization', id } : null;
  });
  protected readonly canEdit = computed(() => {
    const role = this.organizations.currentRole();
    return role === 'owner' || role === 'admin';
  });
}
