import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  computed,
  inject,
  input,
  output,
  signal,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { CurrentProjectStore, NotificationService, Project, ProjectStore } from '@corelog/core';
import { Button } from 'primeng/button';

/**
 * Notice shown while a project is archived: it is read-only. Managers (those who may edit the project)
 * get a "Restaurar" action. By default it follows the project of the current route; pages outside the
 * project area pass `archived`, `projectId` and `canRestore` themselves. Renders nothing otherwise.
 */
@Component({
  selector: 'app-archived-notice',
  imports: [Button],
  templateUrl: './archived-notice.html',
  styleUrl: './archived-notice.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ArchivedNotice {
  private readonly current = inject(CurrentProjectStore);
  private readonly projects = inject(ProjectStore);
  private readonly notify = inject(NotificationService);
  private readonly destroyRef = inject(DestroyRef);

  readonly archived = input<boolean | null>(null);
  readonly projectId = input<string | null>(null);
  readonly canRestore = input<boolean | null>(null);
  /** The project after it was restored. */
  readonly restored = output<Project>();

  protected readonly visible = computed(() => this.archived() ?? this.current.isArchived());
  protected readonly manager = computed(
    () => this.canRestore() ?? this.current.can('edit_project'),
  );
  protected readonly busy = signal(false);

  protected restore(): void {
    const id = this.projectId() ?? this.current.projectId();
    if (!id || this.busy()) return;
    this.busy.set(true);
    this.projects
      .restoreProject(id)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (project) => {
          this.busy.set(false);
          this.current.setProject(project);
          this.notify.success('Proyecto restaurado');
          this.restored.emit(project);
        },
        // The store already reports the failure with a toast.
        error: () => this.busy.set(false),
      });
  }
}
