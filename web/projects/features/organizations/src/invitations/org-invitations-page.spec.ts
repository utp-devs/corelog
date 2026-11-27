import { ComponentFixture, TestBed } from '@angular/core/testing';
import { NotificationService, OrgRole, OrganizationApiService } from '@corelog/core';
import { Observable, of, throwError } from 'rxjs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ORG_ID, createdOf, invitationOf, setupOrgContext } from '../testing/org-test-kit';
import { OrgInvitationsPage } from './org-invitations-page';

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

type Overrides = Partial<
  Record<keyof OrganizationApiService, (...args: never[]) => Observable<unknown>>
>;

let notifySuccess: ReturnType<typeof vi.spyOn>;
let notifyError: ReturnType<typeof vi.spyOn>;

function create(role: OrgRole, overrides: Overrides = {}) {
  const context = setupOrgContext(role, overrides);
  const notify = TestBed.inject(NotificationService);
  notifySuccess = vi.spyOn(notify, 'success');
  notifyError = vi.spyOn(notify, 'error');
  const fixture: ComponentFixture<OrgInvitationsPage> = TestBed.createComponent(OrgInvitationsPage);
  fixture.detectChanges();
  fixture.detectChanges();
  return { ...context, fixture };
}

function buttonNamed(name: string, root: ParentNode = document): HTMLButtonElement | undefined {
  return Array.from(root.querySelectorAll('button')).find(
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

function typeEmails(fixture: ComponentFixture<unknown>, text: string): void {
  const field = fieldLabelled(/Correos electrónicos/) as HTMLTextAreaElement;
  field.value = text;
  field.dispatchEvent(new Event('input'));
  fixture.detectChanges();
}

function rowOf(text: string): HTMLElement {
  const row = Array.from(document.querySelectorAll('tr')).find((tr) =>
    tr.textContent?.includes(text),
  );
  if (!row) throw new Error(`no row for ${text}`);
  return row;
}

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

function rowActionItem(fixture: ComponentFixture<unknown>, text: string): void {
  Array.from(document.querySelectorAll<HTMLElement>('.p-menu-item-content'))
    .find((el) => el.textContent?.includes(text))
    ?.click();
  fixture.detectChanges();
}

describe('OrgInvitationsPage', () => {
  beforeEach(() => {
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText: vi.fn(async () => undefined) },
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('shows the read-only banner and the forbidden state to a member without loading anything', () => {
    const { api } = create('member');
    expect(document.body.textContent).toContain('Solo lectura');
    expect(document.body.textContent).toContain('No tienes permiso para ver esto');
    expect(api.listInvitations).not.toHaveBeenCalled();
  });

  it('shows the empty state with a call to action when nothing is pending', () => {
    const { fixture } = create('owner');
    expect(document.body.textContent).toContain('No hay invitaciones pendientes');
    expect(buttonNamed('Invitar personas', fixture.nativeElement)).toBeDefined();
    expect(document.querySelector('[data-page="org-invitations"]')?.getAttribute('data-fill')).toBe(
      'true',
    );
  });

  it('rejects an invalid email before calling the API', () => {
    const { api, fixture } = create('owner');
    typeEmails(fixture, 'ana@acme.test, no-es-correo');
    buttonNamed('Enviar invitaciones')?.click();
    fixture.detectChanges();
    expect(document.body.textContent).toContain('No válidos: no-es-correo');
    expect(api.createInvitations).not.toHaveBeenCalled();
  });

  it('creates one invitation per email and shows the one-time links with copy buttons', async () => {
    const { api, fixture } = create('owner', {
      createInvitations: () =>
        of([createdOf('i-1', 'ana@acme.test'), createdOf('i-2', 'luis@acme.test')]),
    });
    typeEmails(fixture, 'ana@acme.test; luis@acme.test');
    buttonNamed('Enviar invitaciones')?.click();
    fixture.detectChanges();

    expect(api.createInvitations).toHaveBeenCalledWith(ORG_ID, [
      { email: 'ana@acme.test', role: 'member' },
      { email: 'luis@acme.test', role: 'member' },
    ]);
    expect(document.body.textContent).toContain('Enlaces de invitación');
    expect(document.body.textContent).toContain('Este enlace solo se muestra una vez');

    buttonNamed('Copiar enlace de ana@acme.test')?.click();
    await vi.waitFor(() =>
      expect(navigator.clipboard.writeText).toHaveBeenCalledWith(
        createdOf('i-1', 'ana@acme.test').acceptUrl,
      ),
    );
    await vi.waitFor(() => expect(notifySuccess).toHaveBeenCalledWith('Enlace copiado'));
  });

  it('reports a clipboard failure instead of failing silently', async () => {
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText: vi.fn(async () => Promise.reject(new Error('denied'))) },
    });
    const { fixture } = create('owner', {
      createInvitations: () => of([createdOf('i-1', 'ana@acme.test')]),
    });
    typeEmails(fixture, 'ana@acme.test');
    buttonNamed('Enviar invitaciones')?.click();
    fixture.detectChanges();
    buttonNamed('Copiar enlace de ana@acme.test')?.click();
    await vi.waitFor(() => expect(notifyError).toHaveBeenCalled());
    expect(String(notifyError.mock.calls[0][0])).toContain('No se pudo copiar');
  });

  it('switches between pending and all invitations', () => {
    const { api, fixture } = create('owner', {
      listInvitations: () => of([invitationOf('i-1', 'ana@acme.test')]),
    });
    expect(document.body.textContent).toContain('ana@acme.test');
    const todas = Array.from(
      document.querySelectorAll<HTMLElement>('[role="button"], button'),
    ).find((el) => el.textContent?.trim() === 'Todas');
    todas?.click();
    fixture.detectChanges();
    expect(api.listInvitations).toHaveBeenLastCalledWith(ORG_ID, 'all');
  });

  it('resends an invitation and shows the new link', () => {
    const { api, fixture } = create('owner', {
      listInvitations: () => of([invitationOf('i-1', 'ana@acme.test')]),
    });
    (
      rowOf('ana@acme.test').querySelector(
        '[aria-label="Acciones de la fila"]',
      ) as HTMLButtonElement
    ).click();
    fixture.detectChanges();
    rowActionItem(fixture, 'Reenviar (nuevo enlace)');
    expect(api.resendInvitation).toHaveBeenCalledWith(ORG_ID, 'i-1');
    fixture.detectChanges();
    expect(buttonNamed('Copiar enlace de ana@acme.test')).toBeDefined();
  });

  it('revokes a pending invitation after confirmation', () => {
    const { api, fixture } = create('owner', {
      listInvitations: () => of([invitationOf('i-1', 'ana@acme.test')]),
    });
    (
      rowOf('ana@acme.test').querySelector(
        '[aria-label="Acciones de la fila"]',
      ) as HTMLButtonElement
    ).click();
    fixture.detectChanges();
    rowActionItem(fixture, 'Revocar');
    const dialog = document.querySelector('[role="dialog"]') as HTMLElement;
    buttonNamed('Revocar', dialog)?.click();
    fixture.detectChanges();
    expect(api.revokeInvitation).toHaveBeenCalledWith(ORG_ID, 'i-1');
  });

  it('offers no actions on accepted invitations', () => {
    create('owner', {
      listInvitations: () => of([invitationOf('i-2', 'marta@acme.test', 'accepted')]),
    });
    expect(document.body.textContent).toContain('marta@acme.test');
    expect(document.body.textContent).toContain('Aceptada');
    expect(document.querySelector('[aria-label="Acciones de la fila"]')).toBeNull();
    expect(document.body.textContent).not.toContain('Reenviar (nuevo enlace)');
  });

  it('limits the admin role choices to member and guest', () => {
    const { fixture } = create('admin');
    fieldLabelled(/Rol para todos/).click();
    fixture.detectChanges();
    const options = Array.from(document.querySelectorAll('[role="option"]')).map((el) =>
      el.textContent?.trim(),
    );
    expect(options).toContain('Invitado');
    expect(options).not.toContain('Administrador');
  });

  it('invites with the chosen role', () => {
    const { api, fixture } = create('owner');
    pickOption(fieldLabelled(/Rol para todos/), 'Invitado', fixture);
    typeEmails(fixture, 'marta@acme.test');
    buttonNamed('Enviar invitaciones')?.click();
    fixture.detectChanges();
    expect(api.createInvitations).toHaveBeenCalledWith(ORG_ID, [
      { email: 'marta@acme.test', role: 'guest' },
    ]);
  });
});
