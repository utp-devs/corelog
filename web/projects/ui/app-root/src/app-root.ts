import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { RouterOutlet } from '@angular/router';
import { Toast } from 'primeng/toast';

import { ThemeService } from '@corelog/core';

/** Root component of every app: toast outlet and router outlet. Applies the saved theme on start. */
@Component({
  selector: 'app-root',
  imports: [RouterOutlet, Toast],
  template: `
    <p-toast position="top-right" />
    <router-outlet />
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class AppRoot {
  constructor() {
    inject(ThemeService);
  }
}
