import { ChangeDetectionStrategy, Component, computed, inject, input } from '@angular/core';
import { RouterLink } from '@angular/router';
import { IssueTypeRef, NotificationService } from '@corelog/core';
import { IssueTypeIcon } from '@corelog/ui/issue-type-icon';
import { AppTooltip } from '@corelog/ui/tooltip';
import { Tooltip } from 'primeng/tooltip';

/**
 * Issue key with type icon and copy-to-clipboard button. With an `href` the key is a link; without
 * it the key is plain text (lists where the title is the link). `displayKey` shows a shorter form
 * while the full `issueKey` is still what gets copied.
 */
@Component({
  selector: 'app-issue-key',
  imports: [AppTooltip, IssueTypeIcon, RouterLink, Tooltip],
  templateUrl: './issue-key.html',
  styleUrl: './issue-key.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class IssueKey {
  private readonly notifications = inject(NotificationService);

  /** Full issue key, for example CL-120. This is the value that is copied. */
  readonly issueKey = input.required<string>();
  /** Makes the key a link. Leave it out when another element of the row is the link. */
  readonly href = input<string>();
  /** Shorter text shown instead of the full key. */
  readonly displayKey = input<string>();
  /** Summary shown in the tooltip. */
  readonly summary = input<string>();
  readonly type = input<Pick<IssueTypeRef, 'name' | 'icon' | 'color'>>();
  /** Resolved or closed: the key is muted (never struck through). */
  readonly resolved = input(false);
  /** Whether the copy button and the link can take keyboard focus. */
  readonly focusable = input(true);
  readonly dataTestId = input<string | undefined>(undefined, { alias: 'data-testid' });

  protected readonly text = computed(() => this.displayKey() ?? this.issueKey());
  protected readonly tabIndex = computed(() => (this.focusable() ? null : -1));

  /** In-app paths navigate through the router; absolute URLs stay plain links. */
  protected readonly isAppPath = computed(() => {
    const href = this.href() ?? '';
    return href.startsWith('/') && !href.startsWith('//');
  });

  protected async copy(event: Event): Promise<void> {
    event.stopPropagation();
    try {
      await navigator.clipboard.writeText(this.issueKey());
    } catch {
      return;
    }
    this.notifications.success('Clave copiada');
  }
}
