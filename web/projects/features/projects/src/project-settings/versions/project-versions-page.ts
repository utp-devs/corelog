import { DatePipe } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  signal,
  untracked,
} from '@angular/core';
import {
  FormControl,
  FormGroup,
  FormsModule,
  ReactiveFormsModule,
  Validators,
} from '@angular/forms';
import {
  CurrentProjectStore,
  NotificationService,
  ReleaseNotes,
  ReleaseVersion,
  VersionApiService,
  asApiError,
  type VersionStatus,
} from '@corelog/core';
import { Dialog } from '@corelog/ui/dialog';
import { ErrorState } from '@corelog/ui/error-state';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { ProgressBar } from '@corelog/ui/progress-bar';
import { RichTextView } from '@corelog/ui/rich-text-view';
import { Section, SectionActions } from '@corelog/ui/section';
import { ToneTag, type Tone } from '@corelog/ui/tone-tag';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';
import { Select } from 'primeng/select';
import { Textarea } from 'primeng/textarea';
import { firstValueFrom } from 'rxjs';

import { ProjectSettingsContext } from '../project-settings-context';

type LoadStatus = 'loading' | 'ready' | 'error';
type MoveChoice = 'keep' | 'new' | 'other';

const STATUS_LABELS: Readonly<Record<VersionStatus, string>> = {
  unreleased: 'Sin publicar',
  released: 'Publicada',
  archived: 'Archivada',
};
const STATUS_TONES: Readonly<Record<VersionStatus, Tone>> = {
  unreleased: 'blue',
  released: 'green',
  archived: 'neutral',
};

