import { Injectable, inject } from '@angular/core';
import { Observable, catchError, throwError } from 'rxjs';

import type { ApiError } from '../models/api-error.model';
import type {
  AgileSprint,
  CompleteSprintInput,
  CreateAgileSprintInput,
  SprintCompletion,
  SprintReport,
  StartSprintInput,
  UpdateAgileSprintInput,
} from '../models/sprint-v2.model';
import { SprintV2ApiService } from '../services/api/sprint-v2-api.service';
import { asApiError } from '../utils/api-error';
import { AsyncState } from './async-state';
import { resetOnLogout } from './session-scoped';

/** Spanish message for a failed sprint action, by the code of the server answer. */
export function sprintErrorMessage(error: ApiError): string {
  switch (error.code) {
    case 'sprint.already_active':
      return 'Ya hay un sprint activo en el proyecto. Complétalo antes de iniciar otro.';
    case 'sprint.empty':
      return 'El sprint no tiene incidencias. Añade alguna o inícialo igualmente.';
    case 'sprint.closed':
      return 'El sprint ya está cerrado y no admite cambios.';
    case 'sprint.not_planned':
      return 'Solo se pueden eliminar sprints planificados.';
    case 'project.archived':
      return 'El proyecto está archivado y es de solo lectura.';
    default:
      if (error.status === 403) return 'Solo quien gestiona los sprints puede hacer este cambio.';
      if (error.status === 422 && error.details[0]?.message) return error.details[0].message;
      return 'No se pudo completar la acción. Intenta de nuevo.';
  }
}

/** Sprints of a project, the sprint report and the sprint lifecycle actions. */
@Injectable({ providedIn: 'root' })
export class SprintPlanStore {
  private readonly api = inject(SprintV2ApiService);

  readonly sprints = new AsyncState<AgileSprint[]>();
  readonly report = new AsyncState<SprintReport>();

  constructor() {
    resetOnLogout(() => {
      this.sprints.reset();
      this.report.reset();
    });
  }

  loadSprints(projectId: string): void {
    this.sprints.load(this.api.list(projectId));
  }

  loadReport(sprintId: string): void {
    this.report.load(this.api.report(sprintId));
  }

  create(projectId: string, input: CreateAgileSprintInput): Observable<AgileSprint> {
    return this.api.create(projectId, input).pipe(catchError(rethrow));
  }

  update(sprintId: string, input: UpdateAgileSprintInput): Observable<AgileSprint> {
    return this.api.update(sprintId, input).pipe(catchError(rethrow));
  }

  start(sprintId: string, input: StartSprintInput): Observable<AgileSprint> {
    return this.api.start(sprintId, input).pipe(catchError(rethrow));
  }

  complete(sprintId: string, input: CompleteSprintInput): Observable<SprintCompletion> {
    return this.api.complete(sprintId, input).pipe(catchError(rethrow));
  }

  delete(sprintId: string): Observable<void> {
    return this.api.delete(sprintId).pipe(catchError(rethrow));
  }
}

function rethrow(error: unknown): Observable<never> {
  return throwError(() => asApiError(error));
}
