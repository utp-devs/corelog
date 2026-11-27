import { DOCUMENT, InjectionToken, Signal, effect, inject } from '@angular/core';
import { CanDeactivateFn, RouterStateSnapshot } from '@angular/router';

import {
  UNSAVED_CANCEL_TEXT,
  UNSAVED_CONFIRM_TEXT,
  UNSAVED_DESCRIPTION,
  UNSAVED_TITLE,
} from './unsaved-changes.copy';

/**
 * Contract for a form that guards its unsaved edits. The component exposes `dirty` as a signal
 * and registers `unsavedChangesGuard` in its route `canDeactivate` list.
 */
export interface UnsavedChangesAware {
  /** True while the form holds edits that have not been saved. */
  readonly dirty: Signal<boolean>;
}

/** Texts of the question asked before leaving a dirty form. */
export interface UnsavedChangesPromptCopy {
  readonly title: string;
  readonly description: string;
  readonly confirmText: string;
  readonly cancelText: string;
}

/** Asks whether to leave; resolves true to leave. The application replaces the default with a dialog. */
export type UnsavedChangesPrompt = (copy: UnsavedChangesPromptCopy) => Promise<boolean>;

export const UNSAVED_CHANGES_PROMPT = new InjectionToken<UnsavedChangesPrompt>(
  'UNSAVED_CHANGES_PROMPT',
  {
    providedIn: 'root',
    factory: () => {
      const win = inject(DOCUMENT).defaultView;
      return (copy) => Promise.resolve(win ? win.confirm(copy.description) : true);
    },
  },
);

function pathOf(state: RouterStateSnapshot): string {
  return state.url.split(/[?#]/)[0];
}

/**
 * Blocks in-app navigation away from a dirty form and asks whether to leave or keep editing.
 * Navigation within the same path (query or fragment changes) is not blocked.
 */
export const unsavedChangesGuard: CanDeactivateFn<UnsavedChangesAware> = (
  component,
  _route,
  currentState,
  nextState,
) => {
  if (!component.dirty()) return true;
  if (nextState && pathOf(nextState) === pathOf(currentState)) return true;
  return inject(UNSAVED_CHANGES_PROMPT)({
    title: UNSAVED_TITLE,
    description: UNSAVED_DESCRIPTION,
    confirmText: UNSAVED_CONFIRM_TEXT,
    cancelText: UNSAVED_CANCEL_TEXT,
  });
};

/**
 * Asks the browser to warn before a reload or close while `dirty()` is true.
 * Call it in an injection context (a constructor or a field initializer).
 */
export function warnBeforeUnload(dirty: Signal<boolean>): void {
  const win = inject(DOCUMENT).defaultView;
  effect((onCleanup) => {
    if (!win || !dirty()) return;
    const warn = (event: BeforeUnloadEvent) => {
      event.preventDefault();
      event.returnValue = '';
    };
    win.addEventListener('beforeunload', warn);
    onCleanup(() => win.removeEventListener('beforeunload', warn));
  });
}
