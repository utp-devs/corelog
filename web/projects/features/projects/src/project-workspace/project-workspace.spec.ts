import { Component } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import {
  ApiError,
  CurrentProjectStore,
  OrganizationMember,
  Project,
  projectRouteResolver,
} from '@corelog/core';
import { guardConsole } from '@corelog/core/testing';
import { NEVER, Observable, of, throwError } from 'rxjs';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import { ORG_ID, projectOf, projectTestEnvironment } from '../testing/projects-test-kit';
import { ProjectWorkspace } from './project-workspace';

@Component({ template: '<p>Tablero</p>' })
class BoardStub {}

const ROUTES = [
  { path: 'elsewhere', component: BoardStub },
  {
    path: 'projects/:key',
    resolve: { project: projectRouteResolver },
    component: ProjectWorkspace,
    children: [{ path: 'board', component: BoardStub }],
  },
];

function apiError(status: number, message: string): ApiError {
  return { status, code: `http.${status}`, message, details: [] };
}

function demoProject(overrides: Partial<Project> = {}): Project {
  return projectOf({
    id: 'p1',
    key: 'DEM',
    name: 'Proyecto demo',
    description: '',
    type: 'scrum',
    visibility: 'private',
    ...overrides,
  });
}

function orgMember(userId: string, name: string): OrganizationMember {
  return { userId, name } as unknown as OrganizationMember;
}

