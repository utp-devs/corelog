import { Injectable, inject } from '@angular/core';

import type { Report, ReportDescriptor, ReportQuery } from '../models/report.model';
import { ReportApiService } from '../services/api/report-api.service';
import { AsyncState } from './async-state';
import { resetOnLogout } from './session-scoped';

/** Catalog of reports and the report being read. A newer request cancels the one in flight. */
@Injectable({ providedIn: 'root' })
export class ReportStore {
  private readonly api = inject(ReportApiService);

  readonly catalog = new AsyncState<ReportDescriptor[]>();
  readonly report = new AsyncState<Report>();

  constructor() {
    resetOnLogout(() => this.reset());
  }

  loadCatalog(projectId: string): void {
    this.catalog.load(this.api.list(projectId));
  }

  /** Runs a report. The previous result is cleared first so a stale one is never shown for new parameters. */
  run(projectId: string, kind: string, query: ReportQuery): void {
    this.report.reset();
    this.report.load(this.api.get(projectId, kind, query));
  }

  reset(): void {
    this.catalog.reset();
    this.report.reset();
  }
}
