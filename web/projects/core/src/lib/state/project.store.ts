import { Injectable, computed, inject, signal } from '@angular/core';
import { Observable, catchError, map, tap, throwError } from 'rxjs';

import {
  ApiError,
  CreateProjectInput,
  Project,
  ProjectComponent,
  ProjectLabel,
  ProjectMember,
  ProjectMemberRole,
  UpdateProjectInput,
} from '../models';
import { ProjectApiService } from '../services/api/project-api.service';
import { ORGANIZATION_ID_PROVIDER } from '../utils/org-id-provider';
import { storeError } from '../utils/store-error';
import { AsyncState } from './async-state';
import { resetOnLogout } from './session-scoped';

export type ProjectStoreStatus = 'idle' | 'loading' | 'error';

function sortByName(projects: Project[]): Project[] {
  return [...projects].sort((a, b) => a.name.localeCompare(b.name));
}

/**
 * Projects of the active organization with the members, labels and components of the project in
 * view. Reads are loaded with `fetch*` and expose their state through signals. Mutations return cold
 * observables: they update the state on success and fail with the normalized ApiError (code kept).
 */
@Injectable({ providedIn: 'root' })
export class ProjectStore {
  private readonly api = inject(ProjectApiService);
  private readonly organizationIdOf = inject(ORGANIZATION_ID_PROVIDER);

  private readonly projectsRead = new AsyncState<Project[]>();
  private readonly membersRead = new AsyncState<ProjectMember[]>();
  private readonly labelsRead = new AsyncState<ProjectLabel[]>();
  private readonly componentsRead = new AsyncState<ProjectComponent[]>();

  /** Last failure of a project list or member operation. Null after a successful call. */
  readonly error = signal<ApiError | null>(null);

  readonly projects = computed(() => this.projectsRead.value() ?? []);
  readonly members = computed(() => this.membersRead.value() ?? []);
  readonly labels = computed(() => this.labelsRead.value() ?? []);
  readonly components = computed(() => this.componentsRead.value() ?? []);

  readonly status = computed<ProjectStoreStatus>(() => {
    if (this.projectsRead.loading() || this.membersRead.loading()) return 'loading';
    return this.error() ? 'error' : 'idle';
  });

  constructor() {
    resetOnLogout(() => this.reset());
  }

  fetchProjects(): void {
    this.error.set(null);
    this.projectsRead.load(
      this.api
        .list()
        .pipe(
          catchError((error: unknown) => this.fail(error, 'No se pudieron cargar los proyectos')),
        ),
    );
  }

  createProject(input: Omit<CreateProjectInput, 'organizationId'>): Observable<Project> {
    const organizationId = this.organizationIdOf();
    if (!organizationId) {
      const error: ApiError = {
        status: 422,
        code: 'project.organization_required',
        message: 'Selecciona una organización antes de crear un proyecto.',
        details: [],
      };
      this.error.set(error);
      return throwError(() => error);
    }
    return this.api.create({ ...input, organizationId }).pipe(
      tap((project) => {
        this.projectsRead.update((items) => sortByName([...(items ?? []), project]));
        this.error.set(null);
      }),
      catchError((error: unknown) => this.fail(error, 'No se pudo crear el proyecto')),
    );
  }

  updateProject(id: string, input: UpdateProjectInput): Observable<Project> {
    return this.api.update(id, input).pipe(
      tap((project) => {
        this.projectsRead.update((items) =>
          items?.map((item) => (item.id === id ? project : item)),
        );
        this.error.set(null);
      }),
      catchError((error: unknown) => this.fail(error, 'No se pudo actualizar el proyecto')),
    );
  }

  fetchMembers(projectId: string): void {
    this.error.set(null);
    this.membersRead.load(
      this.api
        .listMembers(projectId)
        .pipe(
          catchError((error: unknown) => this.fail(error, 'No se pudieron cargar los miembros')),
        ),
    );
  }

  /** Codes such as project.user_not_in_org and project.user_not_found reach the caller through error.code. */
  addMember(projectId: string, userId: string, role: ProjectMemberRole): Observable<void> {
    return this.api.addMember(projectId, userId, role).pipe(
      tap((member) => {
        this.membersRead.update((items) => [
          ...(items ?? []).filter((item) => item.userId !== member.userId),
          member,
        ]);
        this.error.set(null);
      }),
      map(() => undefined),
      catchError((error: unknown) => this.fail(error, 'No se pudo agregar el miembro')),
    );
  }

  updateMember(projectId: string, userId: string, role: ProjectMemberRole): Observable<void> {
    return this.api.updateMember(projectId, userId, role).pipe(
      tap((member) => {
        this.membersRead.update((items) =>
          items?.map((item) => (item.userId === userId ? member : item)),
        );
      }),
      map(() => undefined),
    );
  }

  removeMember(projectId: string, userId: string): Observable<void> {
    return this.api.removeMember(projectId, userId).pipe(
      tap(() => {
        this.membersRead.update((items) => items?.filter((item) => item.userId !== userId));
      }),
      map(() => undefined),
    );
  }

  fetchLabels(projectId: string): void {
    this.labelsRead.load(this.api.listLabels(projectId));
  }

  createLabel(projectId: string, name: string, color: string): Observable<void> {
    return this.api.createLabel(projectId, name, color).pipe(
      tap((label) => {
        this.labelsRead.update((items) => [
          ...(items ?? []).filter((item) => item.id !== label.id),
          label,
        ]);
      }),
      map(() => undefined),
    );
  }

  deleteLabel(projectId: string, labelId: string): Observable<void> {
    return this.api.deleteLabel(projectId, labelId).pipe(
      tap(() => {
        this.labelsRead.update((items) => items?.filter((item) => item.id !== labelId));
      }),
      map(() => undefined),
    );
  }

  fetchComponents(projectId: string): void {
    this.componentsRead.load(this.api.listComponents(projectId));
  }

  createComponent(projectId: string, name: string, description: string): Observable<void> {
    return this.api.createComponent(projectId, name, description).pipe(
      tap((component) => {
        this.componentsRead.update((items) => [
          ...(items ?? []).filter((item) => item.id !== component.id),
          component,
        ]);
      }),
      map(() => undefined),
    );
  }

  deleteComponent(projectId: string, componentId: string): Observable<void> {
    return this.api.deleteComponent(projectId, componentId).pipe(
      tap(() => {
        this.componentsRead.update((items) => items?.filter((item) => item.id !== componentId));
      }),
      map(() => undefined),
    );
  }

  reset(): void {
    this.projectsRead.reset();
    this.membersRead.reset();
    this.labelsRead.reset();
    this.componentsRead.reset();
    this.error.set(null);
  }

  private fail(error: unknown, fallbackMessage: string): Observable<never> {
    const apiError = storeError(error, fallbackMessage);
    this.error.set(apiError);
    return throwError(() => apiError);
  }
}
