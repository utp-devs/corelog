import { ChangeDetectionStrategy, Component } from '@angular/core';
import { ErrorPage } from '@corelog/ui/error-page';

/** Shown for unknown paths inside an app: the shared 404 page. */
@Component({
  selector: 'app-not-found',
  imports: [ErrorPage],
  templateUrl: './not-found.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class NotFound {}
