import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { RouterLink } from '@angular/router';
import {
  ActivityGadgetItem,
  FavoriteGadgetItem,
  GadgetData,
  RecentProjectGadgetItem,
  UpcomingGadgetItem,
  issuePath,
  normalizeIssueLink,
  projectPath,
} from '@corelog/core';
import { EmptyState } from '@corelog/ui/empty-state';
import { ProjectAvatar } from '@corelog/ui/project-avatar';
import { ToneTag } from '@corelog/ui/tone-tag';

import { FAVORITE_TYPE_LABELS, activityVerb, formatDateTime } from './gadget-format';

const UPCOMING_KINDS: Readonly<Record<string, string>> = {
  sprint_end: 'Fin de sprint',
  sprint_start: 'Inicio de sprint',
  due: 'Vencimiento',
  event: 'Evento',
  version_release: 'Versión',
};

/** Lists of the activity, recent projects, favorites and upcoming events gadgets. */
@Component({
  selector: 'app-gadget-lists',
  imports: [EmptyState, ProjectAvatar, RouterLink, ToneTag],
  templateUrl: './gadget-lists.html',
  styleUrl: './gadget-lists.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class GadgetLists {
  readonly content = input.required<GadgetData>();

  protected readonly activity = computed<readonly ActivityGadgetItem[] | null>(() => {
    const content = this.content();
    return content.type === 'activity_stream' ? content.data.items : null;
  });
  protected readonly projects = computed<readonly RecentProjectGadgetItem[] | null>(() => {
    const content = this.content();
    return content.type === 'recent_projects' ? content.data.items : null;
  });
  protected readonly favorites = computed<readonly FavoriteGadgetItem[] | null>(() => {
    const content = this.content();
    return content.type === 'favorites' ? content.data.items : null;
  });
  protected readonly upcoming = computed<readonly UpcomingGadgetItem[] | null>(() => {
    const content = this.content();
    return content.type === 'calendar_upcoming' ? content.data.items : null;
  });

  protected issueLink(item: ActivityGadgetItem): string {
    return issuePath(item.issueKey);
  }

  protected verb(item: ActivityGadgetItem): string {
    return activityVerb(item);
  }

  protected when(value: string, allDay = false): string {
    return formatDateTime(value, allDay);
  }

  protected projectLink(item: RecentProjectGadgetItem): string {
    return projectPath(item.key, 'board');
  }

  protected favoriteLink(item: FavoriteGadgetItem): string {
    return normalizeIssueLink(item.link || '/projects');
  }

  protected favoriteType(item: FavoriteGadgetItem): string {
    return FAVORITE_TYPE_LABELS[item.type] ?? item.type;
  }

  protected upcomingLink(item: UpcomingGadgetItem): string | null {
    return item.link ? normalizeIssueLink(item.link) : null;
  }

  protected upcomingKind(item: UpcomingGadgetItem): string {
    return UPCOMING_KINDS[item.kind] ?? 'Evento';
  }
}
