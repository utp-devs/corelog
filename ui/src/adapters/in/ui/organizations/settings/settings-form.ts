import { z } from 'zod'

import type { OrganizationUpdate } from '@/core/organizations/application/ports/organization-admin-repository.port'
import type { Organization } from '@/core/organizations/domain/organization'
import { fieldErrors } from '@/core/shared/app-error'

export const settingsSchema = z.object({
  name: z.string().trim().min(1, 'El nombre es obligatorio').max(120, 'Máximo 120 caracteres'),
  description: z.string().max(500, 'Máximo 500 caracteres'),
  whoCanCreateProjects: z.enum(['members', 'admins']),
  whoCanCreateTeams: z.enum(['members', 'admins']),
  defaultProjectVisibility: z.enum(['open', 'private']),
  hoursPerDay: z
    .number({ error: 'Introduce un número de horas' })
    .min(1, 'Mínimo 1 hora')
    .max(24, 'Máximo 24 horas'),
  daysPerWeek: z
    .number({ error: 'Introduce un número de días' })
    .int('Introduce un número entero')
    .min(1, 'Mínimo 1 día')
    .max(7, 'Máximo 7 días'),
  timezone: z.string().min(1, 'Elige una zona horaria'),
})

export type SettingsFormValues = z.infer<typeof settingsSchema>
export type SettingsFormField = keyof SettingsFormValues

export function settingsValuesOf(org: Organization): SettingsFormValues {
  return {
    name: org.name,
    description: org.description,
    whoCanCreateProjects: org.settings.whoCanCreateProjects,
    whoCanCreateTeams: org.settings.whoCanCreateTeams,
    defaultProjectVisibility: org.settings.defaultProjectVisibility,
    hoursPerDay: org.settings.hoursPerDay,
    daysPerWeek: org.settings.daysPerWeek,
    timezone: org.settings.timezone,
  }
}

/** Editable fields only: the 2FA flag is informational until P12 and is never sent. */
export function settingsPatchOf(values: SettingsFormValues): OrganizationUpdate {
  return {
    name: values.name.trim(),
    description: values.description,
    settings: {
      whoCanCreateProjects: values.whoCanCreateProjects,
      whoCanCreateTeams: values.whoCanCreateTeams,
      defaultProjectVisibility: values.defaultProjectVisibility,
      hoursPerDay: values.hoursPerDay,
      daysPerWeek: values.daysPerWeek,
      timezone: values.timezone,
    },
  }
}

const FORM_FIELDS: readonly SettingsFormField[] = [
  'name',
  'description',
  'whoCanCreateProjects',
  'whoCanCreateTeams',
  'defaultProjectVisibility',
  'hoursPerDay',
  'daysPerWeek',
  'timezone',
]

function toCamelCase(key: string): string {
  return key.replace(/_([a-z0-9])/g, (_match, char: string) => char.toUpperCase())
}

/**
 * Server field errors mapped to form fields. Accepts `field`, `settings.field` and snake_case
 * names (for example `settings.hours_per_day` becomes `hoursPerDay`). Unknown fields are dropped.
 */
export function settingsFieldErrors(error: unknown): Partial<Record<SettingsFormField, string>> {
  const result: Partial<Record<SettingsFormField, string>> = {}
  for (const [rawKey, message] of Object.entries(fieldErrors(error))) {
    const key = toCamelCase(rawKey.replace(/^settings\./, ''))
    const field = FORM_FIELDS.find((candidate) => candidate === key)
    if (field && result[field] === undefined) result[field] = message
  }
  return result
}
