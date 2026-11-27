import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import {
  NavigationCancel,
  NavigationEnd,
  NavigationError,
  NavigationStart,
  Router,
} from '@angular/router';
import { AppRoot } from '@corelog/ui/app-root';
import { ProgressSpinner } from 'primeng/progressspinner';
import { filter, map } from 'rxjs';

/** Centered indicator shown while the router loads a lazy route or waits for a guard. */
@Component({
  selector: 'app-route-loading',
  imports: [ProgressSpinner],
  template: `
    @if (loading()) {
      <div class="route-loading" role="status" aria-live="polite">
        <p-progressspinner ariaLabel="Cargando" />
      </div>
    }
  `,
  styles: `
    .route-loading {
      position: fixed;
      inset: 0;
      display: flex;
      align-items: center;
      justify-content: center;
      pointer-events: none;
      z-index: 1000;
    }
  `,
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

/** Application host: the shared root (toasts, router outlet) plus the route loading indicator. */
@Component({
  selector: 'app-host',
  imports: [AppRoot, RouteLoading],
  template: `
    <app-root />
    <app-route-loading />
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class AppHost {}
