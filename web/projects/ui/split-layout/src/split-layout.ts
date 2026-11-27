import { BreakpointObserver } from '@angular/cdk/layout';
import { ChangeDetectionStrategy, Component, inject, input } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { map } from 'rxjs';

export type SplitSideWidth = 320 | 360 | 400;

const STACKED_QUERY = '(max-width: 991.98px)';

/**
 * Main content plus a sticky side panel. The main content is projected through an element with the
 * appSplitMain attribute, the side panel through an element with the appSplitSide attribute. The
 * panel stacks below the main content under 992 px.
 */
@Component({
  selector: 'app-split-layout',
  templateUrl: './split-layout.html',
  styleUrl: './split-layout.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class SplitLayout {
  /** Side panel width in px (default 320). */
  readonly sideWidth = input<SplitSideWidth>(320);
  /** Accessible name of the side panel landmark (default "Panel lateral"). */
  readonly sideLabel = input('Panel lateral');

  private readonly breakpoints = inject(BreakpointObserver);

  protected readonly stacked = toSignal(
    this.breakpoints.observe(STACKED_QUERY).pipe(map((state) => state.matches)),
    { initialValue: this.breakpoints.isMatched(STACKED_QUERY) },
  );
}
