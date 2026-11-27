import { ComponentFixture, TestBed } from '@angular/core/testing';
import { OrgRole, OrganizationStatus, WorkflowApiService } from '@corelog/core';
import { Observable, of, throwError } from 'rxjs';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { setupOrgContext } from '../testing/org-test-kit';
import { OrgStatusesPage } from './org-statuses-page';

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

const STATUSES: OrganizationStatus[] = [
  { id: 's1', name: 'Por hacer', category: 'todo', position: 0 },
  { id: 's2', name: 'En curso', category: 'in_progress', position: 1 },
  { id: 's3', name: 'Hecho', category: 'done', position: 2 },
];

type Api = Record<
  'listStatuses' | 'createStatus' | 'updateStatus' | 'deleteStatus' | 'reorderStatuses',
  ReturnType<typeof vi.fn>
>;

function mount(
  role: OrgRole,
  overrides: Partial<Record<keyof Api, () => Observable<unknown>>> = {},
) {
  const api: Api = {
    listStatuses: vi.fn(() => of(STATUSES)),
    createStatus: vi.fn(() => of(STATUSES[0])),
    updateStatus: vi.fn(() => of(STATUSES[0])),
    deleteStatus: vi.fn(() => of(0)),
    reorderStatuses: vi.fn((_org: string, ids: string[]) =>
      of(ids.map((id, position) => ({ ...STATUSES.find((s) => s.id === id)!, position }))),
    ),
  };
  for (const [key, impl] of Object.entries(overrides)) api[key as keyof Api] = vi.fn(impl);
  setupOrgContext(role, {}, [{ provide: WorkflowApiService, useValue: api }]);
  const fixture: ComponentFixture<OrgStatusesPage> = TestBed.createComponent(OrgStatusesPage);
  fixture.detectChanges();
  fixture.detectChanges();
  return { api, fixture };
}

function click(label: string): void {
  const target = Array.from(document.querySelectorAll('button')).find(
    (button) => button.textContent?.trim() === label || button.getAttribute('aria-label') === label,
  );
  if (!target) throw new Error(`no button ${label}`);
  target.click();
}

function rowNames(): string[] {
  return Array.from(document.querySelectorAll('[data-testid="status-row"]')).map(
    (row) => row.getAttribute('data-name') ?? '',
  );
}

describe('OrgStatusesPage', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('lists the statuses with their category', () => {
    mount('owner');
    expect(rowNames()).toEqual(['Por hacer', 'En curso', 'Hecho']);
    expect(document.body.textContent).toContain('Por hacer');
    expect(document.body.textContent).toContain('Hecho');
  });

  it('is read-only for members that do not administer the organization', () => {
    mount('member');
    expect(rowNames()).toHaveLength(3);
    expect(document.body.textContent).toContain('Solo lectura');
    expect(document.querySelector('[data-testid="status-create"]')).toBeNull();
    expect(document.querySelector('button[aria-label="Eliminar Hecho"]')).toBeNull();
  });

  it('reorders with the buttons and sends the whole order', () => {
    const { api, fixture } = mount('owner');
    click('Bajar Por hacer');
    fixture.detectChanges();
    expect(api.reorderStatuses).toHaveBeenCalledWith('org-1', ['s2', 's1', 's3']);
  });

  it('creates a status from the dialog', () => {
    const { api, fixture } = mount('owner');
    click('Nuevo estado');
    fixture.detectChanges();
    const input = document.querySelector('app-dialog input[type="text"]') as HTMLInputElement;
    input.value = 'Bloqueado';
    input.dispatchEvent(new Event('input', { bubbles: true }));
    click('Guardar');
    fixture.detectChanges();
    expect(api.createStatus).toHaveBeenCalledWith('org-1', { name: 'Bloqueado', category: 'todo' });
  });

  it('asks where the issues go when the status is in use and then deletes with the target', () => {
    let attempt = 0;
    const { api, fixture } = mount('owner', {
      deleteStatus: () => {
        attempt += 1;
        return attempt === 1
          ? throwError(() => ({
              status: 409,
              code: 'workflow.status_in_use',
              message: 'en uso',
              details: [],
            }))
          : of(2);
      },
    });
    click('Eliminar Hecho');
    fixture.detectChanges();
    click('Eliminar');
    fixture.detectChanges();
    expect(document.body.textContent).toContain('Elige a qué estado pasarlos');
    expect(
      (document.querySelector('[data-testid="status-delete-confirm"] button') as HTMLButtonElement)
        .disabled,
    ).toBe(true);
    expect(api.deleteStatus).toHaveBeenCalledTimes(1);
  });
});
