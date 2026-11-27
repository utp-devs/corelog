import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import {
  NavigationCancel,
  NavigationEnd,
  NavigationError,
  NavigationStart,
  Router,
} from '@angular/router';
import { ProgressSpinner } from 'primeng/progressspinner';
import { filter, map } from 'rxjs';

/** Centered indicator shown while the router loads a lazy route or waits for a guard. */
@Component({
  selector: 'app-route-loading',
  imports: [ProgressSpinner],
  templateUrl: './route-loading.html',
  styleUrl: './route-loading.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class RouteLoading {
  private readonly router = inject(Router);

  protected readonly loading = toSignal(
    this.router.events.pipe(
      filter(
        (event) =>
          event instanceof NavigationStart ||
          event instanceof NavigationEnd ||
          event instanceof NavigationCancel ||
          event instanceof NavigationError,
      ),
      map((event) => event instanceof NavigationStart),
    ),
    { initialValue: false },
  );
}
