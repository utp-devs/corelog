import { TestBed } from '@angular/core/testing';
import { afterEach, describe, expect, it } from 'vitest';

import { mountSettingsPage } from '../settings-spec-helpers';
import { SAMPLE_GRANTS, schemeOf, settingsEnvironment } from '../settings-test-kit';
import { ProjectRolesPage } from './project-roles-page';

function card(role: string): string {
  return document.querySelector(`[data-testid="role-${role}"]`)?.textContent ?? '';
}

describe('ProjectRolesPage', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('explains each role from the scheme', async () => {
    await mountSettingsPage(ProjectRolesPage, settingsEnvironment());
    expect(card('lead')).toContain('Editar el proyecto');
    expect(card('viewer')).toContain('Ver el proyecto');
    expect(card('viewer')).not.toContain('Crear incidencias');
    expect(document.body.textContent).toContain('permisos estándar');
  });

  it('reflects a customized scheme', async () => {
    await mountSettingsPage(
      ProjectRolesPage,
      settingsEnvironment({
        scheme: schemeOf({ ...SAMPLE_GRANTS, create_issue: ['lead', 'viewer'] }, false),
      }),
    );
    expect(card('viewer')).toContain('Crear incidencias');
    expect(card('contributor')).not.toContain('Crear incidencias');
    expect(document.body.textContent).toContain('personalizados');
  });
});
