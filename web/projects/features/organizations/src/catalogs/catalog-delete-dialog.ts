import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  input,
  output,
  signal,
  untracked,
} from '@angular/core';
import { FormsModule } from '@angular/forms';
import { CatalogItem, CatalogKind, toCatalogOption } from '@corelog/core';
import { Dialog } from '@corelog/ui/dialog';
import { FieldRow } from '@corelog/ui/field-row';
import { SelectField } from '@corelog/ui/select-field';
import { Button } from 'primeng/button';
import { Message } from 'primeng/message';

import { CATALOG_COPY, usageText } from './catalog-copy';

/**
 * Confirmation to delete a catalog item. When issues use it, the user must pick the item that receives
 * them (for issue types, one of the same hierarchy level), and the dialog does not let the deletion go on
 * without it.
 */
@Component({
  selector: 'app-catalog-delete-dialog',
  imports: [Button, Dialog, FieldRow, FormsModule, Message, SelectField],
  templateUrl: './catalog-delete-dialog.html',
  styleUrl: './catalog-delete-dialog.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class CatalogDeleteDialog {
  readonly open = input(false);
  readonly kind = input.required<CatalogKind>();
  /** Item to delete. */
  readonly item = input<CatalogItem | null>(null);
  /** The whole catalog, to offer the reassignment targets. */
  readonly catalog = input<readonly CatalogItem[]>([]);
  readonly deleting = input(false);
  readonly error = input<string | null>(null);

  /** The user confirmed; `reassignTo` is set only when the item is in use. */
  readonly confirmed = output<string | undefined>();
  readonly closeRequested = output<void>();

  protected readonly target = signal<string | null>(null);

  protected readonly copy = computed(() => CATALOG_COPY[this.kind()]);
  protected readonly inUse = computed(() => (this.item()?.usageCount ?? 0) > 0);
  protected readonly usage = computed(() => usageText(this.item()?.usageCount ?? 0).toLowerCase());
  protected readonly targets = computed(() => {
    const item = this.item();
    if (!item) return [];
    return this.catalog()
      .filter(
        (candidate) =>
          candidate.id !== item.id &&
          (this.kind() !== 'issue-types' || candidate.hierarchyLevel === item.hierarchyLevel),
      )
      .map(toCatalogOption);
  });
  protected readonly canConfirm = computed(() => !this.inUse() || this.target() !== null);

  constructor() {
    effect(() => {
      if (this.open()) untracked(() => this.target.set(null));
    });
  }

  protected confirm(): void {
    if (!this.canConfirm() || this.deleting()) return;
    this.confirmed.emit(this.inUse() ? (this.target() ?? undefined) : undefined);
  }
}
