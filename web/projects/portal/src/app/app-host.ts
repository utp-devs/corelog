import { ChangeDetectionStrategy, Component } from '@angular/core';
import { AppRoot } from '@corelog/ui/app-root';

import { RouteLoading } from './route-loading';

/** Application host: the shared root (toasts, router outlet) plus the route loading indicator. */
@Component({
  selector: 'app-host',
  imports: [AppRoot, RouteLoading],
  templateUrl: './app-host.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class AppHost {}
