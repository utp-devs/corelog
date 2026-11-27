import { Injectable, computed, inject } from '@angular/core';

import type { MyDashboard } from '../models/dashboard.model';
import { DashboardApiService } from '../services/api/dashboard-api.service';
import { AsyncState } from './async-state';
import { resetOnLogout } from './session-scoped';

const STATUS_LABELS: Readonly<Record<string, string>> = {
  open: 'Abiertas',
  in_progress: 'En curso',
  resolved: 'Resueltas',
  closed: 'Cerradas',
};

const MESSAGE_LOAD_FAILED = 'No se pudo cargar tu resumen. Intenta de nuevo.';

/** One bar of the "by status" chart. `percent` is the share of all assigned issues (0 when none). */
export interface DashboardStatusRow {
  readonly status: string;
  readonly label: string;
  readonly count: number;
  readonly percent: number;
}

/** Personal summary of the signed-in user: assigned work, urgent issues and active sprints. */
@Injectable({ providedIn: 'root' })
export class DashboardStore {
  private readonly api = inject(DashboardApiService);

  readonly summary = new AsyncState<MyDashboard>();

  readonly statusRows = computed<readonly DashboardStatusRow[]>(() => {
    const summary = this.summary.value();
    if (!summary) {
      return [];
    }
    const rows = Object.entries(summary.assignedByStatus);
    const total = rows.reduce((sum, [, count]) => sum + count, 0);
    return rows.map(([status, count]) => ({
      status,
      label: STATUS_LABELS[status] ?? status,
      count,
      percent: total > 0 ? (count / total) * 100 : 0,
    }));
  });

  /** Without assigned work or recent issues the page shows the first steps instead. */
  readonly hasWork = computed(() => {
    const summary = this.summary.value();
    return summary !== undefined && (summary.assignedOpen > 0 || summary.recentIssues.length > 0);
  });

  readonly errorMessage = computed(() => (this.summary.error() ? MESSAGE_LOAD_FAILED : null));

  constructor() {
    resetOnLogout(() => this.reset());
  }

  load(): void {
    this.summary.load(this.api.me());
  }

  retry(): void {
    this.load();
  }

  reset(): void {
    this.summary.reset();
  }
}
