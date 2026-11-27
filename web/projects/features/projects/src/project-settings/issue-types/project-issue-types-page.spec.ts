import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { CatalogItem, CatalogStore } from '@corelog/core';
import { of, throwError } from 'rxjs';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { mountSettingsPage } from '../settings-spec-helpers';
import { settingsEnvironment } from '../settings-test-kit';
import { ProjectIssueTypesPage } from './project-issue-types-page';

function item(id: string, name: string, level: -1 | 0 | 1 = 0): CatalogItem {
  return {
    id,
    name,
    description: '',
    icon: 'task',
    color: 'blue',
    hierarchyLevel: level,
    position: 0,
    isDefault: false,
    usageCount: 0,
    createdAt: '',
    updatedAt: '',
  };
}

const ORG_TYPES = [
  item('epic', 'Épica', 1),
  item('task', 'Tarea'),
  item('bug', 'Error'),
  item('sub', 'Subtarea', -1),
];

function catalog(projectTypes: CatalogItem[], configured: boolean) {
  const save = vi.fn((_id: string, ids: readonly string[]) =>
    of({ items: ORG_TYPES.filter((type) => ids.includes(type.id)), configured: true }),
  );
  return {
    save,
    store: {
      issueTypes: signal(ORG_TYPES),
      ensureLoaded: vi.fn(),
      projectIssueTypes: vi.fn(() => of({ items: projectTypes, configured })),
      saveProjectIssueTypes: save,
    },
  };
}

function checkbox(name: string): HTMLInputElement {
  const row = document.querySelector(`[data-name="${name}"] input`);
  if (!row) throw new Error(`no row ${name}`);
  return row as HTMLInputElement;
}

function saveButton(): HTMLButtonElement | null {
  return document.querySelector('[data-testid="project-issue-types-save"] button');
}

describe('ProjectIssueTypesPage', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('lists every organization type, all on when the project has no own choice', async () => {
    const { store } = catalog(ORG_TYPES, false);
    await mountSettingsPage(ProjectIssueTypesPage, settingsEnvironment(), [
      { provide: CatalogStore, useValue: store },
    ]);
    for (const name of ['Épica', 'Tarea', 'Error', 'Subtarea']) {
      expect(checkbox(name).checked).toBe(true);
    }
    expect(document.body.textContent).toContain('usa todos los tipos de la organización');
  });

  it('keeps the epic and subtask always on', async () => {
    const { store } = catalog(ORG_TYPES, false);
    await mountSettingsPage(ProjectIssueTypesPage, settingsEnvironment(), [
      { provide: CatalogStore, useValue: store },
    ]);
    expect(checkbox('Épica').disabled).toBe(true);
    expect(checkbox('Subtarea').disabled).toBe(true);
    expect(checkbox('Tarea').disabled).toBe(false);
  });

  it('does not let the last standard type be switched off', async () => {
    const { store } = catalog([ORG_TYPES[0], ORG_TYPES[1], ORG_TYPES[3]], true);
    await mountSettingsPage(ProjectIssueTypesPage, settingsEnvironment(), [
      { provide: CatalogStore, useValue: store },
    ]);
    expect(checkbox('Tarea').checked).toBe(true);
    expect(checkbox('Tarea').disabled).toBe(true);
    expect(checkbox('Error').checked).toBe(false);
  });

  it('saves the chosen types in catalog order and enables saving only after a change', async () => {
    const { store, save } = catalog(ORG_TYPES, false);
    const fixture = await mountSettingsPage(ProjectIssueTypesPage, settingsEnvironment(), [
      { provide: CatalogStore, useValue: store },
    ]);
    expect(saveButton()?.disabled).toBe(true);

    checkbox('Error').click();
    fixture.detectChanges();
    expect(saveButton()?.disabled).toBe(false);
    saveButton()?.click();
    fixture.detectChanges();

    expect(save).toHaveBeenCalledWith('p-1', ['epic', 'task', 'sub']);
    expect(saveButton()?.disabled).toBe(true);
  });

  it('shows the Spanish message when the API refuses the choice', async () => {
    const { store } = catalog(ORG_TYPES, false);
    store.saveProjectIssueTypes = vi.fn(() =>
      throwError(() => ({
        status: 422,
        code: 'project.issue_types_required',
        message: 'x',
        details: [],
      })),
    );
    const fixture = await mountSettingsPage(ProjectIssueTypesPage, settingsEnvironment(), [
      { provide: CatalogStore, useValue: store },
    ]);
    checkbox('Error').click();
    fixture.detectChanges();
    saveButton()?.click();
    fixture.detectChanges();
    expect(document.querySelector('[role="alert"]')?.textContent).toContain('al menos un tipo');
  });

  it('is read-only for people who cannot edit the project', async () => {
    const { store } = catalog(ORG_TYPES, false);
    await mountSettingsPage(ProjectIssueTypesPage, settingsEnvironment({ role: 'contributor' }), [
      { provide: CatalogStore, useValue: store },
    ]);
    expect(checkbox('Tarea').disabled).toBe(true);
    expect(saveButton()).toBeNull();
  });
});
