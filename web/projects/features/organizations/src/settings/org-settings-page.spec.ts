import { ComponentFixture, TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { ApiError, LOCATION_ASSIGN, NotificationService } from '@corelog/core';
import { of, throwError } from 'rxjs';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { ORG_ID, memberOf, setupOrgContext } from '../testing/org-test-kit';
import { OrgDangerZone } from './org-danger-zone';
import { OrgSettingsPage } from './org-settings-page';

let notifySuccess: ReturnType<typeof vi.spyOn>;
let notifyError: ReturnType<typeof vi.spyOn>;

function buttonNamed(name: string): HTMLButtonElement | undefined {
  return Array.from(document.querySelectorAll('button')).find(
    (button) => button.textContent?.trim() === name,
  );
}

function fieldLabelled(label: RegExp): HTMLElement {
  const found = Array.from(document.querySelectorAll('label')).find((el) =>
    label.test(el.textContent ?? ''),
  );
  const target = found ? document.getElementById(found.htmlFor) : null;
  if (!target) throw new Error(`no field labelled ${label}`);
  return target;
}

function create(): ComponentFixture<OrgSettingsPage> {
  const notify = TestBed.inject(NotificationService);
  notifySuccess = vi.spyOn(notify, 'success');
  notifyError = vi.spyOn(notify, 'error');
  const fixture = TestBed.createComponent(OrgSettingsPage);
  fixture.detectChanges();
  return fixture;
}

function typeName(fixture: ComponentFixture<OrgSettingsPage>, value: string): void {
  const input = fieldLabelled(/Nombre/) as HTMLInputElement;
  input.value = value;
  input.dispatchEvent(new Event('input'));
  fixture.detectChanges();
}

describe('OrgSettingsPage', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('lets the owner edit, shows the danger zone and keeps save disabled until something changes', () => {
    setupOrgContext('owner');
    const fixture = create();
    expect(buttonNamed('Guardar cambios')?.disabled).toBe(true);
    expect(document.body.textContent).not.toContain('Solo lectura');
    expect(buttonNamed('Transferir propiedad')?.disabled).toBe(true);
    expect(buttonNamed('Eliminar organización')).toBeDefined();

    typeName(fixture, 'Acme Labs');
    expect(buttonNamed('Guardar cambios')?.disabled).toBe(false);
  });

  it('saves the editable fields and never sends the 2FA flag', () => {
    const { api } = setupOrgContext('owner');
    const fixture = create();
    typeName(fixture, 'Acme Labs');
    buttonNamed('Guardar cambios')?.click();
    fixture.detectChanges();

    expect(api['updateOrganization']).toHaveBeenCalledTimes(1);
    const [id, patch] = api['updateOrganization'].mock.calls[0] as [
      string,
      Record<string, unknown>,
    ];
    expect(id).toBe(ORG_ID);
    expect(patch).toMatchObject({ name: 'Acme Labs' });
    expect(JSON.stringify(patch)).not.toMatch(/require2fa|require_2fa/);
    expect(notifySuccess).toHaveBeenCalledWith('Configuración guardada');
  });

  it('does not offer features that are not available yet', () => {
    setupOrgContext('owner');
    create();
    expect(document.body.textContent).not.toContain('Exigir verificación');
    expect(document.body.textContent).not.toContain('Disponible próximamente');
  });

  it('groups the settings into cards and offers the creation policies as radio cards', () => {
    setupOrgContext('owner');
    create();
    const titles = Array.from(document.querySelectorAll('app-section h2')).map((el) =>
      el.textContent?.trim(),
    );
    expect(titles).toEqual(['Identidad', 'Logo', 'Políticas', 'Tiempo', 'Zona de peligro']);

    expect(document.querySelectorAll('app-policy-choice input[type="radio"]').length).toBe(4);
    const projects = document.querySelector('[aria-label="Quién puede crear proyectos"]');
    const checked = projects?.querySelector('input:checked') as HTMLInputElement;
    expect(checked.value).toBe('members');
  });

  it('saves a changed creation policy', () => {
    const { api } = setupOrgContext('owner');
    const fixture = create();
    const teams = document.querySelector('[aria-label="Quién puede crear equipos"]');
    const other = teams?.querySelector('input:not(:checked)') as HTMLInputElement;
    other.click();
    fixture.detectChanges();
    buttonNamed('Guardar cambios')?.click();
    fixture.detectChanges();

    const [, patch] = api['updateOrganization'].mock.calls[0] as [string, Record<string, unknown>];
    expect(patch['settings']).toMatchObject({ whoCanCreateTeams: 'members' });
  });

  it('lets an admin edit settings but not transfer or delete the organization', () => {
    setupOrgContext('admin');
    create();
    expect(document.body.textContent).not.toContain('Solo lectura');
    expect((fieldLabelled(/Nombre/) as HTMLInputElement).disabled).toBe(false);
    expect(buttonNamed('Transferir propiedad')).toBeUndefined();
    expect(buttonNamed('Eliminar organización')).toBeUndefined();
    expect(buttonNamed('Salir de la organización')).toBeDefined();
  });

  it('shows a read-only banner and disables the form for a member', () => {
    setupOrgContext('member');
    create();
    expect(document.body.textContent).toContain('Solo lectura');
    expect((fieldLabelled(/Nombre/) as HTMLInputElement).disabled).toBe(true);
    expect(buttonNamed('Guardar cambios')?.disabled).toBe(true);
    expect(buttonNamed('Salir de la organización')?.disabled).toBe(false);
  });

  it('maps server field errors to the form', () => {
    const error: ApiError = {
      status: 422,
      code: 'org.settings_invalid',
      message: 'Revisa la configuración',
      details: [{ field: 'settings.hours_per_day', message: 'Debe estar entre 1 y 24' }],
    };
    setupOrgContext('owner', { updateOrganization: () => throwError(() => error) });
    const fixture = create();
    typeName(fixture, 'Acme Labs');
    buttonNamed('Guardar cambios')?.click();
    fixture.detectChanges();
    expect(document.body.textContent).toContain('Debe estar entre 1 y 24');
  });

  it('reports the last owner error when leaving the organization', () => {
    const lastOwner: ApiError = {
      status: 409,
      code: 'org.last_owner',
      message: 'Es el último propietario',
      details: [],
    };
    const { api } = setupOrgContext('owner', { leave: () => throwError(() => lastOwner) });
    const fixture = create();
    buttonNamed('Salir de la organización')?.click();
    fixture.detectChanges();

    const dialog = document.querySelector('[role="dialog"]') as HTMLElement;
    expect(dialog.textContent).toContain('¿Salir de la organización?');
    const confirm = Array.from(dialog.querySelectorAll('button')).find(
      (b) => b.textContent?.trim() === 'Salir',
    );
    confirm?.click();
    fixture.detectChanges();

    expect(notifyError).toHaveBeenCalled();
    expect(String(notifyError.mock.calls[0][0])).toMatch(/último propietario/);
    expect(api['leave']).toHaveBeenCalledWith(ORG_ID);
  });

  it('deletes the organization only after the slug is typed', () => {
    const { api } = setupOrgContext('owner');
    const fixture = create();
    buttonNamed('Eliminar organización')?.click();
    fixture.detectChanges();

    const dialog = document.querySelector('[role="dialog"]') as HTMLElement;
    const confirm = Array.from(dialog.querySelectorAll('button')).find(
      (b) => b.textContent?.trim() === 'Eliminar',
    ) as HTMLButtonElement;
    expect(confirm.disabled).toBe(true);

    const typed = dialog.querySelector('input') as HTMLInputElement;
    typed.value = 'acme';
    typed.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    confirm.click();
    fixture.detectChanges();

    expect(api['deleteOrganization']).toHaveBeenCalledWith(ORG_ID, 'acme');
  });

  it('sends the owner to the portal with a full-page move after leaving the organization', () => {
    const assign = vi.fn();
    const { api } = setupOrgContext('owner', { leave: () => of(undefined) }, [
      { provide: LOCATION_ASSIGN, useValue: assign },
    ]);
    const fixture = create();
    buttonNamed('Salir de la organización')?.click();
    fixture.detectChanges();
    const dialog = document.querySelector('[role="dialog"]') as HTMLElement;
    Array.from(dialog.querySelectorAll('button'))
      .find((b) => b.textContent?.trim() === 'Salir')
      ?.click();
    fixture.detectChanges();

    expect(api['leave']).toHaveBeenCalledWith(ORG_ID);
    expect(assign).toHaveBeenCalledWith('/');
  });

  it('offers only members as transfer targets, excluding the caller', () => {
    setupOrgContext('owner', {
      listMembers: () =>
        of([memberOf('me', 'Yo Mismo', 'owner'), memberOf('u-2', 'Luis Pardo', 'member')]),
    });
    const fixture = create();
    const zone = fixture.debugElement.query(By.directive(OrgDangerZone))
      .componentInstance as OrgDangerZone;
    const labels = zone.transferOptions().map((option) => option.label);
    expect(labels).toEqual(['Luis Pardo (u-2@acme.test)']);
    expect(labels.join(' ')).not.toContain('Yo Mismo');
  });
});
