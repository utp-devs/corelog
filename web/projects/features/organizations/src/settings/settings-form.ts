import { FormControl, FormGroup } from '@angular/forms';
import {
  Organization,
  OrganizationPatch,
  OrgDefaultVisibility,
  OrgWhoCan,
  SERVER_ERROR_KEY,
  asApiError,
} from '@corelog/core';

import { isEmptyValue, ruleOf } from '../shared/field-message';

export interface SettingsFormValues {
  readonly name: string;
  readonly description: string;
  readonly whoCanCreateProjects: OrgWhoCan;
  readonly whoCanCreateTeams: OrgWhoCan;
  readonly defaultProjectVisibility: OrgDefaultVisibility;
  readonly hoursPerDay: number;
  readonly daysPerWeek: number;
  readonly timezone: string;
}

export type SettingsField = keyof SettingsFormValues;
export type SettingsFieldErrors = Partial<Record<SettingsField, string>>;

const FORM_FIELDS: readonly SettingsField[] = [
  'name',
  'description',
  'whoCanCreateProjects',
  'whoCanCreateTeams',
  'defaultProjectVisibility',
  'hoursPerDay',
  'daysPerWeek',
  'timezone',
];

/** Reactive form of the organization settings. Rules and messages mirror the API contract. */
export function createSettingsForm(values: SettingsFormValues) {
  return new FormGroup({
    name: new FormControl(values.name, {
      nonNullable: true,
      validators: [
        ruleOf((v) => typeof v === 'string' && v.trim().length > 0, 'El nombre es obligatorio'),
        ruleOf((v) => isEmptyValue(v) || String(v).length <= 120, 'Máximo 120 caracteres'),
      ],
    }),
    description: new FormControl(values.description, {
      nonNullable: true,
      validators: [
        ruleOf((v) => isEmptyValue(v) || String(v).length <= 500, 'Máximo 500 caracteres'),
      ],
    }),
    whoCanCreateProjects: new FormControl<OrgWhoCan>(values.whoCanCreateProjects, {
      nonNullable: true,
    }),
    whoCanCreateTeams: new FormControl<OrgWhoCan>(values.whoCanCreateTeams, { nonNullable: true }),
    defaultProjectVisibility: new FormControl<OrgDefaultVisibility>(
      values.defaultProjectVisibility,
      {
        nonNullable: true,
      },
    ),
    hoursPerDay: new FormControl(values.hoursPerDay, {
      nonNullable: true,
      validators: [
        ruleOf((v) => !isEmptyValue(v), 'Introduce un número de horas'),
        ruleOf((v) => isEmptyValue(v) || Number(v) >= 1, 'Mínimo 1 hora'),
        ruleOf((v) => isEmptyValue(v) || Number(v) <= 24, 'Máximo 24 horas'),
      ],
    }),
    daysPerWeek: new FormControl(values.daysPerWeek, {
      nonNullable: true,
      validators: [
        ruleOf((v) => !isEmptyValue(v), 'Introduce un número de días'),
        ruleOf((v) => isEmptyValue(v) || Number.isInteger(Number(v)), 'Introduce un número entero'),
        ruleOf((v) => isEmptyValue(v) || Number(v) >= 1, 'Mínimo 1 día'),
        ruleOf((v) => isEmptyValue(v) || Number(v) <= 7, 'Máximo 7 días'),
      ],
    }),
    timezone: new FormControl(values.timezone, {
      nonNullable: true,
      validators: [ruleOf((v) => !isEmptyValue(v), 'Elige una zona horaria')],
    }),
  });
}

export type SettingsForm = ReturnType<typeof createSettingsForm>;

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
  };
}

/** Editable fields only: the 2FA flag is informational until two-factor authentication exists and is never sent. */
export function settingsPatchOf(values: SettingsFormValues): OrganizationPatch {
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
  };
}

function toCamelCase(key: string): string {
  return key.replace(/_([a-z0-9])/g, (_match, char: string) => char.toUpperCase());
}

/**
 * Server field errors mapped to form fields. Accepts `field`, `settings.field` and snake_case names
 * (for example `settings.hours_per_day` becomes `hoursPerDay`). Unknown fields are dropped.
 */
export function settingsFieldErrors(error: unknown): SettingsFieldErrors {
  const result: SettingsFieldErrors = {};
  for (const detail of asApiError(error).details) {
    const key = toCamelCase(detail.field.replace(/^settings\./, ''));
    const field = FORM_FIELDS.find((candidate) => candidate === key);
    if (field && result[field] === undefined) result[field] = detail.message;
  }
  return result;
}

/** Marks each mapped control with the server message (SERVER_ERROR_KEY). */
export function applySettingsServerErrors(form: SettingsForm, errors: SettingsFieldErrors): void {
  for (const field of FORM_FIELDS) {
    const message = errors[field];
    if (message === undefined) continue;
    const control = form.controls[field];
    control.setErrors({ ...control.errors, [SERVER_ERROR_KEY]: message });
    control.markAsTouched();
  }
}
