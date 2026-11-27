import { BreakpointObserver } from '@angular/cdk/layout';
import {
  booleanAttribute,
  ChangeDetectionStrategy,
  Component,
  computed,
  contentChild,
  Directive,
  inject,
  input,
} from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { map } from 'rxjs';

export type PageVariant = 'reading' | 'wide' | 'fluid';

const STACKED_QUERY = '(max-width: 991.98px)';

/** Marks the element that is projected into the side panel of a page container. */
@Directive({
  selector: '[appPageSidebar]',
})
export class PageSidebar {}

/**
 * Wraps the content of a page: max-width per template, gutters, bottom padding and an optional
 * right panel. The shell owns the main landmark, so this renders a div.
 *
 * The side panel is projected through an element with the appPageSidebar attribute. It sits beside
 * the content from 992 px and moves below it on narrower viewports.
 */
@Component({
  selector: 'app-page-container',
  templateUrl: './page-container.html',
  styleUrl: './page-container.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class PageContainer {
  /** reading: forms and detail (max 1200 px). wide: data pages (max 1680 px). fluid: edge to edge. */
  readonly variant = input.required<PageVariant>();
  /** Sets data-ready="true" when true, "false" otherwise. Lets audits and e2e wait for real content. */
  readonly ready = input(false, { transform: booleanAttribute });
  /** Sets data-page for layout audits and e2e selectors. */
  readonly pageName = input<string>();
  /**
   * Fill the viewport below the top bar: the content column grows to at least
   * calc(100dvh - topbar height - 2 gutters), so a child with flex-grow (app-section fill) extends to the bottom.
   */
  readonly fill = input(false, { transform: booleanAttribute });

  private readonly breakpoints = inject(BreakpointObserver);
  private readonly sidebarSlot = contentChild(PageSidebar);

  protected readonly hasSidebar = computed(() => this.sidebarSlot() !== undefined);
  protected readonly stacked = toSignal(
    this.breakpoints.observe(STACKED_QUERY).pipe(map((state) => state.matches)),
    { initialValue: this.breakpoints.isMatched(STACKED_QUERY) },
  );
}
