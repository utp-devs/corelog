import { Observable, of, throwError } from 'rxjs';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ApiError, NotificationService, OrgRole, OrganizationApiService } from '@corelog/core';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { ME_ID, ORG_ID, memberOf, setupOrgContext } from '../testing/org-test-kit';
import { OrgMembersPage } from './org-members-page';

type Overrides = Partial<
  Record<keyof OrganizationApiService, (...args: never[]) => Observable<unknown>>
>;

if (!window.matchMedia) {
  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    value: (query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: () => undefined,
      removeListener: () => undefined,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
      dispatchEvent: () => false,
    }),
  });
}

let notifySuccess: ReturnType<typeof vi.spyOn>;
let notifyError: ReturnType<typeof vi.spyOn>;

function team() {
  return [
    memberOf(ME_ID, 'Yo Mismo', 'owner'),
    memberOf('u-2', 'Luis Pardo', 'member'),
    memberOf('u-3', 'Marta Ruiz', 'guest'),
    memberOf('u-4', 'Carla Vega', 'admin'),
  ];
}

function create(role: OrgRole, overrides: Overrides = {}) {
  const context = setupOrgContext(role, { listMembers: () => of(team()), ...overrides });
  const notify = TestBed.inject(NotificationService);
  notifySuccess = vi.spyOn(notify, 'success');
  notifyError = vi.spyOn(notify, 'error');
  const fixture: ComponentFixture<OrgMembersPage> = TestBed.createComponent(OrgMembersPage);
  fixture.detectChanges();
  fixture.detectChanges();
  return { ...context, fixture };
}

function byAria(label: string): HTMLElement | null {
  return document.querySelector(`[aria-label="${label}"]`);
}

function buttonNamed(name: string, root: ParentNode = document): HTMLButtonElement | undefined {
  return Array.from(root.querySelectorAll('button')).find(
    (button) => button.textContent?.trim() === name,
  );
}

function rowOf(name: string): HTMLElement {
  const row = Array.from(document.querySelectorAll('tr')).find((tr) =>
    tr.textContent?.includes(name),
  );
  if (!row) throw new Error(`no row for ${name}`);
  return row;
}

/** Opens a PrimeNG select and picks the option with the given label from the open overlay. */
function pickOption(
  combobox: HTMLElement,
  label: string,
  fixture: ComponentFixture<unknown>,
): void {
  combobox.click();
  fixture.detectChanges();
  const option = Array.from(document.querySelectorAll<HTMLElement>('[role="option"]')).find(
    (el) => el.textContent?.trim() === label,
  );
  if (!option) throw new Error(`option ${label} not open`);
  option.click();
  fixture.detectChanges();
}

