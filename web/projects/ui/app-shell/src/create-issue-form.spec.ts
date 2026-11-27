import { signal } from '@angular/core';
import { TestBed, type ComponentFixture } from '@angular/core/testing';
import { Router } from '@angular/router';
import {
  AuthStore,
  NotificationService,
  OrganizationApiService,
  OrganizationStore,
  ProjectStore,
  StorageService,
  IssueStore,
  UserDirectoryApiService,
  type IssueDraftFormGroup,
  type Project,
  type Issue,
} from '@corelog/core';
import { Observable, of, throwError } from 'rxjs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { CreateIssueForm, LAST_CREATE_PROJECT_KEY } from './create-issue-form';

function projectOf(
  id: string,
  name: string,
  key: string,
  archivedAt: string | null = null,
): Project {
  return {
    id,
    organizationId: 'org-1',
    key,
    name,
    description: '',
    type: 'kanban',
    leadId: 'u-1',
    visibility: 'open',
    archivedAt,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  };
}

const ISSUE = {
  id: '1234abcd-0000-4000-8000-000000000000',
  key: 'CORE-7',
  summary: 'Falla el login',
} as Issue;

interface Options {
  projects?: Project[];
  status?: 'idle' | 'loading' | 'error';
  url?: string;
  role?: 'owner' | 'admin' | 'member' | 'guest' | null;
  policy?: 'members' | 'admins';
  create?: (input: unknown) => Observable<Issue>;
  assign?: (id: string, assigneeId: string | null) => Observable<Issue>;
  stored?: string;
}

interface FormAccess {
  form: IssueDraftFormGroup;
}

function setup(options: Options = {}) {
  const projects = signal(options.projects ?? [projectOf('p-1', 'Core', 'CORE')]);
  const status = signal<'idle' | 'loading' | 'error'>(options.status ?? 'idle');
  const fetchProjects = vi.fn();
  const createIssue = vi.fn(options.create ?? (() => of(ISSUE)));
  const assign = vi.fn(options.assign ?? (() => of(ISSUE)));
  const router = { url: options.url ?? '/issues', navigateByUrl: vi.fn() };
  const notify = { success: vi.fn(), info: vi.fn(), error: vi.fn() };
  const storage = new Map<string, string>();
  if (options.stored) storage.set(LAST_CREATE_PROJECT_KEY, options.stored);

  TestBed.configureTestingModule({
    providers: [
      { provide: Router, useValue: router },
      { provide: ProjectStore, useValue: { projects, status, fetchProjects } },
      { provide: IssueStore, useValue: { createIssue, assign } },
      { provide: NotificationService, useValue: notify },
      { provide: AuthStore, useValue: { user: signal({ role: 'user' }) } },
      {
        provide: OrganizationStore,
        useValue: {
          currentRole: signal(options.role ?? 'member'),
          currentOrg: signal({ settings: { whoCanCreateProjects: options.policy ?? 'admins' } }),
        },
      },
      { provide: OrganizationApiService, useValue: { listMembers: () => of([]) } },
      { provide: UserDirectoryApiService, useValue: { searchTeammates: () => of([]) } },
      {
        provide: StorageService,
        useValue: {
          getItem: (key: string) => storage.get(key) ?? null,
          setItem: (key: string, value: string) => storage.set(key, value),
        },
      },
    ],
  });
  const fixture = TestBed.createComponent(CreateIssueForm);
  const closed = vi.fn();
  fixture.componentInstance.closed.subscribe(closed);
  fixture.detectChanges();
  return { fixture, createIssue, assign, router, notify, closed, fetchProjects, storage };
}

function el(fixture: ComponentFixture<unknown>): HTMLElement {
  return fixture.nativeElement as HTMLElement;
}

function buttonNamed(
  fixture: ComponentFixture<unknown>,
  name: string,
): HTMLButtonElement | undefined {
  return Array.from(el(fixture).querySelectorAll('button')).find(
    (button) => button.textContent?.trim() === name,
  );
}

