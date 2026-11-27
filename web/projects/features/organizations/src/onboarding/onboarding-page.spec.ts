import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ActivatedRoute, Router, convertToParamMap } from '@angular/router';
import { ApiError, Organization, OrganizationApiService } from '@corelog/core';
import { Observable, of, throwError } from 'rxjs';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { createdOf, orgOf, setupOrgContext } from '../testing/org-test-kit';
import { OnboardingPage } from './onboarding-page';

type Overrides = Partial<
  Record<keyof OrganizationApiService, (...args: never[]) => Observable<unknown>>
>;

function newOrg(id: string, name: string): Organization {
  return orgOf('owner', { id, name, slug: 'acme-corp' });
}

function create(path: { new?: boolean } = {}, overrides: Overrides = {}) {
  const query = path.new ? { new: '1' } : {};
  const context = setupOrgContext('owner', overrides, [
    {
      provide: ActivatedRoute,
      useValue: { snapshot: { queryParamMap: convertToParamMap(query) } },
    },
  ]);
  const router = TestBed.inject(Router);
  const navigate = vi.spyOn(router, 'navigateByUrl').mockResolvedValue(true);
  const fixture: ComponentFixture<OnboardingPage> = TestBed.createComponent(OnboardingPage);
  fixture.detectChanges();
  return { ...context, fixture, navigate };
}

function input(label: string): HTMLInputElement {
  const el = document.querySelector(`input[aria-label="${label}"]`);
  if (!el) throw new Error(`no input ${label}`);
  return el as HTMLInputElement;
}

function typeInto(el: HTMLInputElement, value: string, fixture: ComponentFixture<unknown>): void {
  el.value = value;
  el.dispatchEvent(new Event('input'));
  fixture.detectChanges();
}

function buttonNamed(name: string): HTMLButtonElement | undefined {
  return Array.from(document.querySelectorAll('button')).find(
    (b) => b.textContent?.trim() === name,
  );
}

function heading(name: string): HTMLElement | undefined {
  return Array.from(document.querySelectorAll<HTMLElement>('h2, h3')).find(
    (h) => h.textContent?.trim() === name,
  );
}

/** Fills the organization form (name derives the slug) and submits it. */
function fillOrgForm(fixture: ComponentFixture<unknown>, name = 'Acme Corp'): void {
  typeInto(input('Nombre de la organización'), name, fixture);
  buttonNamed('Crear organización')?.click();
  fixture.detectChanges();
}

describe('OnboardingPage', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    localStorage.clear();
  });

  it('starts at the choice step with create and join options', () => {
    create();
    expect(buttonNamed('Crear organización')).toBeDefined();
    expect(input('Enlace o código de invitación')).toBeDefined();
  });

  it('starts at the organization step with ?new=1', () => {
    create({ new: true });
    expect(heading('Datos de la organización')).toBeDefined();
    expect(document.querySelector('input[aria-label="Enlace o código de invitación"]')).toBeNull();
  });

  it('derives the slug from the name and keeps it editable', () => {
    const { fixture } = create({ new: true });
    typeInto(input('Nombre de la organización'), 'Acme Corp', fixture);
    expect(input('Identificador de la organización').value).toBe('acme-corp');
    typeInto(input('Identificador de la organización'), 'mi-org', fixture);
    typeInto(input('Nombre de la organización'), 'Otra', fixture);
    expect(input('Identificador de la organización').value).toBe('mi-org');
  });

  it('does not submit an invalid slug', () => {
    const { api, fixture } = create({ new: true });
    typeInto(input('Nombre de la organización'), 'Acme', fixture);
    typeInto(input('Identificador de la organización'), 'a', fixture);
    buttonNamed('Crear organización')?.click();
    fixture.detectChanges();
    expect(api.create).not.toHaveBeenCalled();
    expect(document.body.textContent).toContain('Usa 3 a 40 caracteres');
  });

  it('shows the server message when the slug is taken and stays on the form', () => {
    const taken: ApiError = { status: 409, code: 'org.slug_taken', message: 'taken', details: [] };
    const { fixture } = create({ new: true }, { create: () => throwError(() => taken) });
    fillOrgForm(fixture);
    expect(document.body.textContent).toContain('Ese identificador ya está en uso. Elige otro.');
    expect(heading('Datos de la organización')).toBeDefined();
  });

  it('shows the limit message when the user already owns five organizations', () => {
    const limit: ApiError = {
      status: 409,
      code: 'org.limit_reached',
      message: 'limit',
      details: [],
    };
    const { fixture } = create({ new: true }, { create: () => throwError(() => limit) });
    fillOrgForm(fixture);
    expect(document.body.textContent).toContain(
      'Has alcanzado el máximo de organizaciones que puedes crear (5).',
    );
  });

  it('creates the organization, makes it current and moves to the invite step', () => {
    const { api, store, fixture } = create(
      { new: true },
      { create: () => of(newOrg('o-new', 'Acme Corp')) },
    );
    fillOrgForm(fixture);
    expect(heading('Invita a tu equipo')).toBeDefined();
    expect(api.create).toHaveBeenCalledWith({ name: 'Acme Corp', slug: 'acme-corp' });
    expect(store.currentOrgId()).toBe('o-new');
  });

  it('validates invitation emails and sends the valid ones', () => {
    const { api, fixture } = create(
      { new: true },
      {
        create: () => of(newOrg('o-new', 'Acme Corp')),
        createInvitations: () => of([createdOf('i1', 'ana@example.com')]),
      },
    );
    fillOrgForm(fixture);

    typeInto(input('Correo del invitado 1'), 'no-es-correo', fixture);
    buttonNamed('Crear invitaciones')?.click();
    fixture.detectChanges();
    expect(document.body.textContent).toContain('Introduce un correo electrónico válido');
    expect(api.createInvitations).not.toHaveBeenCalled();

    typeInto(input('Correo del invitado 1'), 'ana@example.com', fixture);
    buttonNamed('Crear invitaciones')?.click();
    fixture.detectChanges();
    expect(
      document.querySelector('[aria-label="Copiar enlace de ana@example.com"]'),
    ).not.toBeNull();
    expect(api.createInvitations).toHaveBeenCalledWith('o-new', [
      { email: 'ana@example.com', role: 'member' },
    ]);
  });

  it('lets the user skip the invite step and finish in the projects page', () => {
    const { fixture, navigate } = create(
      { new: true },
      { create: () => of(newOrg('o-new', 'Acme Corp')) },
    );
    fillOrgForm(fixture);
    buttonNamed('Omitir este paso')?.click();
    fixture.detectChanges();
    expect(heading('Todo listo')).toBeDefined();

    buttonNamed('Ir a proyectos')?.click();
    expect(navigate).toHaveBeenCalledWith('/projects', { replaceUrl: true });
  });

  it('takes a pasted invitation link to the accept page', () => {
    const { fixture, navigate } = create();
    typeInto(
      input('Enlace o código de invitación'),
      'http://localhost:5173/invite/tok-abcdefghijklmnop',
      fixture,
    );
    buttonNamed('Continuar')?.click();
    expect(navigate).toHaveBeenCalledWith('/invite/tok-abcdefghijklmnop');
  });

  it('rejects text that is not an invitation link', () => {
    const { fixture, navigate } = create();
    typeInto(input('Enlace o código de invitación'), 'hola', fixture);
    buttonNamed('Continuar')?.click();
    fixture.detectChanges();
    expect(document.body.textContent).toContain('Introduce un enlace de invitación válido.');
    expect(navigate).not.toHaveBeenCalled();
  });
});
