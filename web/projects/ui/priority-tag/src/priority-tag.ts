import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { PriorityRef } from '@corelog/core';
import { CatalogIcon } from '@corelog/ui/catalog-icon';

/** Issue priority: the icon of its catalog entry in the entry color, followed by the name in regular text color. */
@Component({
  selector: 'app-priority-tag',
  imports: [CatalogIcon],
  templateUrl: './priority-tag.html',
  styleUrl: './priority-tag.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class PriorityTag {
  readonly priority = input.required<Pick<PriorityRef, 'name' | 'icon' | 'color'>>();
}
