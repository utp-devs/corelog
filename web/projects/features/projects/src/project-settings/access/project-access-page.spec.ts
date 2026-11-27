import { TestBed } from '@angular/core/testing';
import { throwError } from 'rxjs';
import { afterEach, describe, expect, it } from 'vitest';

import { buttonLabelled, buttonNamed, mountSettingsPage } from '../settings-spec-helpers';
import { memberOf, settingsEnvironment, teamOf } from '../settings-test-kit';
import { ProjectAccessPage } from './project-access-page';

const LAST_LEAD = { status: 409, code: 'project.last_lead', message: 'x', details: [] };

type Internals = {
  newUserId: { set(value: string): void };
  newTeamId: { set(value: string): void };
  personRoleControl(row: unknown): { setValue(value: string): void };
  rows(): unknown[];
};

describe('ProjectAccessPage', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('lists the people with their source and effective role', async () => {
    const fixture = await mountSettingsPage(
      ProjectAccessPage,
      settingsEnvironment({
        members: [
          memberOf('u-me', 'lead'),
          memberOf('u-2', 'viewer', { viaTeam: true, teamName: 'Soporte' }),
        ],
      }),
    );
    const text = fixture.nativeElement.textContent as string;
    expect(text).toContain('Ana Pérez');
    expect(text).toContain('(tú)');
    expect(text).toContain('Directo');
    expect(text).toContain('Por equipo: Soporte');
    // A person who reaches the project only through a team has no role select or remove button.
    expect(buttonLabelled('Quitar a Luis Gómez')).toBeNull();
    expect(buttonLabelled('Quitar a Ana Pérez')).not.toBeNull();
  });

  it('adds a person from the organization with the chosen role', async () => {
    const environment = settingsEnvironment();
    const fixture = await mountSettingsPage(ProjectAccessPage, environment);
    (fixture.componentInstance as unknown as Internals).newUserId.set('u-3');
    fixture.detectChanges();
    buttonNamed('Agregar')?.click();
    fixture.detectChanges();
    expect(environment.projectApi['addMember']).toHaveBeenCalledWith('p-1', 'u-3', 'contributor', {
      silent: true,
    });
  });

  it('adds a team with a role', async () => {
    const environment = settingsEnvironment({ teams: [teamOf('t-1', 'Soporte')] });
    const fixture = await mountSettingsPage(ProjectAccessPage, environment);
    (fixture.componentInstance as unknown as Internals).newTeamId.set('t-1');
    fixture.detectChanges();
    const addButtons = Array.from(document.querySelectorAll('button')).filter(
      (button) => button.textContent?.trim() === 'Agregar',
    );
    addButtons[1].click();
    expect(environment.permissionsApi['putTeamRole']).toHaveBeenCalledWith(
      'p-1',
      't-1',
      'contributor',
      { silent: true },
    );
  });

  it('removes a person only after the confirmation', async () => {
    const environment = settingsEnvironment();
    const fixture = await mountSettingsPage(ProjectAccessPage, environment);
    buttonLabelled('Quitar a Luis Gómez')?.click();
    fixture.detectChanges();
    expect(environment.projectApi['removeMember']).not.toHaveBeenCalled();
    expect(document.body.textContent).toContain('¿Quitar a Luis Gómez del proyecto?');
    buttonNamed('Quitar')?.click();
    expect(environment.projectApi['removeMember']).toHaveBeenCalledWith('p-1', 'u-2', {
      silent: true,
    });
  });

  it('explains the last lead rule in Spanish and keeps the previous role', async () => {
    const environment = settingsEnvironment({
      projectApi: { updateMember: () => throwError(() => LAST_LEAD) },
    });
    const fixture = await mountSettingsPage(ProjectAccessPage, environment);
    const page = fixture.componentInstance as unknown as Internals;
    const row = page.rows()[0];
    const control = page.personRoleControl(row);
    control.setValue('viewer');
    fixture.detectChanges();
    expect(document.querySelector('[role="alert"]')?.textContent).toContain('al menos un líder');
    expect((control as unknown as { value: string }).value).toBe('lead');
  });

  it('is read-only without the permission to manage people', async () => {
    const environment = settingsEnvironment({ role: 'contributor' });
    await mountSettingsPage(ProjectAccessPage, environment);
    expect(buttonNamed('Agregar')).toBeUndefined();
    expect(buttonLabelled('Quitar a Luis Gómez')).toBeNull();
    // Team roles are only readable with the permission to manage people.
    expect(environment.permissionsApi['listTeamRoles']).not.toHaveBeenCalled();
    expect(document.body.textContent).not.toContain('Equipos');
  });
});
