import { ComponentFixture, TestBed } from '@angular/core/testing';
import { NotificationService, OrganizationMember, Project } from '@corelog/core';
import { guardConsole, installViewport, provideCorelogTesting } from '@corelog/core/testing';
import { Observable, throwError, of, NEVER } from 'rxjs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ORG_ID, projectOf, projectTestEnvironment } from '../testing/projects-test-kit';
import { ProjectsPage } from './projects-page';

function orgMember(userId: string, name: string): OrganizationMember {
  return { userId, name } as unknown as OrganizationMember;
}

function apiError(status: number, message: string) {
  return { status, code: `http.${status}`, message, details: [] };
}

const projects: Project[] = [
  projectOf({ id: 'p-1', key: 'CORE', name: 'CoreLog', visibility: 'open' }),
  projectOf({ id: 'p-2', key: 'SEC', name: 'Seguridad', visibility: 'private', type: 'scrum' }),
  projectOf({
    id: 'p-3',
    key: 'OLD',
    name: 'Heredado',
    visibility: 'open',
    archivedAt: '2026-02-01T00:00:00Z',
  }),
];

describe('ProjectsPage', () => {
  const consoleGuard = guardConsole();

  afterEach(() => {
    TestBed.resetTestingModule();
  });

  /** Creates the page with the given fakes and waits for the first render. */
  async function render(options: Parameters<typeof projectTestEnvironment>[0] = {}): Promise<{
    fixture: ComponentFixture<ProjectsPage>;
    env: ReturnType<typeof projectTestEnvironment>;
  }> {
    const env = projectTestEnvironment(options);
    TestBed.configureTestingModule({ providers: [provideCorelogTesting(), ...env.providers] });
    const fixture = TestBed.createComponent(ProjectsPage);
    fixture.detectChanges();
    await settle(fixture);
    return { fixture, env };
  }

  async function settle(fixture: ComponentFixture<unknown>): Promise<void> {
    for (let pass = 0; pass < 3; pass += 1) {
      fixture.detectChanges();
      await Promise.resolve();
    }
  }

  function element(fixture: ComponentFixture<unknown>, selector: string): HTMLElement {
    const found = (fixture.nativeElement as HTMLElement).querySelector<HTMLElement>(selector);
    if (!found) throw new Error(`no element for ${selector}`);
    return found;
  }

  function buttonNamed(
    fixture: ComponentFixture<unknown>,
    name: string,
  ): HTMLButtonElement | undefined {
    return Array.from((fixture.nativeElement as HTMLElement).querySelectorAll('button')).find(
      (button) => button.textContent?.trim() === name,
    );
  }

  function textOf(fixture: ComponentFixture<unknown>): string {
    return (fixture.nativeElement as HTMLElement).textContent ?? '';
  }

  /** Opens a PrimeNG select and picks the option with the given label from the overlay. */
  async function pickOption(
    fixture: ComponentFixture<unknown>,
    combobox: HTMLElement,
    label: string,
  ): Promise<void> {
    combobox.click();
    fixture.detectChanges();
    await settle(fixture);
    const option = Array.from(document.querySelectorAll<HTMLElement>('[role="option"]')).find(
      (el) => el.textContent?.trim() === label,
    );
    if (!option) throw new Error(`option ${label} not open`);
    option.click();
    fixture.detectChanges();
    await settle(fixture);
  }

  beforeEach(() => {
    vi.restoreAllMocks();
    // PrimeNG overlays (the create dialog) read matchMedia, which jsdom does not provide.
    installViewport(1440);
  });

  it('shows the projects as cards with key, visibility and type, hiding archived ones by default', async () => {
    const { fixture, env } = await render({ projectApi: { list: () => of(projects) } });
    expect(env.projectApi['list']).toHaveBeenCalled();

    const grid = element(fixture, '[role="group"][aria-label="Proyectos"]');
    expect(grid.querySelectorAll('article')).toHaveLength(2);
    const core = element(fixture, '[data-testid="project-card-CORE"]');
    expect(core.querySelector('h3')?.textContent).toContain('CoreLog');
    expect(core.textContent).toContain('Abierto');
    expect(element(fixture, '[data-testid="project-card-SEC"]').textContent).toContain('Privado');
    expect(textOf(fixture)).not.toContain('Heredado');
    consoleGuard.expectNoConsoleOutput();
  });

  it('filters the cards by visibility', async () => {
    const { fixture } = await render({ projectApi: { list: () => of(projects) } });

    const privates = buttonNamed(fixture, 'Privados') ?? findLabel(fixture, 'Privados');
    privates.click();
    fixture.detectChanges();
    await settle(fixture);

    expect(textOf(fixture)).not.toContain('CoreLog');
    expect(textOf(fixture)).toContain('Seguridad');
  });

  it('shows the archived projects when the status filter asks for them', async () => {
    const { fixture } = await render({ projectApi: { list: () => of(projects) } });

    await pickOption(fixture, element(fixture, '[aria-label="Filtrar por estado"]'), 'Archivados');

    expect(textOf(fixture)).toContain('Heredado');
    expect(textOf(fixture)).not.toContain('CoreLog');
  });

  it('shows an empty state with a create action when there are no projects', async () => {
    const { fixture } = await render({ projectApi: { list: () => of([]) } });

    expect(textOf(fixture)).toContain('Aún no hay proyectos');
    expect(buttonNamed(fixture, 'Nuevo proyecto')).toBeDefined();
    consoleGuard.expectNoConsoleOutput();
  });

  it('disables project creation for non-admin users with an explanation', async () => {
    const { fixture } = await render({ role: 'user', projectApi: { list: () => of(projects) } });

    const create = buttonNamed(fixture, 'Nuevo proyecto');
    expect(create?.disabled).toBe(true);
    consoleGuard.expectNoConsoleOutput();
  });

  it('lets organization members create projects when the policy opens creation to members', async () => {
    const { fixture } = await render({
      role: 'user',
      organization: { role: 'member', policy: 'members' },
      projectApi: { list: () => of(projects) },
    });

    expect(buttonNamed(fixture, 'Nuevo proyecto')?.disabled).toBe(false);
  });

  it('keeps creation closed to members when only admins may create, and says why', async () => {
    const { fixture } = await render({
      role: 'user',
      organization: { role: 'member', policy: 'admins' },
      projectApi: { list: () => of(projects) },
    });

    expect(buttonNamed(fixture, 'Nuevo proyecto')?.disabled).toBe(true);
  });

  it('lets organization admins create projects even without the global admin role', async () => {
    const { fixture } = await render({
      role: 'user',
      organization: { role: 'admin', policy: 'admins' },
      projectApi: { list: () => of(projects) },
    });

    expect(buttonNamed(fixture, 'Nuevo proyecto')?.disabled).toBe(false);
  });

  it('opens the create dialog when the URL asks for it and clears the parameter', async () => {
    const { fixture, env } = await render({
      queryParams: { new: '1' },
      projectApi: { list: () => of(projects) },
    });

    expect(textOf(fixture)).toContain('Clave');
    expect(env.navigate).toHaveBeenCalledWith(
      [],
      expect.objectContaining({ queryParams: { new: null }, replaceUrl: true }),
    );
  });

  it('shows the 403 page when the API forbids listing projects', async () => {
    const { fixture } = await render({
      projectApi: { list: () => throwError(() => apiError(403, 'forbidden')) },
    });

    expect(textOf(fixture)).toContain('No tienes acceso a esta página');
    consoleGuard.expectNoConsoleOutput();
  });

  it('shows an error with retry when the list fails for another reason', async () => {
    const { fixture, env } = await render({
      projectApi: {
        list: () => throwError(() => apiError(500, 'No se pudieron cargar los proyectos.')),
      },
    });
    expect(element(fixture, '[role="alert"]').textContent).toContain(
      'No se pudieron cargar los proyectos.',
    );

    env.projectApi['list']?.mockReturnValue(of(projects));
    buttonNamed(fixture, 'Reintentar')?.click();
    await settle(fixture);

    expect(textOf(fixture)).toContain('CoreLog');
    expect(env.projectApi['list']).toHaveBeenCalledTimes(2);
  });

  it('shows the loading skeleton while the projects are requested', async () => {
    const { fixture } = await render({
      projectApi: { list: () => NEVER as Observable<Project[]> },
    });

    const loading = element(fixture, '[aria-label="Cargando proyectos"]');
    expect(loading.getAttribute('aria-busy')).toBe('true');
  });

  it('shows the lead name on each card, resolved through the organization members', async () => {
    const { fixture, env } = await render({
      projectApi: { list: () => of(projects) },
      orgApi: { listMembers: () => of([orgMember('u1', 'Ana Pérez')]) },
    });

    expect(element(fixture, '[data-testid="project-lead-CORE"]').textContent).toContain(
      'Ana Pérez',
    );
    expect(element(fixture, '[data-testid="project-card-SEC"]').textContent).toContain('Ana Pérez');
    expect(env.orgApi['listMembers']).toHaveBeenCalledWith(ORG_ID);
    consoleGuard.expectNoConsoleOutput();
  });

  it('shows "Sin responsable" when the lead is not a member of the organization', async () => {
    const { fixture } = await render({
      projectApi: { list: () => of([projectOf({ leadId: 'ghost' })]) },
    });

    expect(element(fixture, '[data-testid="project-lead-CORE"]').textContent).toContain(
      'Sin responsable',
    );
    consoleGuard.expectNoConsoleOutput();
  });

  it('shows the lead as loading until the organization members arrive', async () => {
    const { fixture } = await render({
      projectApi: { list: () => of([projectOf({})]) },
      orgApi: { listMembers: () => NEVER as Observable<OrganizationMember[]> },
    });

    expect(element(fixture, '[data-testid="project-lead-CORE"]').textContent).toContain(
      'Cargando responsable',
    );
  });

  it('renders the cards in the shared CardGrid at 390px and on desktop, one card per direct child', async () => {
    installViewport(390);
    const mobile = await render({ projectApi: { list: () => of(projects) } });
    const mobileCard = element(mobile.fixture, '[data-testid="project-card-CORE"]');
    expect(mobileCard.parentElement?.hasAttribute('data-card-grid')).toBe(true);
    TestBed.resetTestingModule();

    installViewport(1440);
    const desktop = await render({ projectApi: { list: () => of(projects) } });
    expect(
      element(desktop.fixture, '[data-testid="project-card-CORE"]').parentElement?.hasAttribute(
        'data-card-grid',
      ),
    ).toBe(true);
    consoleGuard.expectNoConsoleOutput();
  });

  it('creates a project from the dialog and shows the success message', async () => {
    const { fixture, env } = await render({
      projectApi: {
        list: () => of(projects),
        create: () => of(projectOf({ id: 'p-9', key: 'NEW', name: 'Nuevo' })),
      },
    });
    const notify = TestBed.inject(NotificationService);
    const success = vi.spyOn(notify, 'success');

    buttonNamed(fixture, 'Nuevo proyecto')?.click();
    fixture.detectChanges();
    await settle(fixture);
    setInput(fixture, 'input[formcontrolname="key"]', 'NEW');
    setInput(fixture, 'input[formcontrolname="name"]', 'Nuevo');
    buttonNamed(fixture, 'Crear')?.click();
    await settle(fixture);

    expect(env.projectApi['create']).toHaveBeenCalledWith(
      expect.objectContaining({
        organizationId: ORG_ID,
        key: 'NEW',
        name: 'Nuevo',
        type: 'kanban',
        visibility: 'open',
      }),
    );
    expect(success).toHaveBeenCalledWith('Proyecto creado');
  });

  it('keeps the create dialog open and shows the client messages when the form is incomplete', async () => {
    const { fixture, env } = await render({ projectApi: { list: () => of([]) } });

    buttonNamed(fixture, 'Nuevo proyecto')?.click();
    fixture.detectChanges();
    await settle(fixture);
    buttonNamed(fixture, 'Crear')?.click();
    await settle(fixture);

    expect(env.projectApi['create']).not.toHaveBeenCalled();
    expect(textOf(fixture)).toContain('Escribe una clave');
    expect(textOf(fixture)).toContain('Escribe un nombre');
  });
});

/** Finds a button by its visible label, for PrimeNG selectbutton options that are not buttons. */
function findLabel(fixture: ComponentFixture<unknown>, label: string): HTMLElement {
  const match = Array.from(
    (fixture.nativeElement as HTMLElement).querySelectorAll<HTMLElement>('*'),
  ).find((el) => el.children.length === 0 && el.textContent?.trim() === label);
  if (!match) throw new Error(`no control labelled ${label}`);
  return match;
}

/** Types into an input found by selector, firing the input event the way a user does. */
function setInput(fixture: ComponentFixture<unknown>, selector: string, value: string): void {
  const input = (fixture.nativeElement as HTMLElement).querySelector<HTMLInputElement>(selector);
  if (!input) throw new Error(`no input ${selector}`);
  input.value = value;
  input.dispatchEvent(new Event('input'));
  input.dispatchEvent(new Event('change'));
  fixture.detectChanges();
}
