import {
  ChangeDetectionStrategy,
  Component,
  computed,
  contentChild,
  Directive,
  input,
} from '@angular/core';

/** Marks the element projected into the actions area of a page header (buttons, links). */
@Directive({
  selector: '[appPageHeaderActions]',
})
export class PageHeaderActions {}

/** Section banner photograph: the file name under /images/banners without the `banner-` prefix and extension. */
export type PageBanner =
  | 'projects'
  | 'teams'
  | 'issues'
  | 'board'
  | 'backlog'
  | 'calendar'
  | 'reports'
  | 'settings'
  | 'profile';

/**
 * Title block of a page: heading and optional subtitle, with an optional actions area on the right.
 * Actions are projected through an element with the appPageHeaderActions attribute. With `banner` the header
 * becomes a photograph strip with white text over a dark gradient.
 */
@Component({
  selector: 'app-page-header',
  templateUrl: './page-header.html',
  styleUrl: './page-header.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class PageHeader {
  readonly title = input.required<string>();
  readonly subtitle = input<string>();
  /** Photograph strip behind the title. */
  readonly banner = input<PageBanner>();

  private readonly actionsSlot = contentChild(PageHeaderActions);

  protected readonly bannerSrc = computed(() => {
    const name = this.banner();
    return name ? `/images/banners/banner-${name}.webp` : null;
  });

  protected readonly hasActions = computed(() => this.actionsSlot() !== undefined);
}
