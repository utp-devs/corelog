import { TestBed } from '@angular/core/testing';
import { ReleaseVersion, VersionApiService } from '@corelog/core';
import { of } from 'rxjs';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { buttonNamed, mountSettingsPage } from '../settings-spec-helpers';
import { LEAD_PERMISSIONS, PROJECT_ID, settingsEnvironment } from '../settings-test-kit';
import { ProjectVersionsPage } from './project-versions-page';

function version(overrides: Partial<ReleaseVersion> = {}): ReleaseVersion {
  return {
    id: 'v1',
    projectId: PROJECT_ID,
    name: '1.0',
    description: '',
    startDate: null,
    releaseDate: null,
    status: 'unreleased',
    releasedAt: null,
    progress: { total: 4, done: 1, inProgress: 1, todo: 2, pointsTotal: 0, pointsDone: 0 },
    ...overrides,
  };
}

function api(versions: ReleaseVersion[]) {
  return {
    list: vi.fn(() => of(versions)),
    create: vi.fn(() => of(version({ id: 'v9', name: 'nueva' }))),
    update: vi.fn(() => of(version())),
    delete: vi.fn(() => of(undefined)),
    archive: vi.fn(() => of(version({ status: 'archived' }))),
    notes: vi.fn(),
    release: vi.fn(() =>
      of({
        version: version({ status: 'released' }),
        movedIssues: 3,
        unresolvedIssues: 0,
        nextVersion: null,
      }),
    ),
  };
}

const MANAGER = { permissions: [...LEAD_PERMISSIONS, 'manage_versions'] };

describe('ProjectVersionsPage', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('lists the versions with their progress', async () => {
    const stub = api([version(), version({ id: 'v2', name: '0.9', status: 'released' })]);
    await mountSettingsPage(ProjectVersionsPage, settingsEnvironment(MANAGER), [
      { provide: VersionApiService, useValue: stub },
    ]);
    const items = document.querySelectorAll('[data-testid="version-item"]');
    expect(items).toHaveLength(2);
    expect(items[0].textContent).toContain('1 de 4 incidencias terminadas');
    expect(items[0].textContent).toContain('Sin publicar');
    expect(items[1].textContent).toContain('Publicada');
  });

  it('releases moving the unfinished issues to a new version', async () => {
    const stub = api([version()]);
    const fixture = await mountSettingsPage(ProjectVersionsPage, settingsEnvironment(MANAGER), [
      { provide: VersionApiService, useValue: stub },
    ]);
    buttonNamed('Publicar')?.click();
    fixture.detectChanges();
    // Three issues are still open: the dialog asks what to do with them.
    expect(document.body.textContent).toContain('Quedan 3 incidencias sin terminar');
    document.querySelector<HTMLInputElement>('[data-testid="move-new"]')?.click();
    fixture.detectChanges();
    document.querySelector<HTMLButtonElement>('[data-testid="release-confirm"] button')?.click();
    await fixture.whenStable();
    expect(stub.release).toHaveBeenCalledWith('v1', 'new', expect.any(String));
  });

  it('hides the management actions without the permission', async () => {
    const stub = api([version()]);
    await mountSettingsPage(
      ProjectVersionsPage,
      settingsEnvironment({ permissions: ['browse_project'] }),
      [{ provide: VersionApiService, useValue: stub }],
    );
    expect(document.querySelector('[data-testid="version-new"]')).toBeNull();
    expect(document.querySelector('[data-testid="version-release"]')).toBeNull();
    expect(buttonNamed('Notas de la versión')).toBeDefined();
  });
});
