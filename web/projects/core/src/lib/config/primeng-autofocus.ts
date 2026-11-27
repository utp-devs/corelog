import { AutoFocus } from 'primeng/autofocus';

type AutoFocusHook = (this: AutoFocus) => void;
type PatchedPrototype = { onAfterContentChecked: AutoFocusHook };

let patched = false;

/**
 * PrimeNG sets the native `autofocus` attribute on every component that uses its autofocus directive
 * (buttons among them) unless the input is exactly `false`; an input left undefined counts as "on", and
 * the browser then focuses the first control of a freshly loaded page, leaving a focus ring on it.
 * Treating an unset input as `false` fixes it for every PrimeNG control in one place. Idempotent.
 */
export function normalizePrimeAutofocus(): void {
  if (patched) {
    return;
  }
  patched = true;
  const prototype = AutoFocus.prototype as unknown as PatchedPrototype;
  const original = prototype.onAfterContentChecked;
  prototype.onAfterContentChecked = function (this: AutoFocus): void {
    if (this.autofocus == null) {
      this.autofocus = false;
    }
    original.call(this);
  };
}
