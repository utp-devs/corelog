import { ChangeDetectionStrategy, Component, computed, inject } from '@angular/core';
import { RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { CurrentProjectStore, projectPath } from '@corelog/core';
import { ErrorPage } from '@corelog/ui/error-page';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader } from '@corelog/ui/page-header';
import { Message } from 'primeng/message';

import { ProjectSettingsContext } from './project-settings-context';

interface SettingsSection {
  readonly path: string;
  readonly label: string;
  readonly icon: string;
  readonly danger?: boolean;
}

/** Sections of the settings area, in navigation order. */
export const SETTINGS_SECTIONS: readonly SettingsSection[] = [
  { path: 'details', label: 'Detalles', icon: 'pi pi-pencil' },
  { path: 'access', label: 'Acceso', icon: 'pi pi-users' },
  { path: 'roles', label: 'Roles', icon: 'pi pi-id-card' },
  { path: 'permissions', label: 'Permisos', icon: 'pi pi-lock' },
  { path: 'issue-types', label: 'Tipos de incidencia', icon: 'pi pi-tags' },
  { path: 'workflow', label: 'Flujo de trabajo', icon: 'pi pi-sitemap' },
  { path: 'danger', label: 'Zona de peligro', icon: 'pi pi-exclamation-triangle', danger: true },
];

/**
 * Frame of the project settings: title, notices that apply to every section and the section menu.
 * Only people who work in the project reach it; observers and outsiders see the forbidden state.
 */
@Component({
  selector: 'app-project-settings-shell',
  imports: [
    RouterLink,
    RouterLinkActive,
    RouterOutlet,
    PageContainer,
    PageHeader,
    ErrorPage,
    LoadingSkeleton,
    Message,
  ],
  providers: [ProjectSettingsContext],
  templateUrl: './project-settings-shell.html',
  styleUrl: './project-settings-shell.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProjectSettingsShell {
  private readonly current = inject(CurrentProjectStore);
  protected readonly context = inject(ProjectSettingsContext);

  protected readonly sections = SETTINGS_SECTIONS;

  protected readonly loaded = computed(
    () => this.current.project() !== null && this.current.permissionsLoaded(),
  );

  /** Leads, people who manage members and contributors open the area; the rest has no business here. */
  protected readonly allowed = computed(
    () =>
      this.context.canEdit() ||
      this.context.canManageMembers() ||
      this.context.role() === 'lead' ||
      this.context.role() === 'contributor',
  );

  protected readonly readOnly = computed(
    () => !this.context.canEdit() && !this.context.canManageMembers(),
  );

  protected linkOf(section: SettingsSection): string {
    const project = this.current.project();
    return project ? projectPath(project, `settings/${section.path}`) : '.';
  }
}