function formOf(fixture: ComponentFixture<CreateIssueForm>): IssueDraftFormGroup {
  return (fixture.componentInstance as unknown as FormAccess).form;
}

function fill(fixture: ComponentFixture<CreateIssueForm>, summary = 'Falla el login'): void {
  formOf(fixture).patchValue({ summary, description: 'No se puede iniciar sesión.' });
  fixture.detectChanges();
}

describe('CreateIssueForm', () => {
  beforeEach(() => vi.restoreAllMocks());
  afterEach(() => TestBed.resetTestingModule());

  it('opens the project list and the form with the first project selected', () => {
    const { fixture, fetchProjects } = setup({
      projects: [projectOf('p-1', 'Core', 'CORE'), projectOf('p-2', 'Seguridad', 'SEC')],
    });
    expect(fetchProjects).toHaveBeenCalled();
    expect(el(fixture).textContent).toContain('Crear incidencia');
    expect(el(fixture).textContent).toContain('Resumen');
    expect(el(fixture).textContent).toContain('Descripción');
    expect(formOf(fixture).controls.projectId.value).toBe('p-1');
  });

  it('preselects the project in view, then the last one used', () => {
    const projects = [projectOf('p-1', 'Core', 'CORE'), projectOf('p-2', 'Seguridad', 'SEC')];
    const inView = setup({ projects, url: '/projects/p-2/board', stored: 'p-1' });
    expect(formOf(inView.fixture).controls.projectId.value).toBe('p-2');
    TestBed.resetTestingModule();

    const lastUsed = setup({ projects, stored: 'p-2' });
    expect(formOf(lastUsed.fixture).controls.projectId.value).toBe('p-2');
  });

  it('never offers archived projects', () => {
    const { fixture } = setup({
      projects: [
        projectOf('p-1', 'Viejo', 'OLD', '2026-02-01T00:00:00Z'),
        projectOf('p-2', 'Core', 'CORE'),
      ],
    });
    expect(formOf(fixture).controls.projectId.value).toBe('p-2');
  });

  it('does not create and shows messages when the form is incomplete', () => {
    const { fixture, createIssue } = setup();
    buttonNamed(fixture, 'Crear incidencia')?.click();
    fixture.detectChanges();

    expect(createIssue).not.toHaveBeenCalled();
    expect(el(fixture).textContent).toContain('Escribe un resumen.');
    expect(el(fixture).textContent).toContain('Describe la incidencia.');
  });

  it('creates the issue in the chosen project through the issue store', () => {
    const { fixture, createIssue, notify, storage } = setup();
    fill(fixture);
    formOf(fixture).patchValue({ issueType: 'bug', priority: 'high' });
    buttonNamed(fixture, 'Crear incidencia')?.click();
    fixture.detectChanges();

    expect(createIssue).toHaveBeenCalledWith({
      projectId: 'p-1',
      summary: 'Falla el login',
      description: 'No se puede iniciar sesión.',
      priority: 'high',
      issueType: 'bug',
    });
    expect(notify.success).toHaveBeenCalled();
    expect(storage.get(LAST_CREATE_PROJECT_KEY)).toBe('p-1');
    expect(buttonNamed(fixture, 'Ver incidencia')).toBeDefined();
    expect(buttonNamed(fixture, 'Crear otra')).toBeDefined();
    expect(el(fixture).textContent).toContain('Incidencia creada');
  });

  it('assigns the issue when an assignee is chosen', () => {
    const { fixture, assign } = setup();
    fill(fixture);
    formOf(fixture).controls.assigneeId.setValue('u-9');
    buttonNamed(fixture, 'Crear incidencia')?.click();

    expect(assign).toHaveBeenCalledWith(ISSUE.id, 'u-9');
  });

  it('keeps the created issue and warns when the assignment fails', () => {
    const { fixture, notify } = setup({ assign: () => throwError(() => new Error('nope')) });
    fill(fixture);
    formOf(fixture).controls.assigneeId.setValue('u-9');
    buttonNamed(fixture, 'Crear incidencia')?.click();
    fixture.detectChanges();

    expect(notify.info).toHaveBeenCalledWith('La incidencia se creó, pero no se pudo asignar.');
    expect(buttonNamed(fixture, 'Ver incidencia')).toBeDefined();
  });

  it('opens the created issue and closes the dialog', () => {
    const { fixture, router, closed } = setup();
    fill(fixture);
    buttonNamed(fixture, 'Crear incidencia')?.click();
    fixture.detectChanges();
    buttonNamed(fixture, 'Ver incidencia')?.click();

    expect(closed).toHaveBeenCalled();
    expect(router.navigateByUrl).toHaveBeenCalledWith(`/browse/${ISSUE.key}`);
  });

  it('offers to create another one with the project and type kept', () => {
    const { fixture } = setup();
    fill(fixture);
    formOf(fixture).patchValue({ issueType: 'story' });
    buttonNamed(fixture, 'Crear incidencia')?.click();
    fixture.detectChanges();
    buttonNamed(fixture, 'Crear otra')?.click();
    fixture.detectChanges();

    const form = formOf(fixture);
    expect(form.controls.summary.value).toBe('');
    expect(form.controls.description.value).toBe('');
    expect(form.controls.projectId.value).toBe('p-1');
    expect(form.controls.issueType.value).toBe('story');
    expect(buttonNamed(fixture, 'Crear incidencia')).toBeDefined();
  });

  it('shows a translated error inside the dialog when the creation fails', () => {
    const forbidden = { status: 403, code: 'http.403', message: 'forbidden', details: [] };
    const { fixture, notify } = setup({ create: () => throwError(() => forbidden) });
    fill(fixture);
    buttonNamed(fixture, 'Crear incidencia')?.click();
    fixture.detectChanges();

    expect(el(fixture).querySelector('[role="alert"]')?.textContent).toContain('No tienes permiso');
    expect(notify.success).not.toHaveBeenCalled();
    expect(buttonNamed(fixture, 'Ver incidencia')).toBeUndefined();
  });

  it('asks before discarding typed text', () => {
    const { fixture, closed } = setup();
    formOf(fixture).controls.summary.markAsDirty();
    el(fixture)
      .querySelector('[role="dialog"]')
      ?.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    fixture.detectChanges();

    expect(closed).not.toHaveBeenCalled();
    expect(document.body.textContent ?? '').toContain('Tienes cambios sin guardar');
  });

  it('closes at once when nothing was typed', () => {
    const { fixture, closed } = setup();
    buttonNamed(fixture, 'Cancelar')?.click();
    expect(closed).toHaveBeenCalled();
  });

  describe('without projects', () => {
    it('explains it to a member who may not create projects', () => {
      const { fixture } = setup({ projects: [], role: 'member', policy: 'admins' });
      expect(el(fixture).textContent).toContain('Aún no hay proyectos');
      expect(el(fixture).textContent).toContain('Pide a quien administra');
      expect(buttonNamed(fixture, 'Crear un proyecto')).toBeUndefined();
    });

    it('offers to create a project when the person is allowed to', () => {
      const { fixture, router, closed } = setup({ projects: [], role: 'admin' });
      buttonNamed(fixture, 'Crear un proyecto')?.click();
      expect(closed).toHaveBeenCalled();
      expect(router.navigateByUrl).toHaveBeenCalledWith('/projects');
    });

    it('lets members create projects when the organization policy allows it', () => {
      const { fixture } = setup({ projects: [], role: 'member', policy: 'members' });
      expect(buttonNamed(fixture, 'Crear un proyecto')).toBeDefined();
    });
  });

  it('shows a loading note while the projects load', () => {
    const { fixture } = setup({ projects: [], status: 'loading' });
    expect(el(fixture).textContent).toContain('Cargando proyectos');
  });

  it('offers to retry when the projects fail to load', () => {
    const { fixture, fetchProjects } = setup({ projects: [], status: 'error' });
    fetchProjects.mockClear();
    buttonNamed(fixture, 'Reintentar')?.click();
    expect(fetchProjects).toHaveBeenCalledTimes(1);
  });
});
