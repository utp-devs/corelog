import { ChangeDetectionStrategy, Component, computed, inject } from '@angular/core';
import { ErrorState } from '@corelog/ui/error-state';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { Lozenge } from '@corelog/ui/lozenge';
import { Section } from '@corelog/ui/section';

import { ProjectSettingsContext } from '../project-settings-context';
import { ROLE_LABELS, ROLE_ORDER, ROLE_SUMMARIES } from '../settings-copy';
import { summarizeRole } from './roles-summary';

/** What each project role can do, explained from the permission scheme of the project. */
@Component({
  selector: 'app-project-roles-page',
  imports: [Section, Lozenge, LoadingSkeleton, ErrorState],
  templateUrl: './project-roles-page.html',
  styleUrl: './project-roles-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProjectRolesPage {
  private readonly context = inject(ProjectSettingsContext);

  protected readonly labels = ROLE_LABELS;
  protected readonly summaries = ROLE_SUMMARIES;
  protected readonly scheme = this.context.permissions.scheme;
  protected readonly isDefault = this.context.permissions.isDefault;
  protected readonly failed = computed(() => this.context.permissions.schemeError() !== null);
  protected readonly loading = computed(
    () => this.context.permissions.schemeLoading() && this.scheme() === null,
  );

  protected readonly roles = computed(() => {
    const scheme = this.scheme();
    return scheme ? ROLE_ORDER.map((role) => summarizeRole(scheme, role)) : [];
  });

  protected retry(): void {
    this.context.reloadProjectData();
  }
}