describe('OrgMembersPage', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('shows every role select for an owner except on their own row', () => {
    create('owner');
    expect(byAria('Rol de Luis Pardo')).not.toBeNull();
    expect(byAria('Rol de Carla Vega')).not.toBeNull();
    expect(byAria('Rol de Yo Mismo')).toBeNull();
    expect(document.body.textContent).toContain('(tú)');
    expect(document.body.textContent).not.toContain('Solo lectura');
    expect(document.querySelector('[data-page="org-members"]')?.getAttribute('data-fill')).toBe(
      'true',
    );
  });

  it('limits an admin to changing members and guests', () => {
    create('admin');
    expect(byAria('Rol de Luis Pardo')).not.toBeNull();
    expect(byAria('Rol de Marta Ruiz')).not.toBeNull();
    expect(byAria('Rol de Carla Vega')).toBeNull();
    expect(byAria('Rol de Yo Mismo')).toBeNull();
  });

  it('shows a read-only banner and no controls for a member', () => {
    create('member');
    expect(document.body.textContent).toContain('Solo lectura');
    expect(document.querySelector('[aria-label^="Rol de "]')).toBeNull();
    expect(buttonNamed('Invitar personas')).toBeUndefined();
  });

  it('shows the forbidden state for a guest and does not load the list', () => {
    const { api } = create('guest');
    expect(document.body.textContent).toContain('No tienes permiso para ver esto');
    expect(api.listMembers).not.toHaveBeenCalled();
  });

  it('confirms a role change before calling the API', () => {
    const { api, fixture } = create('owner');
    pickOption(byAria('Rol de Luis Pardo') as HTMLElement, 'Administrador', fixture);

    expect(document.body.textContent).toContain(
      '¿Cambiar el rol de Luis Pardo de Miembro a Administrador?',
    );
    expect(api.changeMemberRole).not.toHaveBeenCalled();

    buttonNamed('Cambiar rol')?.click();
    fixture.detectChanges();
    expect(api.changeMemberRole).toHaveBeenCalledWith(ORG_ID, 'u-2', 'admin');
    expect(notifySuccess).toHaveBeenCalledWith('Rol de Luis Pardo actualizado');
  });

  it('does not call the API when the role confirmation is cancelled', () => {
    const { api, fixture } = create('owner');
    pickOption(byAria('Rol de Luis Pardo') as HTMLElement, 'Invitado', fixture);
    expect(document.body.textContent).toContain('¿Cambiar el rol');
    buttonNamed('Cancelar')?.click();
    fixture.detectChanges();
    expect(api.changeMemberRole).not.toHaveBeenCalled();
  });

  it('shows the last owner explanation when the API refuses the change', () => {
    const lastOwner: ApiError = {
      status: 409,
      code: 'org.last_owner',
      message: 'Es el último propietario',
      details: [],
    };
    const { fixture } = create('owner', { changeMemberRole: () => throwError(() => lastOwner) });
    pickOption(byAria('Rol de Luis Pardo') as HTMLElement, 'Administrador', fixture);
    buttonNamed('Cambiar rol')?.click();
    fixture.detectChanges();

    const alert = document.querySelector('[role="alert"]') as HTMLElement;
    expect(alert.textContent).toContain('Es el último propietario de la organización');
  });

  it('removes one member after typing the email in the danger confirmation', () => {
    const { api, fixture } = create('owner');
    const row = rowOf('Luis Pardo');
    (row.querySelector('[aria-label="Acciones de la fila"]') as HTMLButtonElement).click();
    fixture.detectChanges();
    const item = Array.from(document.querySelectorAll<HTMLElement>('.p-menu-item-content')).find(
      (el) => el.textContent?.includes('Quitar de la organización'),
    );
    item?.click();
    fixture.detectChanges();

    const dialog = document.querySelector('[role="dialog"]') as HTMLElement;
    const confirm = buttonNamed('Quitar', dialog) as HTMLButtonElement;
    expect(confirm.disabled).toBe(true);
    const typed = dialog.querySelector('input') as HTMLInputElement;
    typed.value = 'u-2@acme.test';
    typed.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    confirm.click();
    fixture.detectChanges();
    expect(api.removeMember).toHaveBeenCalledWith(ORG_ID, 'u-2');
  });

  it('filters by role and by search text', () => {
    const { fixture } = create('owner');
    expect(document.body.textContent).toContain('Marta Ruiz');
    pickOption(byAria('Filtrar por rol') as HTMLElement, 'Invitado', fixture);
    expect(document.body.textContent).not.toContain('Luis Pardo');
    expect(document.body.textContent).toContain('Marta Ruiz');
  });

  it('shows the empty state with an invitation call to action when there are no members', () => {
    create('owner', { listMembers: () => of([]) });
    expect(document.body.textContent).toContain('Aún no hay miembros');
    expect(buttonNamed('Invitar personas')).toBeDefined();
  });

  it('shows the load error with a retry that reloads the list', () => {
    const failure: ApiError = {
      status: 500,
      code: 'http.500',
      message: 'Fallo del servidor',
      details: [],
    };
    const listMembers = vi
      .fn()
      .mockReturnValueOnce(throwError(() => failure))
      .mockReturnValue(of(team()));
    const { api, fixture } = create('owner', {
      listMembers: listMembers as Overrides['listMembers'],
    });
    expect(document.body.textContent).toContain('Fallo del servidor');

    buttonNamed('Reintentar')?.click();
    fixture.detectChanges();
    expect(document.body.textContent).toContain('Luis Pardo');
    expect(api.listMembers).toHaveBeenCalledTimes(2);
  });
});
