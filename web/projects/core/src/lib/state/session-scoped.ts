import { effect, inject, untracked } from '@angular/core';

import { SessionService } from '../services/session.service';

/** Runs `reset` whenever the session ends, so no data of a previous user survives a logout. */
export function resetOnLogout(reset: () => void): void {
  const session = inject(SessionService);
  effect(() => {
    if (!session.isAuthenticated()) {
      untracked(reset);
    }
  });
}
