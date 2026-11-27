import { Component } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideRouter, RouterOutlet } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { ApiError } from '@corelog/core';
import { guardConsole, installViewport } from '@corelog/core/testing';
import { NEVER, Observable, of, throwError } from 'rxjs';
import { afterEach, describe, expect, it } from 'vitest';

import { memberOf, projectTestEnvironment } from '../testing/projects-test-kit';
import { ProjectPermissionsPage } from './project-permissions-page';

@Component({ imports: [RouterOutlet], template: '<router-outlet />' })
class WorkspaceStub {}

const ROUTES = [
  {
    path: 'projects/:id',
    component: WorkspaceStub,
    children: [{ path: 'permisos', component: ProjectPermissionsPage }],
  },
];

function apiError(status: number, message: string): ApiError {
  return { status, code: `http.${status}`, message, details: [] };
}

describe('ProjectPermissionsPage', () => {
  const consoleGuard = guardConsole();
  let restoreViewport: (() => void) | undefined;

  afterEach(() => {
    restoreViewport?.();
    restoreViewport = undefined;
    TestBed.resetTestingModule();
  });

  async function open(
    options: Parameters<typeof projectTestEnvironment>[0] = {},
  ): Promise<ComponentFixture<unknown>> {
    // Regular user: the matrix marks their own role, not the administrator override.
    const env = projectTestEnvironment({ role: 'user', ...options });
    TestBed.configureTestingModule({ providers: [provideRouter(ROUTES), ...env.providers] });
    const harness = await RouterTestingHarness.create('/projects/p-1/permisos');
    await settle(harness.fixture);
    return harness.fixture as ComponentFixture<unknown>;
  }

  async function settle(fixture: ComponentFixture<unknown>): Promise<void> {
    for (let pass = 0; pass < 3; pass += 1) {
      fixture.detectChanges();
      await Promise.resolve();
    }
  }

  function text(fixture: ComponentFixture<unknown>): string {
    return (fixture.nativeElement as HTMLElement).textContent ?? '';
  }

  function heading(fixture: ComponentFixture<unknown>, name: string): HTMLElement | null {
    return (
      Array.from(
        (fixture.nativeElement as HTMLElement).querySelectorAll<HTMLElement>('h2,h3'),
      ).find((el) => el.textContent?.trim() === name) ?? null
    );
  }

  it('shows the role matrix in a table with the current role highlighted', async () => {
    const fixture = await open({
      projectApi: {
        listMembers: () => of([memberOf('u1', 'lead'), memberOf('u2', 'viewer')]),
      },
    });

    expect(text(fixture)).toContain('Ver el proyecto');
    expect(text(fixture)).toContain('Tú');
    expect(heading(fixture, 'Matriz de permisos')).not.toBeNull();
    expect(heading(fixture, 'Tu rol en este proyecto')).not.toBeNull();
    consoleGuard.expectNoConsoleOutput();
  });

  it('tells users who are not members that they need access', async () => {
    const fixture = await open({ projectApi: { listMembers: () => of([]) } });

    expect(text(fixture)).toContain('Sin acceso como miembro');
    expect(text(fixture)).toContain(
      'No apareces como miembro del proyecto. Pide acceso a un líder.',
    );
  });

  it('shows the 403 page when the project members are forbidden', async () => {
    const fixture = await open({
      projectApi: { listMembers: () => throwError(() => apiError(403, 'forbidden')) },
    });

    expect(text(fixture)).toContain('No tienes acceso a esta página');
    consoleGuard.expectNoConsoleOutput();
  });

  it('shows the skeleton for the role section while the members load', async () => {
    const fixture = await open({ projectApi: { listMembers: () => NEVER as Observable<never[]> } });

    expect(heading(fixture, 'Matriz de permisos')).not.toBeNull();
    expect(text(fixture)).not.toContain('Sin acceso como miembro');
    consoleGuard.expectNoConsoleOutput();
  });

  it('keeps the permission matrix and warns when the members fail for another reason', async () => {
    const fixture = await open({
      projectApi: { listMembers: () => throwError(() => apiError(500, 'boom')) },
    });

    expect(text(fixture)).toContain(
      'No se pudieron cargar los miembros. La matriz de permisos sigue vigente.',
    );
    expect(text(fixture)).toContain('Ver el proyecto');
    consoleGuard.expectNoConsoleOutput();
  });

  it('shows the matrix as one card per permission on phones', async () => {
    restoreViewport = installViewport(390);
    const fixture = await open({ projectApi: { listMembers: () => of([memberOf('u1', 'lead')]) } });

    expect(fixture.nativeElement.querySelector('table')).toBeNull();
    expect(
      fixture.nativeElement.querySelector('[aria-label="Matriz de permisos por rol"]'),
    ).not.toBeNull();
  });
});
