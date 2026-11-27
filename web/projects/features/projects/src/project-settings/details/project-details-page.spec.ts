import { ComponentFixture, TestBed } from '@angular/core/testing';
import { NotificationService } from '@corelog/core';
import { of, throwError } from 'rxjs';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { buttonNamed, mountSettingsPage } from '../settings-spec-helpers';
import { memberOf, projectOf, settingsEnvironment } from '../settings-test-kit';
import { ProjectDetailsPage } from './project-details-page';

function field(testId: string): HTMLInputElement {
  return document.querySelector<HTMLInputElement>(`[data-testid="${testId}"]`)!;
}

function type(fixture: ComponentFixture<ProjectDetailsPage>, testId: string, value: string): void {
  const input = field(testId);
  input.value = value;
  input.dispatchEvent(new Event('input'));
  fixture.detectChanges();
}

describe('ProjectDetailsPage', () => {
  afterEach(() => {
    TestBed.resetTestingModule();
    vi.restoreAllMocks();
  });

  it('shows the project, keeps save disabled until something changes and tracks dirty', async () => {
    const fixture = await mountSettingsPage(ProjectDetailsPage, settingsEnvironment());
    expect(field('details-name').value).toBe('CoreLog');
    expect(field('details-key').value).toBe('CORE');
    expect(fixture.componentInstance.dirty()).toBe(false);
    expect(buttonNamed('Guardar cambios')?.disabled).toBe(true);

    type(fixture, 'details-name', 'CoreLog 2');
    expect(fixture.componentInstance.dirty()).toBe(true);
    expect(buttonNamed('Guardar cambios')?.disabled).toBe(false);
    expect(document.body.textContent).toContain('Tienes cambios sin guardar');
  });

  it('saves the edited details and refreshes the current project', async () => {
    const updated = projectOf({ name: 'CoreLog 2' });
    const environment = settingsEnvironment({ projectApi: { update: () => of(updated) } });
    const fixture = await mountSettingsPage(ProjectDetailsPage, environment);
    const notify = vi.spyOn(TestBed.inject(NotificationService), 'success');
    type(fixture, 'details-name', 'CoreLog 2');
    buttonNamed('Guardar cambios')?.click();
    fixture.detectChanges();

    const [id, input] = environment.projectApi['update'].mock.calls[0] as [
      string,
      Record<string, unknown>,
    ];
    expect(id).toBe('p-1');
    expect(input).toMatchObject({
      name: 'CoreLog 2',
      leadId: 'u-me',
      visibility: 'open',
      type: 'kanban',
    });
    expect(environment.current.setProject).toHaveBeenCalledWith(updated);
    expect(environment.current.reloadPermissions).toHaveBeenCalled();
    expect(notify).toHaveBeenCalledWith('Detalles del proyecto guardados');
  });

  it('requires a name', async () => {
    const environment = settingsEnvironment();
    const fixture = await mountSettingsPage(ProjectDetailsPage, environment);
    type(fixture, 'details-name', '');
    buttonNamed('Guardar cambios')?.click();
    fixture.detectChanges();
    expect(environment.projectApi['update']).not.toHaveBeenCalled();
    expect(document.body.textContent).toContain('Escribe el nombre del proyecto.');
  });

  it('shows the failure in Spanish when saving fails', async () => {
    const environment = settingsEnvironment({
      projectApi: {
        update: () =>
          throwError(() => ({ status: 409, code: 'project.archived', message: 'x', details: [] })),
      },
    });
    const fixture = await mountSettingsPage(ProjectDetailsPage, environment);
    type(fixture, 'details-name', 'Otro');
    buttonNamed('Guardar cambios')?.click();
    fixture.detectChanges();
    expect(document.body.textContent).toContain('El proyecto está archivado');
  });

  it('offers the members as lead candidates', async () => {
    const fixture = await mountSettingsPage(
      ProjectDetailsPage,
      settingsEnvironment({ members: [memberOf('u-me', 'lead'), memberOf('u-2', 'contributor')] }),
    );
    const options = (
      fixture.componentInstance as unknown as { leadOptions(): { label: string }[] }
    ).leadOptions();
    expect(options.map((option) => option.label)).toEqual(['Ana Pérez', 'Luis Gómez']);
  });

  it('is read-only for an archived project and for people who cannot edit', async () => {
    await mountSettingsPage(
      ProjectDetailsPage,
      settingsEnvironment({ project: projectOf({ archivedAt: '2026-02-01T00:00:00Z' }) }),
    );
    expect(field('details-name').disabled).toBe(true);
    expect(buttonNamed('Guardar cambios')).toBeUndefined();
    TestBed.resetTestingModule();

    await mountSettingsPage(ProjectDetailsPage, settingsEnvironment({ role: 'contributor' }));
    expect(field('details-name').disabled).toBe(true);
    expect(buttonNamed('Guardar cambios')).toBeUndefined();
  });
});
