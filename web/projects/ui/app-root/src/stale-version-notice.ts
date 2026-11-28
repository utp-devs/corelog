import { ChangeDetectionStrategy, Component, inject } from '@angular/core';

import { ChunkRecovery } from '@corelog/core';

/** Notice shown when a lazy screen cannot load because a new version was published. */
@Component({
  selector: 'app-stale-version-notice',
  templateUrl: './stale-version-notice.html',
  styleUrl: './stale-version-notice.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class StaleVersionNotice {
  protected readonly recovery = inject(ChunkRecovery);
}
