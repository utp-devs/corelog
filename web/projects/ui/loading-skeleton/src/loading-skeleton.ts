import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

export type LoadingSkeletonVariant = 'table' | 'card' | 'detail';

/** Loading placeholder with the shape of the final content. Announces the loading state to screen readers. */
@Component({
  selector: 'app-loading-skeleton',
  templateUrl: './loading-skeleton.html',
  styleUrl: './loading-skeleton.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class LoadingSkeleton {
  /** Shape of the content being loaded. */
  readonly variant = input<LoadingSkeletonVariant>('table');
  /** Number of rows in the table variant. */
  readonly rows = input(5);

  /** Line count of each block: [title?, lines]. Title is drawn above the lines. */
  protected readonly blocks = computed<{ title: boolean; lines: number[] }[]>(() => {
    switch (this.variant()) {
      case 'card':
        return [{ title: true, lines: [0, 1, 2] }];
      case 'detail':
        return [
          { title: true, lines: [0, 1] },
          { title: true, lines: [0, 1, 2, 3] },
        ];
      default:
        return [{ title: false, lines: Array.from({ length: this.rows() }, (_, i) => i) }];
    }
  });
}
