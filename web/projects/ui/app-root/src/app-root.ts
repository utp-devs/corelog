import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { RouterOutlet } from '@angular/router';

import { ThemeService } from '@corelog/core';
import { ConsentGate } from '@corelog/ui/consent-gate';
import { ToastHost } from '@corelog/ui/toast-host';

/** Root component of every app: toast outlet, router outlet and the blocking re-consent modal. Applies the saved theme on start. */
@Component({
  selector: 'app-root',
  imports: [ConsentGate, RouterOutlet, ToastHost],
  templateUrl: './app-root.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class AppRoot {
  constructor() {
    inject(ThemeService);
  }
}
