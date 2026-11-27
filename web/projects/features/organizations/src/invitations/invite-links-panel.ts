import { ChangeDetectionStrategy, Component, input, output, inject } from '@angular/core';
import { Button } from 'primeng/button';
import { InvitationCreated, NotificationService, ROLE_LABELS, OrgRole } from '@corelog/core';
import { Section } from '@corelog/ui/section';

import { copyText } from '../shared/copy-text';

/**
 * Accept links of the invitations just created or resent. The plain token is only returned once by the
 * API, so this panel explains that the link cannot be shown again.
 */
@Component({
  selector: 'app-invite-links-panel',
  imports: [Button, Section],
  templateUrl: './invite-links-panel.html',
  styleUrl: './invite-links-panel.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class InviteLinksPanel {
  private readonly notify = inject(NotificationService);

  readonly invitations = input.required<InvitationCreated[]>();
  readonly dismissed = output<void>();

  protected roleLabel(role: OrgRole): string {
    return ROLE_LABELS[role];
  }

  protected async copy(url: string): Promise<void> {
    if (await copyText(url)) {
      this.notify.success('Enlace copiado');
    } else {
      this.notify.error('No se pudo copiar. Selecciona el enlace y cópialo manualmente.');
    }
  }
}
