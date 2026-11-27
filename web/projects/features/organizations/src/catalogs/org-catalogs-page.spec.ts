import { ComponentFixture, TestBed } from '@angular/core/testing';
import { CatalogApiService, CatalogItem, NotificationService, OrgRole } from '@corelog/core';
import { Observable, of, throwError } from 'rxjs';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { setupOrgContext } from '../testing/org-test-kit';
import { OrgCatalogsPage } from './org-catalogs-page';

if (!window.matchMedia) {
  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    writable: true,
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

function item(id: string, name: string, extra: Partial<CatalogItem> = {}): CatalogItem {
  return {
    id,
    name,
    description: '',
    icon: 'task',
    color: 'blue',
    hierarchyLevel: 0,
    position: 0,
    isDefault: false,
    usageCount: 0,
    createdAt: '',
    updatedAt: '',
    ...extra,
  };
}

const TYPES = [
  item('epic', 'Épica', { hierarchyLevel: 1, icon: 'epic' }),
  item('task', 'Tarea', { isDefault: true, usageCount: 3 }),
  item('bug', 'Error', { icon: 'bug', color: 'red' }),
  item('sub', 'Subtarea', { hierarchyLevel: -1 }),
];
const PRIORITIES = [
  item('p1', 'Alta', { icon: 'arrow-up' }),
  item('p2', 'Media', { isDefault: true }),
];
const RESOLUTIONS = [item('r1', 'Hecho', { icon: null, color: null, isDefault: true })];

type Api = Record<'list' | 'create' | 'update' | 'remove' | 'reorder', ReturnType<typeof vi.fn>>;

function mount(
  role: OrgRole,
  overrides: Partial<Record<keyof Api, () => Observable<unknown>>> = {},
) {
  const api: Api = {
    list: vi.fn((_org: string, kind: string) =>
      of(kind === 'issue-types' ? TYPES : kind === 'priorities' ? PRIORITIES : RESOLUTIONS),
    ),
    create: vi.fn(() => of(item('new', 'Nuevo'))),
    update: vi.fn(() => of(item('x', 'X'))),
    remove: vi.fn(() => of(undefined)),
    reorder: vi.fn((_org: string, _kind: string, ids: string[]) =>
      of(ids.map((id, position) => ({ ...TYPES.find((t) => t.id === id)!, position }))),
    ),
  };
  for (const [key, impl] of Object.entries(overrides)) api[key as keyof Api] = vi.fn(impl);
  setupOrgContext(role, {}, [{ provide: CatalogApiService, useValue: api }]);
  const notify = TestBed.inject(NotificationService);
  const success = vi.spyOn(notify, 'success');
  const fixture: ComponentFixture<OrgCatalogsPage> = TestBed.createComponent(OrgCatalogsPage);
  fixture.detectChanges();
  fixture.detectChanges();
  return { api, fixture, success };
}

function click(label: string): void {
  const target = Array.from(document.querySelectorAll('button')).find(
    (button) => button.textContent?.trim() === label || button.getAttribute('aria-label') === label,
  );
  if (!target) throw new Error(`no button ${label}`);
  target.click();
}

function rowNames(): string[] {
  return Array.from(document.querySelectorAll('[data-testid="catalog-row"]')).map(
    (row) => row.getAttribute('data-name') ?? '',
  );
}

describe('OrgCatalogsPage', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('lists the issue types first and switches tabs with the arrow keys', () => {
    const { fixture } = mount('owner');
    expect(rowNames()).toEqual(['Épica', 'Tarea', 'Error', 'Subtarea']);
    const tabs = Array.from(document.querySelectorAll<HTMLElement>('[role="tab"]'));
    expect(tabs.map((tab) => tab.textContent?.trim())).toEqual([
      'Tipos de incidencia',
      'Prioridades',
      'Resoluciones',
    ]);
    tabs[0].dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }));
    fixture.detectChanges();
    expect(rowNames()).toEqual(['Alta', 'Media']);
    expect(tabs[1].getAttribute('aria-selected')).toBe('true');
  });

  it('is read-only for members: a notice, no create button and no row actions', () => {
    mount('member');
    expect(document.body.textContent).toContain('Solo lectura');
    expect(document.querySelector('[data-testid="catalog-create"]')).toBeNull();
    expect(document.querySelector('[data-testid="catalog-row"] button')).toBeNull();
    expect(rowNames()).toHaveLength(4);
  });

  it('creates a type from the dialog with the chosen name, color and icon', () => {
    const { api, fixture, success } = mount('admin');
    click('Nuevo tipo');
    fixture.detectChanges();
    const name = document.querySelector<HTMLInputElement>('input[formcontrolname="name"]');
    if (!name) throw new Error('no name field');
    name.value = 'Hotfix';
    name.dispatchEvent(new Event('input'));
    document.querySelector<HTMLElement>('[role="radio"][aria-label="Rosa"]')?.click();
    fixture.detectChanges();
    click('Guardar');
    fixture.detectChanges();

    expect(api.create).toHaveBeenCalledWith(
      'org-1',
      'issue-types',
      expect.objectContaining({ name: 'Hotfix', color: 'pink', icon: 'task' }),
    );
    expect(success).toHaveBeenCalledWith('Se creó Nuevo');
  });

  it('shows the Spanish message of a refused create inside the dialog', () => {
    const taken = { status: 409, code: 'catalog.name_taken', message: 'x', details: [] };
    const { fixture } = mount('owner', { create: () => throwError(() => taken) });
    click('Nuevo tipo');
    fixture.detectChanges();
    const name = document.querySelector<HTMLInputElement>('input[formcontrolname="name"]');
    if (!name) throw new Error('no name field');
    name.value = 'Tarea';
    name.dispatchEvent(new Event('input'));
    click('Guardar');
    fixture.detectChanges();
    expect(document.querySelector('[role="alert"]')?.textContent).toContain(
      'Ya existe un elemento',
    );
  });

  it('sends the new order of the catalog when a row moves', () => {
    const { api, fixture } = mount('owner');
    click('Bajar Tarea');
    fixture.detectChanges();
    expect(api.reorder).toHaveBeenCalledWith('org-1', 'issue-types', [
      'epic',
      'bug',
      'task',
      'sub',
    ]);
  });

  it('asks for a destination before deleting a type that is in use', () => {
    const { api, fixture } = mount('owner');
    click('Eliminar Tarea');
    fixture.detectChanges();
    expect(document.querySelector('[data-testid="catalog-delete-in-use"]')?.textContent).toContain(
      '3 incidencias',
    );
    const confirm = Array.from(document.querySelectorAll('button')).find(
      (button) => button.textContent?.trim() === 'Eliminar',
    );
    expect(confirm?.disabled).toBe(true);
    expect(api.remove).not.toHaveBeenCalled();
  });

  it('deletes an unused item without a destination', () => {
    const { api, fixture } = mount('owner');
    click('Eliminar Error');
    fixture.detectChanges();
    expect(document.querySelector('[data-testid="catalog-delete-in-use"]')).toBeNull();
    const confirm = Array.from(document.querySelectorAll('button')).find(
      (button) => button.textContent?.trim() === 'Eliminar',
    );
    confirm?.click();
    fixture.detectChanges();
    expect(api.remove).toHaveBeenCalledWith('org-1', 'issue-types', 'bug', undefined);
  });

  it('marks another item as the default', () => {
    const { api, fixture } = mount('owner');
    click('Usar Error como predeterminado');
    fixture.detectChanges();
    expect(api.update).toHaveBeenCalledWith('org-1', 'issue-types', 'bug', { isDefault: true });
  });
});
