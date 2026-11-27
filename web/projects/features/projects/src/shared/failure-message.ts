import { asApiError } from '@corelog/core';

/** Message of a failed request, or the fallback when the failure carries no text. */
export function failureMessage(error: unknown, fallback: string): string {
  return asApiError(error).message || fallback;
}

/** Value of a text field from its input event. */
export function inputValue(event: Event): string {
  return (event.target as HTMLInputElement | HTMLTextAreaElement).value;
}
