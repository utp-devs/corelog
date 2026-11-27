import { ChangeDetectionStrategy, Component, computed, inject, input } from '@angular/core';
import { RouterLink } from '@angular/router';
import { IssueType, NotificationService } from '@corelog/core';
import { IssueTypeIcon } from '@corelog/ui/issue-type-icon';
import { Tooltip } from 'primeng/tooltip';

/** Issue key with type icon, summary tooltip and copy-to-clipboard button. */
@Component({
  selector: 'app-issue-key',
  imports: [IssueTypeIcon, RouterLink, Tooltip],
  templateUrl: './issue-key.html',
  styleUrl: './issue-key.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class IssueKey {
  private readonly notifications = inject(NotificationService);

  /** Issue key, for example CL-120. */
  readonly issueKey = input.required<string>();
  readonly href = input.required<string>();
  /** Summary shown in the tooltip. */
  readonly summary = input<string>();
  readonly type = input<IssueType>();
  /** Resolved or closed: the key is struck through. */
  readonly resolved = input(false);
  readonly dataTestId = input<string | undefined>(undefined, { alias: 'data-testid' });

  /** In-app paths navigate through the router; absolute URLs stay plain links. */
  protected readonly isAppPath = computed(() => {
    const href = this.href();
    return href.startsWith('/') && !href.startsWith('//');
  });

  protected async copy(): Promise<void> {
    try {
      await navigator.clipboard.writeText(this.issueKey());
    } catch {
      return;
    }
    this.notifications.success('Clave copiada');
  }
}
