import { ComponentFixture, TestBed } from '@angular/core/testing';
import { Router, provideRouter } from '@angular/router';
import { TeamApiService } from '@corelog/core';
import { guardConsole, provideCorelogTesting } from '@corelog/core/testing';
import { of, throwError } from 'rxjs';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { projectOf, projectTestEnvironment } from '../testing/projects-test-kit';
import { ProjectWizard } from './project-wizard';

const KEY_DELAY = 400;

describe('ProjectWizard', () => {
  const consoleGuard = guardConsole();

  afterEach(() => {
    vi.useRealTimers();
    TestBed.resetTestingModule();
  });

  async function settle(fixture: ComponentFixture<unknown>): Promise<void> {
    for (let pass = 0; pass < 3; pass += 1) {
      fixture.detectChanges();
      await Promise.resolve();
    }
  }

  async function render(options: Parameters<typeof projectTestEnvironment>[0] = {}) {
    const env = projectTestEnvironment({
      organization: { role: 'member', policy: 'members' },
      role: 'user',
      ...options,
    });
    TestBed.configureTestingModule({
      providers: [
        provideCorelogTesting(),
        provideRouter([]),
        ...env.providers,
        { provide: TeamApiService, useValue: { list: () => of([]) } },
      ],
    });
    const navigate = vi.spyOn(TestBed.inject(Router), 'navigateByUrl').mockResolvedValue(true);
    const fixture = TestBed.createComponent(ProjectWizard);
    fixture.detectChanges();
    await settle(fixture);
    return { fixture, env, navigate, root: fixture.nativeElement as HTMLElement };
  }

  function byTestId(root: HTMLElement, id: string): HTMLElement | null {
    return root.querySelector<HTMLElement>(`[data-testid="${id}"]`);
  }

  function clickTestId(root: HTMLElement, id: string): void {
    const element = byTestId(root, id);
    (element?.matches('button') ? element : element?.querySelector('button'))?.click();
  }

  async function type(
    fixture: ComponentFixture<unknown>,
    selector: string,
    value: string,
  ): Promise<void> {
    const input = (fixture.nativeElement as HTMLElement).querySelector<HTMLInputElement>(selector)!;
    input.value = value;
    input.dispatchEvent(new Event('input'));
    await settle(fixture);
  }

  async function fillDetails(fixture: ComponentFixture<unknown>): Promise<void> {
    vi.useFakeTimers();
    await type(fixture, 'input[formcontrolname="name"]', 'Plataforma de Pagos');
    await vi.advanceTimersByTimeAsync(KEY_DELAY);
    vi.useRealTimers();
    await settle(fixture);
  }

  it('starts on the template step with Scrum preselected and four templates', async () => {
    const { root } = await render();
    expect(root.querySelectorAll('[data-testid^="template-"]').length).toBe(4);
    expect(
      byTestId(root, 'template-scrum')?.querySelector<HTMLInputElement>('input')?.checked,
    ).toBe(true);
    expect(root.textContent).toContain('Seguimiento de errores');
    expect(root.textContent).toContain('Nuevo');
  });

  it('suggests the key from the name and checks it', async () => {
    const { fixture, env, root } = await render();
    clickTestId(root, 'wizard-next');
    await settle(fixture);

    await fillDetails(fixture);

    expect(root.querySelector<HTMLInputElement>('input[formcontrolname="key"]')?.value).toBe('PDP');
    expect(env.projectApi['keyAvailable']).toHaveBeenCalledWith('PDP');
    expect(root.textContent).toContain('Clave disponible');
  });

  it('blocks the step when the key is taken', async () => {
    const { fixture, root } = await render({
      projectApi: { keyAvailable: () => of({ available: false, key: 'PDP' }) },
    });
    clickTestId(root, 'wizard-next');
    await settle(fixture);
    await fillDetails(fixture);

    expect(root.textContent).toContain('Ya existe un proyecto con esta clave');
    clickTestId(root, 'wizard-next');
    await settle(fixture);
    expect(byTestId(root, 'wizard-step-details')?.getAttribute('aria-current')).toBe('step');
  });

  it('marks an invalid key without asking the server', async () => {
    const { fixture, env, root } = await render();
    clickTestId(root, 'wizard-next');
    await settle(fixture);
    await type(fixture, 'input[formcontrolname="key"]', 'a');

    expect(root.textContent).toContain('Usa de 2 a 10 caracteres');
    expect(env.projectApi['keyAvailable']).not.toHaveBeenCalled();
  });

  it('creates the project from the review step and opens it', async () => {
    const create = vi.fn(() =>
      of(projectOf({ id: 'p-9', key: 'PDP', name: 'Plataforma de Pagos' })),
    );
    const { fixture, env, navigate, root } = await render({ projectApi: { create } });
    root
      .querySelector<HTMLInputElement>('[data-testid="template-bug_tracking"] input')
      ?.dispatchEvent(new Event('change'));
    await settle(fixture);
    clickTestId(root, 'wizard-next');
    await settle(fixture);
    await fillDetails(fixture);
    clickTestId(root, 'wizard-next');
    await settle(fixture);
    clickTestId(root, 'wizard-next');
    await settle(fixture);

    expect(byTestId(root, 'wizard-summary')?.textContent).toContain('Plataforma de Pagos');
    clickTestId(root, 'wizard-create');
    await settle(fixture);

    expect(env.projectApi['create']).toHaveBeenCalledWith(
      expect.objectContaining({
        key: 'PDP',
        name: 'Plataforma de Pagos',
        template: 'bug_tracking',
        type: 'kanban',
        visibility: 'open',
      }),
    );
    expect(navigate).toHaveBeenCalledWith('/projects/PDP');
  });

  it('maps a key conflict from the server back to the details step', async () => {
    const create = vi.fn(() =>
      throwError(() => ({
        status: 409,
        code: 'project.key_taken',
        message: 'La clave ya existe',
        details: [],
      })),
    );
    const { fixture, navigate, root } = await render({ projectApi: { create } });
    clickTestId(root, 'wizard-next');
    await settle(fixture);
    await fillDetails(fixture);
    clickTestId(root, 'wizard-next');
    await settle(fixture);
    clickTestId(root, 'wizard-next');
    await settle(fixture);
    clickTestId(root, 'wizard-create');
    await settle(fixture);

    expect(navigate).not.toHaveBeenCalled();
    expect(byTestId(root, 'wizard-step-details')?.getAttribute('aria-current')).toBe('step');
    expect(root.textContent).toContain('Ya existe un proyecto con esta clave');
  });

  it('shows a forbidden page to guests instead of the wizard', async () => {
    const { root } = await render({ organization: { role: 'guest', policy: 'members' } });
    expect(root.querySelector('app-error-page')).not.toBeNull();
    expect(byTestId(root, 'wizard-next')).toBeNull();
  });

  it('reports unsaved edits once the user moves on', async () => {
    const { fixture, root } = await render();
    expect(fixture.componentInstance.dirty()).toBe(false);
    clickTestId(root, 'wizard-next');
    await settle(fixture);
    expect(fixture.componentInstance.dirty()).toBe(true);
  });
});
