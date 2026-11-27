import { Signal, computed, inject } from '@angular/core';
import { CurrentProjectStore } from '@corelog/core';

/**
 * UUID of the project in view, for the section routes. The project area resolves the `:key` route
 * parameter (key or UUID) into the current project; the value is empty until the project is read.
 */
export function injectProjectId(): Signal<string> {
  const current = inject(CurrentProjectStore);
  return computed(() => current.projectId() ?? '');
}
