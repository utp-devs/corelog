import { AbstractControl } from '@angular/forms';

import { ApiError } from '../models';

/** Error key set on a control when the server rejects its value. */
export const SERVER_ERROR_KEY = 'server';

/**
 * Copies field-level details of an ApiError onto the matching controls of a form.
 * Returns true when at least one control was found and marked.
 */
export function applyServerErrors(form: AbstractControl, error: ApiError): boolean {
  let applied = false;
  for (const detail of error.details) {
    const control = form.get(detail.field);
    if (!control) {
      continue;
    }
    control.setErrors({ ...control.errors, [SERVER_ERROR_KEY]: detail.message });
    control.markAsTouched();
    applied = true;
  }
  return applied;
}
