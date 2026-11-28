import { ChangeDetectionStrategy, Component, computed, inject } from '@angular/core';
import { CurrentProjectStore, type NotificationSchemeScope } from '@corelog/core';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { Section } from '@corelog/ui/section';
import { Message } from 'primeng/message';

import { SchemeEditor } from '../scheme-editor/scheme-editor';

/** Notification section of the project settings: who is told about what, and by which channel. */
@Component({
  selector: 'app-project-notifications-page',
  imports: [LoadingSkeleton, Message, SchemeEditor, Section],
  templateUrl: './project-notifications-page.html',
  styleUrl: './project-notifications-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProjectNotificationsPage {
  private readonly current = inject(CurrentProjectStore);

  protected readonly loaded = computed(
    () => this.current.project() !== null && this.current.permissionsLoaded(),
  );
  /** The scheme is only readable by people who can edit the project. */
  protected readonly canEdit = computed(() => this.current.can('edit_project'));
  protected readonly editable = computed(() => this.canEdit() && !this.current.isArchived());
  protected readonly scope = computed<NotificationSchemeScope | null>(() => {
    const id = this.current.projectId();
    return id ? { kind: 'project', id } : null;
  });
}
