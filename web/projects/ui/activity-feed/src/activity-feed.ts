import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';

import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { RichTextView } from '@corelog/ui/rich-text-view';
import { UserAvatar } from '@corelog/ui/user-avatar';

/** One line of a history entry ("Prioridad: Media -> Alta"). */
export interface ActivityLineView {
  readonly label: string;
  readonly from: string | null;
  readonly to: string | null;
  /** Free text replacing the from-to form (links added or removed). */
  readonly text?: string;
}

export interface ActivityItemView {
  readonly id: string;
  readonly kind: 'comment' | 'history';
  readonly actorId: string;
  readonly actorName: string;
  readonly timeText: string;
  readonly timeTitle: string;
  /** Comment body (kind comment). */
  readonly body?: string;
  readonly edited?: boolean;
  /** Changes of one update, grouped under one actor and time (kind history). */
  readonly lines: readonly ActivityLineView[];
}

/**
 * Merged feed of comments and history. History entries show every change of one update together
 * with the readable field names. The "Cargar más" button asks the owner for the next page.
 */
@Component({
  selector: 'app-activity-feed',
  imports: [LoadingSkeleton, RichTextView, UserAvatar],
  templateUrl: './activity-feed.html',
  styleUrl: './activity-feed.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ActivityFeed {
  readonly items = input.required<readonly ActivityItemView[]>();
  readonly loading = input(false);
  readonly loadingMore = input(false);
  readonly hasMore = input(false);
  readonly emptyText = input('Aún no hay actividad');

  readonly loadMore = output<void>();
}
