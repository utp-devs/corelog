import { ChangeDetectionStrategy, Component } from '@angular/core';
import { RouterLink } from '@angular/router';
import { ErrorPage } from '@corelog/ui/error-page';

/**
 * Not-found page of the portal: the shared 404 page under a branded header, so a visitor who mistyped
 * an address still sees the product and a way home.
 */
@Component({
  selector: 'app-portal-not-found',
  imports: [ErrorPage, RouterLink],
  templateUrl: './portal-not-found.html',
  styleUrl: './portal-not-found.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class PortalNotFound {}
