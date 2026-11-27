import { TestBed } from '@angular/core/testing';
import { of, throwError } from 'rxjs';
import { afterEach, describe, expect, it } from 'vitest';

import { buttonNamed, mountSettingsPage } from '../settings-spec-helpers';
import { SAMPLE_GRANTS, schemeOf, settingsEnvironment } from '../settings-test-kit';
import { ProjectPermissionsMatrixPage } from './project-permissions-matrix-page';

function box(permission: string, holder: string): HTMLInputElement {
  return document.querySelector<HTMLInputElement>(`[data-testid="perm-${permission}-${holder}"]`)!;
}

function flip(permission: string, holder: string): void {
  const input = box(permission, holder);
  input.checked = !input.checked;
  input.dispatchEvent(new Event('change'));
}

function confirmDialog(): void {
  document.querySelector<HTMLButtonElement>('[data-testid="permissions-confirm"] button')?.click();
}

describe('ProjectPermissionsMatrixPage', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('renders the scheme as a matrix with the default indicator', async () => {
    await mountSettingsPage(ProjectPermissionsMatrixPage, settingsEnvironment());
    expect(box('create_issue', 'lead').checked).toBe(true);
    expect(box('create_issue', 'viewer').checked).toBe(false);
    expect(box('edit_own_issue', 'reporter').checked).toBe(true);
    expect(
      document.querySelector('[data-testid="permissions-default-state"]')?.textContent,
    ).toContain('Permisos estándar');
    expect(buttonNamed('Guardar cambios')?.disabled).toBe(true);
  });

  it('locks what the lead must keep', async () => {
    await mountSettingsPage(ProjectPermissionsMatrixPage, settingsEnvironment());
    expect(box('manage_members', 'lead').disabled).toBe(true);
    expect(box('edit_project', 'lead').disabled).toBe(true);
    expect(box('create_issue', 'lead').disabled).toBe(false);
  });

  it('previews the differences before saving and sends the full scheme', async () => {
    const environment = settingsEnvironment();
    const fixture = await mountSettingsPage(ProjectPermissionsMatrixPage, environment);
    flip('create_issue', 'viewer');
    fixture.detectChanges();
    expect(fixture.componentInstance.dirty()).toBe(true);

    buttonNamed('Guardar cambios')?.click();
    fixture.detectChanges();
    const diff = document.querySelector('[data-testid="permissions-diff"]')?.textContent ?? '';
    expect(diff).toContain('Crear incidencias');
    expect(diff).toContain('Ahora pueden: Observador');
    expect(environment.permissionsApi['putScheme']).not.toHaveBeenCalled();

    confirmDialog();
    const [id, grants] = environment.permissionsApi['putScheme'].mock.calls[0] as [
      string,
      { permission: string; holders: string[] }[],
    ];
    expect(id).toBe('p-1');
    expect(grants).toHaveLength(Object.keys(SAMPLE_GRANTS).length);
    expect(grants.find((grant) => grant.permission === 'create_issue')?.holders).toEqual([
      'lead',
      'contributor',
      'viewer',
    ]);
  });

  it('blocks a scheme nobody can delete the project with', async () => {
    const fixture = await mountSettingsPage(ProjectPermissionsMatrixPage, settingsEnvironment());
    flip('delete_project', 'lead');
    flip('delete_project', 'org_admin');
    fixture.detectChanges();
    expect(document.querySelector('[data-testid="permissions-problems"]')?.textContent).toContain(
      'Alguien debe poder eliminar el proyecto',
    );
    expect(buttonNamed('Guardar cambios')?.disabled).toBe(true);
  });

  it('shows the server rule error', async () => {
    const environment = settingsEnvironment({
      permissionsApi: {
        putScheme: () =>
          throwError(() => ({
            status: 422,
            code: 'authorization.invalid_scheme',
            message: 'el lead debe conservar manage_members',
            details: [],
          })),
      },
    });
    const fixture = await mountSettingsPage(ProjectPermissionsMatrixPage, environment);
    flip('create_issue', 'viewer');
    fixture.detectChanges();
    buttonNamed('Guardar cambios')?.click();
    fixture.detectChanges();
    confirmDialog();
    fixture.detectChanges();
    expect(document.querySelector('[role="alert"]')?.textContent).toContain(
      'Los permisos no son válidos: el lead debe conservar manage_members',
    );
  });

  it('restores the default scheme after a preview', async () => {
    const custom = schemeOf({ ...SAMPLE_GRANTS, create_issue: ['lead'] }, false);
    const environment = settingsEnvironment({
      scheme: custom,
      permissionsApi: { resetScheme: () => of(schemeOf(SAMPLE_GRANTS)) },
    });
    const fixture = await mountSettingsPage(ProjectPermissionsMatrixPage, environment);
    expect(document.body.textContent).toContain('Personalizados');
    buttonNamed('Restaurar estándar')?.click();
    fixture.detectChanges();
    expect(document.querySelector('[data-testid="permissions-diff"]')?.textContent).toContain(
      'Ahora pueden: Colaborador',
    );
    confirmDialog();
    expect(environment.permissionsApi['resetScheme']).toHaveBeenCalledWith('p-1');
  });

  it('is read-only for people who cannot edit', async () => {
    await mountSettingsPage(
      ProjectPermissionsMatrixPage,
      settingsEnvironment({ role: 'contributor' }),
    );
    expect(box('create_issue', 'lead').disabled).toBe(true);
    expect(buttonNamed('Guardar cambios')).toBeUndefined();
    expect(buttonNamed('Restaurar estándar')).toBeUndefined();
  });

  it('moves between boxes with the arrow keys', async () => {
    await mountSettingsPage(ProjectPermissionsMatrixPage, settingsEnvironment());
    const first = box('browse_project', 'lead');
    first.focus();
    first.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }));
    expect(document.activeElement).toBe(box('browse_project', 'contributor'));
  });
});
