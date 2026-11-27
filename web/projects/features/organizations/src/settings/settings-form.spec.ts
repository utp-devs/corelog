import { ApiError } from '@corelog/core';
import { describe, expect, it } from 'vitest';

import { orgOf } from '../testing/org-test-kit';
import {
  createSettingsForm,
  settingsFieldErrors,
  settingsPatchOf,
  settingsValuesOf,
} from './settings-form';

describe('settings form', () => {
  it('maps the organization to form values and back to an editable patch without the 2FA flag', () => {
    const values = settingsValuesOf(orgOf('owner'));
    expect(values.hoursPerDay).toBe(8);
    const patch = settingsPatchOf({ ...values, name: '  Acme Labs ' });
    expect(patch.name).toBe('Acme Labs');
    expect(patch.settings).toEqual({
      whoCanCreateProjects: 'members',
      whoCanCreateTeams: 'admins',
      defaultProjectVisibility: 'open',
      hoursPerDay: 8,
      daysPerWeek: 5,
      timezone: 'Europe/Madrid',
    });
    expect(patch.settings).not.toHaveProperty('require2fa');
  });

  it('rejects hours outside 1 to 24, non-integer days and a blank name', () => {
    const base = settingsValuesOf(orgOf('owner'));
    const form = createSettingsForm(base);

    form.controls.hoursPerDay.setValue(25);
    expect(form.controls.hoursPerDay.valid).toBe(false);
    form.controls.hoursPerDay.setValue(0.5);
    expect(form.controls.hoursPerDay.valid).toBe(false);
    form.controls.hoursPerDay.setValue(7.5);
    expect(form.controls.hoursPerDay.valid).toBe(true);

    form.controls.daysPerWeek.setValue(2.5);
    expect(form.controls.daysPerWeek.valid).toBe(false);

    form.controls.name.setValue('   ');
    expect(form.controls.name.valid).toBe(false);
    expect(base.name).toBe('Acme');
  });

  it('maps nested and snake_case server details to form fields', () => {
    const error: ApiError = {
      status: 422,
      code: 'org.settings_invalid',
      message: 'Revisa la configuración',
      details: [
        { field: 'settings.hours_per_day', message: 'Debe estar entre 1 y 24' },
        { field: 'settings.who_can_create_teams', message: 'Valor no válido' },
        { field: 'unknown_field', message: 'ignorado' },
      ],
    };
    expect(settingsFieldErrors(error)).toEqual({
      hoursPerDay: 'Debe estar entre 1 y 24',
      whoCanCreateTeams: 'Valor no válido',
    });
    expect(settingsFieldErrors(new Error('plain'))).toEqual({});
  });
});
