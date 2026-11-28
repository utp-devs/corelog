import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { RouterLink } from '@angular/router';
import type { SavedQuery } from '@corelog/core';
import { AppTooltip } from '@corelog/ui/tooltip';

interface FilterGroup {
  readonly key: string;
  readonly title: string;
  readonly filters: readonly SavedQuery[];
}

/** Saved filters next to the navigator: own, shared with the user and starred. */
@Component({
  selector: 'app-filters-sidebar',
  imports: [AppTooltip, RouterLink],
  templateUrl: './filters-sidebar.html',
  styleUrl: './filters-sidebar.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class FiltersSidebar {
  readonly filters = input<readonly SavedQuery[]>([]);
  readonly activeId = input<string | null>(null);
  readonly loading = input(false);
  readonly error = input<string | null>(null);
  readonly select = output<SavedQuery>();
  readonly toggleSubscription = output<SavedQuery>();

  protected readonly groups = computed<FilterGroup[]>(() => {
    const all = this.filters();
    const groups: FilterGroup[] = [
      { key: 'mine', title: 'Mis filtros', filters: all.filter((f) => f.access === 'owner') },
      {
        key: 'shared',
        title: 'Compartidos conmigo',
        filters: all.filter((f) => f.access !== 'owner'),
      },
    ];
    return groups.filter((group) => group.filters.length > 0);
  });
}
