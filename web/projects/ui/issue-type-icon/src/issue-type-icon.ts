import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { IssueTypeRef } from '@corelog/core';
import { CatalogIcon } from '@corelog/ui/catalog-icon';

/** Icon of an issue type in the color of its catalog entry. The type name stays available to screen readers. */
@Component({
  selector: 'app-issue-type-icon',
  imports: [CatalogIcon],
  templateUrl: './issue-type-icon.html',
  styleUrl: './issue-type-icon.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class IssueTypeIcon {
  readonly type = input.required<Pick<IssueTypeRef, 'name' | 'icon' | 'color'>>();
  /** Shows the type name next to the icon. */
  readonly showLabel = input(false);
}
