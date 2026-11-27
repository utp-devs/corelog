import { CdkDrag, CdkDragDrop, CdkDragHandle, CdkDropList } from '@angular/cdk/drag-drop';
import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { CatalogItem, CatalogKind } from '@corelog/core';
import { CatalogIcon } from '@corelog/ui/catalog-icon';
import { EmptyState } from '@corelog/ui/empty-state';
import { AppTooltip } from '@corelog/ui/tooltip';
import { Button } from 'primeng/button';

import { CATALOG_COPY, hierarchyText, protectedReason, usageText } from './catalog-copy';

/**
 * Ordered list of one catalog. Rows are reordered by dragging the handle or with the move up and move
 * down buttons (the keyboard and touch alternative). Without manage rights the list is read-only.
 */
@Component({
  selector: 'app-catalog-table',
  imports: [AppTooltip, Button, CatalogIcon, CdkDrag, CdkDragHandle, CdkDropList, EmptyState],
  templateUrl: './catalog-table.html',
  styleUrl: './catalog-table.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class CatalogTable {
  readonly kind = input.required<CatalogKind>();
  readonly items = input.required<readonly CatalogItem[]>();
  readonly canManage = input(false);
  /** A change is being saved: reordering is paused. */
  readonly busy = input(false);

  /** Ids of the whole catalog in the new order. */
  readonly reordered = output<string[]>();
  readonly editRequested = output<CatalogItem>();
  readonly defaultRequested = output<CatalogItem>();
  readonly deleteRequested = output<CatalogItem>();
  readonly createRequested = output<void>();

  protected readonly copy = computed(() => CATALOG_COPY[this.kind()]);
  protected readonly reorderDisabled = computed(() => !this.canManage() || this.busy());

  protected readonly usageText = usageText;
  protected readonly hierarchyText = hierarchyText;

  protected deleteReason(item: CatalogItem): string | null {
    return protectedReason(this.kind(), item, this.items().length);
  }

  protected drop(event: CdkDragDrop<unknown>): void {
    this.moveTo(event.previousIndex, event.currentIndex);
  }

  protected move(index: number, delta: -1 | 1): void {
    this.moveTo(index, index + delta);
  }

  private moveTo(from: number, to: number): void {
    const ids = this.items().map((item) => item.id);
    if (from === to || to < 0 || to >= ids.length) return;
    const [moved] = ids.splice(from, 1);
    ids.splice(to, 0, moved);
    this.reordered.emit(ids);
  }
}