/** Versions of the project: progress of their issues, release with the open issues moved, notes and archive. */
@Component({
  selector: 'app-project-versions-page',
  imports: [
    Button,
    DatePipe,
    Dialog,
    ErrorState,
    FormsModule,
    InputText,
    LoadingSkeleton,
    ProgressBar,
    ReactiveFormsModule,
    RichTextView,
    Section,
    SectionActions,
    Select,
    Textarea,
    ToneTag,
  ],
  templateUrl: './project-versions-page.html',
  styleUrl: './project-versions-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProjectVersionsPage {
  private readonly context = inject(ProjectSettingsContext);
  private readonly current = inject(CurrentProjectStore);
  private readonly api = inject(VersionApiService);
  private readonly notify = inject(NotificationService);

  protected readonly status = signal<LoadStatus>('loading');
  protected readonly versions = signal<readonly ReleaseVersion[]>([]);
  protected readonly canManage = computed(
    () => this.current.can('manage_versions') && !this.context.archived(),
  );
  protected readonly busy = signal(false);
  protected readonly error = signal<string | null>(null);

  protected readonly formOpen = signal(false);
  protected readonly editing = signal<ReleaseVersion | null>(null);
  protected readonly form = new FormGroup({
    name: new FormControl('', {
      nonNullable: true,
      validators: [Validators.required, Validators.maxLength(80)],
    }),
    description: new FormControl('', { nonNullable: true }),
    startDate: new FormControl('', { nonNullable: true }),
    releaseDate: new FormControl('', { nonNullable: true }),
  });

  protected readonly releasing = signal<ReleaseVersion | null>(null);
  protected readonly moveChoice = signal<MoveChoice>('keep');
  protected readonly moveTarget = signal<string | null>(null);
  protected readonly releaseDate = new FormControl('', { nonNullable: true });
  protected readonly unresolved = computed(() => {
    const version = this.releasing();
    return version ? Math.max(0, version.progress.total - version.progress.done) : 0;
  });
  protected readonly targets = computed(() =>
    this.versions()
      .filter((version) => version.status === 'unreleased' && version.id !== this.releasing()?.id)
      .map((version) => ({ value: version.id, label: version.name })),
  );

  protected readonly notes = signal<ReleaseNotes | null>(null);
  protected readonly notesOpen = signal(false);
  protected readonly notesLoading = signal(false);
  protected readonly deleting = signal<ReleaseVersion | null>(null);

  constructor() {
    effect(() => {
      const id = this.context.projectId();
      if (id) untracked(() => void this.load(id));
    });
  }

  protected statusLabel(status: VersionStatus): string {
    return STATUS_LABELS[status];
  }

  protected statusTone(status: VersionStatus): Tone {
    return STATUS_TONES[status];
  }

  protected percent(version: ReleaseVersion): number {
    const { total, done } = version.progress;
    return total === 0 ? 0 : Math.round((done / total) * 100);
  }

  protected progressLabel(version: ReleaseVersion): string {
    const { total, done } = version.progress;
    return `${version.name}: ${done} de ${total} incidencias terminadas`;
  }

  protected retry(): void {
    const id = this.context.projectId();
    if (id) void this.load(id);
  }

  // Create and edit.
  protected openForm(version: ReleaseVersion | null = null): void {
    this.editing.set(version);
    this.error.set(null);
    this.form.reset({
      name: version?.name ?? '',
      description: version?.description ?? '',
      startDate: version?.startDate ?? '',
      releaseDate: version?.releaseDate ?? '',
    });
    this.formOpen.set(true);
  }

  protected async saveForm(): Promise<void> {
    const id = this.context.projectId();
    if (!id || this.form.invalid || this.busy()) return;
    const value = this.form.getRawValue();
    const payload = {
      name: value.name.trim(),
      description: value.description.trim(),
      startDate: value.startDate || null,
      releaseDate: value.releaseDate || null,
    };
    await this.run(async () => {
      const editing = this.editing();
      await firstValueFrom(
        editing ? this.api.update(editing.id, payload) : this.api.create(id, payload),
      );
      this.formOpen.set(false);
      this.notify.success(editing ? 'Versión actualizada' : 'Versión creada');
    });
  }

  // Release.
  protected openRelease(version: ReleaseVersion): void {
    this.error.set(null);
    this.moveChoice.set('keep');
    this.moveTarget.set(null);
    this.releaseDate.setValue(new Date().toISOString().slice(0, 10));
    this.releasing.set(version);
  }

  protected async confirmRelease(): Promise<void> {
    const version = this.releasing();
    if (!version || this.busy()) return;
    const choice = this.moveChoice();
    const target = choice === 'new' ? 'new' : choice === 'other' ? this.moveTarget() : null;
    if (choice === 'other' && !target) {
      this.error.set('Elige la versión que recibirá las incidencias.');
      return;
    }
    await this.run(async () => {
      const result = await firstValueFrom(
        this.api.release(
          version.id,
          this.unresolved() > 0 ? target : null,
          this.releaseDate.value || undefined,
        ),
      );
      this.releasing.set(null);
      this.notify.success(
        result.movedIssues > 0
          ? `Versión publicada. ${result.movedIssues} incidencias pasaron a la siguiente.`
          : 'Versión publicada',
      );
    });
  }

  // Notes.
  protected async openNotes(version: ReleaseVersion): Promise<void> {
    this.notes.set(null);
    this.error.set(null);
    this.notesOpen.set(true);
    this.notesLoading.set(true);
    try {
      this.notes.set(await firstValueFrom(this.api.notes(version.id)));
    } catch (error) {
      this.error.set(asApiError(error).message);
    } finally {
      this.notesLoading.set(false);
    }
  }

  // Archive, reopen and delete.
  protected async archive(version: ReleaseVersion): Promise<void> {
    await this.run(async () => {
      await firstValueFrom(this.api.archive(version.id));
      this.notify.success('Versión archivada');
    });
  }

  protected async reopen(version: ReleaseVersion): Promise<void> {
    await this.run(async () => {
      await firstValueFrom(this.api.update(version.id, { reopen: true }));
      this.notify.success('Versión reabierta');
    });
  }

  protected async confirmDelete(): Promise<void> {
    const version = this.deleting();
    if (!version) return;
    await this.run(async () => {
      await firstValueFrom(this.api.delete(version.id));
      this.deleting.set(null);
      this.notify.success('Versión eliminada');
    });
  }

  /** Runs a change, then reloads the list; the failure text of the server stays in the open dialog. */
  private async run(action: () => Promise<void>): Promise<void> {
    this.busy.set(true);
    this.error.set(null);
    try {
      await action();
      const id = this.context.projectId();
      if (id) await this.load(id);
    } catch (error) {
      this.error.set(asApiError(error).message);
    } finally {
      this.busy.set(false);
    }
  }

  private async load(projectId: string): Promise<void> {
    try {
      this.versions.set(await firstValueFrom(this.api.list(projectId)));
      this.status.set('ready');
    } catch {
      this.status.set('error');
    }
  }
}
