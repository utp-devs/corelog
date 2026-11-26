import { describe, expect, it } from 'vitest'

import { AppError } from '@/core/shared/app-error'
import { ORG } from '../members/test-fakes'
import { settingsFieldErrors, settingsPatchOf, settingsSchema, settingsValuesOf } from './settings-form'

describe('settings form', () => {
  it('maps the organization to form values and back to an editable patch without the 2FA flag', () => {
    const values = settingsValuesOf(ORG)
    expect(values.hoursPerDay).toBe(8)
    const patch = settingsPatchOf({ ...values, name: '  Acme Labs ' })
    expect(patch.name).toBe('Acme Labs')
    expect(patch.settings).toEqual({
      whoCanCreateProjects: 'members',
      whoCanCreateTeams: 'admins',
      defaultProjectVisibility: 'open',
      hoursPerDay: 8,
      daysPerWeek: 5,
      timezone: 'Europe/Madrid',
    })
    expect(patch.settings).not.toHaveProperty('require2fa')
  })

  it('rejects hours outside 1 to 24 and non-integer days', () => {
    const base = settingsValuesOf(ORG)
    expect(settingsSchema.safeParse({ ...base, hoursPerDay: 25 }).success).toBe(false)
    expect(settingsSchema.safeParse({ ...base, hoursPerDay: 0.5 }).success).toBe(false)
    expect(settingsSchema.safeParse({ ...base, hoursPerDay: 7.5 }).success).toBe(true)
    expect(settingsSchema.safeParse({ ...base, daysPerWeek: 2.5 }).success).toBe(false)
    expect(settingsSchema.safeParse({ ...base, name: '   ' }).success).toBe(false)
  })

  it('maps nested and snake_case server details to form fields', () => {
    const error = new AppError({
      status: 422,
      code: 'org.settings_invalid',
      message: 'Revisa la configuración',
      details: [
        { field: 'settings.hours_per_day', message: 'Debe estar entre 1 y 24' },
        { field: 'settings.who_can_create_teams', message: 'Valor no válido' },
        { field: 'unknown_field', message: 'ignorado' },
      ],
    })
    expect(settingsFieldErrors(error)).toEqual({
      hoursPerDay: 'Debe estar entre 1 y 24',
      whoCanCreateTeams: 'Valor no válido',
    })
    expect(settingsFieldErrors(new Error('plain'))).toEqual({})
  })
})
