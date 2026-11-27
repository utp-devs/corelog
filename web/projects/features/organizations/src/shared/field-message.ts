import { AbstractControl, ValidatorFn } from '@angular/forms';
import { SERVER_ERROR_KEY } from '@corelog/core';

/** A validator that fails with `message` when `valid(value)` is false. */
export function ruleOf(valid: (value: unknown) => boolean, message: string): ValidatorFn {
  return (control) => (valid(control.value) ? null : { message });
}

/** Message to show under a control: the server message when present, otherwise the failed rule message. */
export function fieldMessage(control: AbstractControl): string | null {
  const errors = control.errors;
  if (!errors) return null;
  const server: unknown = errors[SERVER_ERROR_KEY];
  if (typeof server === 'string') return server;
  const message: unknown = errors['message'];
  return typeof message === 'string' ? message : null;
}

/** True for the values a required field treats as empty. */
export function isEmptyValue(value: unknown): boolean {
  return value === null || value === undefined || value === '';
}
