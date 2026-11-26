import { AbstractControl, ValidationErrors, ValidatorFn } from '@angular/forms';

/** Rejects values that are empty or only whitespace. */
export const noBlankValidator: ValidatorFn = (
  control: AbstractControl,
): ValidationErrors | null => {
  const value: unknown = control.value;
  return typeof value === 'string' && value.trim().length === 0 ? { blank: true } : null;
};