describe('ProjectWorkspace', () => {
  const consoleGuard = guardConsole();

  beforeEach(() => {
    window.localStorage.clear();
  });

  afterEach(() => {
    TestBed.resetTestingModule();
  });

  /** Opens the workspace at the board section with the given fakes. */
  async function open(options: Parameters<typeof projectTestEnvironment>[0] = {}) {
    const env = projectTestEnvironment(options);
    TestBed.configureTestingModule({
      providers: [provideRouter(ROUTES), ...env.providers],
    });
    const harness = await RouterTestingHarness.create('/projects/p1/board');
    await settle(harness.fixture);
    return { harness, fixture: harness.fixture, env };
  }

  async function settle(fixture: ComponentFixture<unknown>): Promise<void> {
    for (let pass = 0; pass < 3; pass += 1) {
      fixture.detectChanges();
      await Promise.resolve();
    }
  }

  function root(fixture: ComponentFixture<unknown>): HTMLElement {
    return fixture.nativeElement as HTMLElement;
  }

  it('shows the project identity, and no duplicated section tabs or breadcrumb', async () => {
    const { fixture } = await open({ projectApi: { get: () => of(demoProject()) } });

    expect(root(fixture).querySelector('h2')?.textContent).toContain('Proyecto demo');
    expect(root(fixture).querySelector('nav[aria-label="Secciones del proyecto"]')).toBeNull();
    expect(root(fixture).querySelector('nav[aria-label="Ruta de navegación"]')).toBeNull();
    consoleGuard.expectNoConsoleOutput();
  });

  it('draws a cover photograph behind the project header', async () => {
    const { fixture } = await open({ projectApi: { get: () => of(demoProject()) } });

    const cover = root(fixture).querySelector('.project-workspace__cover');
    expect(cover?.getAttribute('src')).toContain('/images/covers/cover-0');
    expect(cover?.getAttribute('alt')).toBe('');
  });

  it('collapses a long description to one line and expands it on demand', async () => {
    const { fixture } = await open({
      projectApi: { get: () => of(demoProject({ description: 'x'.repeat(200) })) },
    });

    const description = root(fixture).querySelector('.project-workspace__description');
    expect(description?.classList.contains('project-workspace__description--open')).toBe(false);
    const toggle = root(fixture).querySelector<HTMLButtonElement>('.project-workspace__toggle');
    expect(toggle?.textContent).toContain('Ver más');
    toggle?.click();
    fixture.detectChanges();
    expect(description?.classList.contains('project-workspace__description--open')).toBe(true);
    expect(toggle?.getAttribute('aria-expanded')).toBe('true');
  });

  it('shows the 403 page instead of the project when the project is forbidden', async () => {
    const { fixture } = await open({
      projectApi: { get: () => throwError(() => apiError(403, 'forbidden')) },
    });

    expect(root(fixture).textContent).toContain('No tienes acceso a esta página');
    expect(root(fixture).querySelector('nav[aria-label="Secciones del proyecto"]')).toBeNull();
    consoleGuard.expectNoConsoleOutput();
  });

  it('shows an error with retry when the project cannot be loaded for another reason', async () => {
    const { fixture, env } = await open({
      projectApi: { get: () => throwError(() => apiError(500, 'boom')) },
    });

    expect(root(fixture).querySelector('[role="alert"]')?.textContent).toContain(
      'No se pudo abrir el proyecto',
    );
    env.projectApi['get']?.mockReturnValue(of(demoProject()));
    const retry = Array.from(root(fixture).querySelectorAll('button')).find(
      (button) => button.textContent?.trim() === 'Reintentar',
    );
    retry?.click();
    await settle(fixture);

    expect(root(fixture).querySelector('h2')?.textContent).toContain('Proyecto demo');
    expect(env.projectApi['get']).toHaveBeenCalledTimes(2);
  });

  it('records the project in the recent items once its header is loaded', async () => {
    const { fixture } = await open({ projectApi: { get: () => of(demoProject()) } });

    expect(root(fixture).querySelector('h2')?.textContent).toContain('Proyecto demo');
    const recent = JSON.parse(window.localStorage.getItem('corelog.recent') ?? '[]') as {
      kind: string;
      id: string;
      subtitle: string;
    }[];
    expect(recent[0]).toMatchObject({ kind: 'project', id: 'p1', subtitle: 'DEM' });
  });

  it('shows the lead name resolved through the organization members', async () => {
    const { fixture, env } = await open({
      projectApi: { get: () => of(demoProject({ organizationId: ORG_ID, leadId: 'u1' })) },
      orgApi: { listMembers: () => of([orgMember('u1', 'Ana Pérez')]) },
    });

    expect(root(fixture).querySelector('[data-testid="workspace-lead"]')?.textContent).toContain(
      'Ana Pérez',
    );
    expect(env.orgApi['listMembers']).toHaveBeenCalledWith(ORG_ID);
    consoleGuard.expectNoConsoleOutput();
  });

  it('shows "Sin responsable" when the project has no lead', async () => {
    const { fixture } = await open({
      projectApi: { get: () => of(demoProject({ organizationId: ORG_ID, leadId: '' })) },
      orgApi: { listMembers: () => of([]) },
    });

    expect(root(fixture).querySelector('[data-testid="workspace-lead"]')?.textContent).toContain(
      'Sin responsable',
    );
    consoleGuard.expectNoConsoleOutput();
  });

  it('shows the loading state without tabs while the project request is pending', async () => {
    const { fixture } = await open({ projectApi: { get: () => NEVER as Observable<Project> } });

    expect(root(fixture).querySelector('[aria-busy="true"]')).not.toBeNull();
    expect(root(fixture).querySelector('nav[aria-label="Secciones del proyecto"]')).toBeNull();
  });

  it('records the recent project with its key path', async () => {
    await open({ projectApi: { get: () => of(demoProject()) } });

    const recent = JSON.parse(window.localStorage.getItem('corelog.recent') ?? '[]') as {
      link: string;
    }[];
    expect(recent[0].link).toBe('/projects/DEM');
  });

  it('clears the current project when the area is left', async () => {
    const { harness } = await open({ projectApi: { get: () => of(demoProject()) } });
    const current = TestBed.inject(CurrentProjectStore);
    expect(current.project()?.key).toBe('DEM');

    await harness.navigateByUrl('/elsewhere');

    expect(current.project()).toBeUndefined();
  });
});
