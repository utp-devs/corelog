import type { DestroyRef } from '@angular/core';
import type { AbstractControl, ValidationErrors, ValidatorFn } from '@angular/forms';

/** Letters of any alphabet, marks, spaces, hyphens and apostrophes. */
const NAME_PATTERN = /^[\p{L}\p{M}][\p{L}\p{M}\s'’-]*$/u;

/** Trims and collapses inner whitespace; the value that is validated and sent. */
export function normalizeName(value: string): string {
  return value.replace(/\s+/g, ' ').trim();
}

/**
 * Validates a person name field: required, at most `max` characters once normalized, only letters with
 * accents, spaces, hyphens and apostrophes. Keys: `required`, `maxlength`, `pattern`.
 */
export function personNameValidator(max: number): ValidatorFn {
  return (control: AbstractControl): ValidationErrors | null => {
    const value = normalizeName(String(control.value ?? ''));
    if (!value) return { required: true };
    if (value.length > max)
      return { maxlength: { requiredLength: max, actualLength: value.length } };
    return NAME_PATTERN.test(value) ? null : { pattern: true };
  };
}

export const FIRST_NAME_MAX = 80;
export const LAST_NAME_MAX = 120;

/**
 * Validator of the confirmation control: it must equal the sibling password control. Error key
 * `mismatch`. An empty confirmation is left to `required`. Pair it with `revalidateOnPasswordChange` so a
 * later edit of the password re-evaluates the confirmation.
 */
export function passwordMatchValidator(passwordKey: string): ValidatorFn {
  return (confirmation: AbstractControl): ValidationErrors | null => {
    const password = confirmation.parent?.get(passwordKey);
    if (!password || !confirmation.value) return null;
    return password.value === confirmation.value ? null : { mismatch: true };
  };
}

/** Re-runs the confirmation validators whenever the password changes. The subscription ends with the form. */
export function revalidateOnPasswordChange(
  group: AbstractControl,
  passwordKey: string,
  confirmationKey: string,
  destroyRef: DestroyRef,
): void {
  const password = group.get(passwordKey);
  const confirmation = group.get(confirmationKey);
  if (!password || !confirmation) return;
  const subscription = password.valueChanges.subscribe(() => confirmation.updateValueAndValidity());
  destroyRef.onDestroy(() => subscription.unsubscribe());
}
