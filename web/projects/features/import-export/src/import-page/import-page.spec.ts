import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import {
  CurrentProjectStore,
  ImportExportApiService,
  type ImportJob,
  type ImportPreview,
} from '@corelog/core';
import { of } from 'rxjs';
import { vi } from 'vitest';

import { ImportPage } from './import-page';

const PREVIEW: ImportPreview = {
  jobId: 'j1',
  columns: ['Titulo', 'Prioridad'],
  sampleRows: [['Uno', 'High']],
  suggestedMapping: { summary: 'Titulo', priority: 'Prioridad', labels: 'Inexistente' },
  totalRows: 3,
  delimiter: ',',
};

function job(patch: Partial<ImportJob> = {}): ImportJob {
  return {
    id: 'j1',
    status: 'done',
    totalRows: 3,
    processedRows: 3,
    createdRows: 2,
    errorRows: 1,
    dryRun: true,
    fileName: 'a.csv',
    errors: [{ row: 3, errors: ['Falta el resumen'] }],
    createdAt: '2026-01-01T00:00:00Z',
    finishedAt: null,
    ...patch,
  };
}

function setup(can = true) {
  const api = {
    upload: vi.fn(() => of(PREVIEW)),
    start: vi.fn((_p: string, _j: string, _m: unknown, dry: boolean) => of(job({ dryRun: dry }))),
    job: vi.fn((_p: string, id: string) => of(job({ id }))),
    errorsCsv: vi.fn(),
  };
  const current = {
    projectId: signal('p1'),
    project: signal({ id: 'p1', key: 'KAN' }),
    permissionsLoaded: signal(true),
    isArchived: signal(false),
    can: () => can,
  };
  TestBed.configureTestingModule({
    providers: [
      provideRouter([]),
      { provide: ImportExportApiService, useValue: api },
      { provide: CurrentProjectStore, useValue: current },
    ],
  });
  const fixture = TestBed.createComponent(ImportPage);
  fixture.detectChanges();
  return { fixture, api, el: fixture.nativeElement as HTMLElement };
}

async function settle(fixture: { detectChanges(): void; whenStable(): Promise<unknown> }) {
  for (let i = 0; i < 3; i += 1) {
    fixture.detectChanges();
    await fixture.whenStable();
    await new Promise((resolve) => setTimeout(resolve, 0));
  }
}

function pick(el: HTMLElement, file: File): void {
  const input = el.querySelector<HTMLInputElement>('[data-testid="import-file"]')!;
  Object.defineProperty(input, 'files', {
    value: { item: () => file, length: 1 },
    configurable: true,
  });
  input.dispatchEvent(new Event('change'));
}

describe('ImportPage', () => {
  it('refuses files that are not CSV or are too large without calling the server', async () => {
    const { fixture, api, el } = setup();
    pick(el, new File(['x'], 'datos.xlsx'));
    await settle(fixture);
    expect(api.upload).not.toHaveBeenCalled();
    expect(el.querySelector('[data-testid="import-error"]')?.textContent).toContain('CSV');
  });

  it('uploads a CSV, keeps the valid suggested mapping and moves to the columns step', async () => {
    const { fixture, api, el } = setup();
    pick(el, new File(['a,b\n1,2'], 'a.csv'));
    await settle(fixture);
    expect(api.upload).toHaveBeenCalled();
    expect(el.querySelector('[data-testid="import-step-map"]')).not.toBeNull();
    const select = el.querySelector<HTMLSelectElement>('[data-testid="mapping-select-0"]');
    expect(select?.value).toBe('summary');
  });

  it('runs a dry run, shows the counts and imports with a second job over the same file', async () => {
    const { fixture, api, el } = setup();
    pick(el, new File(['a,b\n1,2'], 'a.csv'));
    await settle(fixture);
    el.querySelector<HTMLButtonElement>('[data-testid="import-review"] button')?.click();
    await settle(fixture);
    expect(api.start).toHaveBeenCalledWith(
      'p1',
      'j1',
      { summary: 'Titulo', priority: 'Prioridad' },
      true,
    );
    expect(el.querySelector('[data-testid="dry-created"]')?.textContent).toContain('2');
    expect(el.querySelector('[data-testid="dry-error-table"]')?.textContent).toContain(
      'Falta el resumen',
    );

    el.querySelector<HTMLButtonElement>('[data-testid="import-start"] button')?.click();
    await settle(fixture);
    expect(api.upload).toHaveBeenCalledTimes(2);
    expect(api.start).toHaveBeenLastCalledWith(
      'p1',
      'j1',
      { summary: 'Titulo', priority: 'Prioridad' },
      false,
    );
    expect(el.querySelector('[data-testid="import-created"]')?.textContent).toContain('2');
    expect(el.querySelector('[data-testid="import-open-issues"]')).not.toBeNull();
  });

  it('tells people without permission that they cannot import', () => {
    const { el } = setup(false);
    expect(el.querySelector('[data-testid="import-forbidden"]')).not.toBeNull();
    expect(el.querySelector('[data-testid="import-drop"]')).toBeNull();
  });
});
