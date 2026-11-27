import { ChangeDetectionStrategy, Component, DestroyRef, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Router } from '@angular/router';
import { CurrentProjectStore, NotificationService, PROJECTS_PATH } from '@corelog/core';
import { ConfirmDangerModal } from '@corelog/ui/confirm-danger-modal';
import { ConfirmModal } from '@corelog/ui/confirm-modal';
import { Section } from '@corelog/ui/section';
import { Button } from 'primeng/button';
import { Message } from 'primeng/message';

import { ProjectSettingsContext } from '../project-settings-context';
import { settingsErrorMessage } from '../settings-copy';

/** Archive, restore and delete the project, each with a confirmation that explains what happens. */
@Component({
  selector: 'app-project-danger-page',
  imports: [Section, Button, Message, ConfirmModal, ConfirmDangerModal],
  templateUrl: './project-danger-page.html',
  styleUrl: './project-danger-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProjectDangerPage {
  private readonly context = inject(ProjectSettingsContext);
  private readonly current = inject(CurrentProjectStore);
  private readonly notify = inject(NotificationService);
  private readonly router = inject(Router);
  private readonly destroyRef = inject(DestroyRef);

  protected readonly project = this.context.project;
  protected readonly archived = this.context.archived;
  protected readonly canArchive = this.context.canEdit;
  protected readonly canDelete = this.context.canDelete;

  protected readonly archiveOpen = signal(false);
  protected readonly deleteOpen = signal(false);
  protected readonly busy = signal(false);
  protected readonly failure = signal<string | null>(null);

  protected askArchive(): void {
    this.failure.set(null);
    this.archiveOpen.set(true);
  }

  protected askDelete(): void {
    this.failure.set(null);
    this.deleteOpen.set(true);
  }

  protected toggleArchive(): void {
    const project = this.project();
    if (!project) return;
    const archiving = !this.archived();
    this.busy.set(true);
    const request = archiving
      ? this.context.projects.archiveProject(project.id)
      : this.context.projects.restoreProject(project.id);
    request.pipe(takeUntilDestroyed(this.destroyRef)).subscribe({
      next: (updated) => {
        this.busy.set(false);
        this.archiveOpen.set(false);
        this.current.setProject(updated);
        this.notify.success(archiving ? 'Proyecto archivado' : 'Proyecto restaurado');
      },
      error: (error: unknown) => {
        this.busy.set(false);
        this.archiveOpen.set(false);
        this.failure.set(
          settingsErrorMessage(
            error,
            archiving ? 'No se pudo archivar el proyecto.' : 'No se pudo restaurar el proyecto.',
          ),
        );
      },
    });
  }

  protected deleteProject(): void {
    const project = this.project();
    if (!project) return;
    this.busy.set(true);
    this.context.projects
      .deleteProject(project.id, project.name)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: () => {
          this.busy.set(false);
          this.deleteOpen.set(false);
          this.notify.success(`El proyecto ${project.name} se eliminó`);
          void this.router.navigateByUrl(PROJECTS_PATH);
        },
        error: (error: unknown) => {
          this.busy.set(false);
          this.deleteOpen.set(false);
          this.failure.set(settingsErrorMessage(error, 'No se pudo eliminar el proyecto.'));
        },
      });
  }
}
