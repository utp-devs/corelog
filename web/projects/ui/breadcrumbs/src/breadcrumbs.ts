import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { RouterLink } from '@angular/router';

export type BreadcrumbItem = {
  label: string;
  /** Route to link to. The last item is the current page and is never a link. */
  to?: string;
};

/**
 * Route trail. Middle items are hidden below 768 px (CSS) so the first and last stay visible.
 * The last item is marked aria-current="page". Renders nothing for an empty trail.
 */
@Component({
  selector: 'app-breadcrumbs',
  imports: [RouterLink],
  templateUrl: './breadcrumbs.html',
  styleUrl: './breadcrumbs.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class Breadcrumbs {
  readonly items = input.required<BreadcrumbItem[]>();

  protected isMiddle(index: number): boolean {
    return index > 0 && index < this.items().length - 1;
  }
}
