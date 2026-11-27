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

/**
 * Title block of a page: heading and optional subtitle, with an optional actions area on the right.
 * Actions are projected through an element with the appPageHeaderActions attribute.
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

  private readonly actionsSlot = contentChild(PageHeaderActions);

  protected readonly hasActions = computed(() => this.actionsSlot() !== undefined);
}
