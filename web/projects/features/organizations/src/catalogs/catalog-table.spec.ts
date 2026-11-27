import { TestBed } from '@angular/core/testing';
import { CatalogItem, CatalogKind } from '@corelog/core';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { CatalogTable } from './catalog-table';

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
  item('epic', 'Épica', { hierarchyLevel: 1, icon: 'epic', usageCount: 1 }),
  item('task', 'Tarea', { isDefault: true, usageCount: 7 }),
  item('sub', 'Subtarea', { hierarchyLevel: -1 }),
];

function mount(items: CatalogItem[], canManage = true, kind: CatalogKind = 'issue-types') {
  TestBed.configureTestingModule({});
  const fixture = TestBed.createComponent(CatalogTable);
  fixture.componentRef.setInput('kind', kind);
  fixture.componentRef.setInput('items', items);
  fixture.componentRef.setInput('canManage', canManage);
  const reordered = vi.fn();
  const deleteRequested = vi.fn();
  const defaultRequested = vi.fn();
  fixture.componentInstance.reordered.subscribe(reordered);
  fixture.componentInstance.deleteRequested.subscribe(deleteRequested);
  fixture.componentInstance.defaultRequested.subscribe(defaultRequested);
  fixture.detectChanges();
  const root = fixture.nativeElement as HTMLElement;
  const button = (label: string) =>
    root.querySelector<HTMLButtonElement>(`button[aria-label="${label}"]`);
  return { fixture, root, button, reordered, deleteRequested, defaultRequested };
}

describe('CatalogTable', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('lists each item with its usage count, default mark and hierarchy level', () => {
    const { root } = mount(TYPES);
    const rows = Array.from(root.querySelectorAll('[data-testid="catalog-row"]'));
    expect(rows.map((row) => row.getAttribute('data-name'))).toEqual([
      'Épica',
      'Tarea',
      'Subtarea',
    ]);
    expect(rows[0].textContent).toContain('1 incidencia');
    expect(rows[0].textContent).toContain('Nivel épica');
    expect(rows[1].textContent).toContain('7 incidencias');
    expect(rows[1].textContent).toContain('Predeterminado');
    expect(rows[2].textContent).toContain('Sin incidencias');
  });

  it('moves a row with the keyboard buttons and emits the whole order', () => {
    const { button, reordered } = mount(TYPES);
    button('Bajar Épica')?.click();
    expect(reordered).toHaveBeenLastCalledWith(['task', 'epic', 'sub']);
    button('Subir Subtarea')?.click();
    expect(reordered).toHaveBeenLastCalledWith(['epic', 'sub', 'task']);
  });

  it('cannot move the first row up nor the last row down', () => {
    const { button } = mount(TYPES);
    expect(button('Subir Épica')?.disabled).toBe(true);
    expect(button('Bajar Subtarea')?.disabled).toBe(true);
  });

  it('protects the epic and subtask types from deletion with an explanation', () => {
    const { button, deleteRequested } = mount(TYPES);
    expect(button('Eliminar Épica')?.disabled).toBe(true);
    expect(button('Eliminar Subtarea')?.disabled).toBe(true);
    expect(button('Eliminar Tarea')?.disabled).toBe(false);
    button('Eliminar Tarea')?.click();
    expect(deleteRequested).toHaveBeenCalledWith(TYPES[1]);
  });

  it('offers to make an item the default only when it is not', () => {
    const { button, defaultRequested } = mount(TYPES);
    expect(button('Usar Tarea como predeterminado')).toBeNull();
    button('Usar Épica como predeterminado')?.click();
    expect(defaultRequested).toHaveBeenCalledWith(TYPES[0]);
  });

  it('is read-only without manage rights: no handles and no actions', () => {
    const { root } = mount(TYPES, false);
    expect(root.querySelector('button')).toBeNull();
    expect(root.querySelector('.catalog-table__handle')).toBeNull();
    expect(root.textContent).toContain('Tarea');
  });

  it('shows the empty state with a create action for managers', () => {
    const { root } = mount([], true, 'priorities');
    expect(root.textContent).toContain('Aún no hay prioridades');
    expect(root.textContent).toContain('Nueva prioridad');
  });

  it('does not draw icons for resolutions', () => {
    const { root } = mount([item('r', 'Hecho', { icon: null, color: null })], true, 'resolutions');
    expect(root.querySelector('app-catalog-icon')).toBeNull();
  });
});
