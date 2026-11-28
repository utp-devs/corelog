import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import {
  GADGET_CATALOG,
  GADGET_CATEGORY_LABELS,
  GadgetCategory,
  GadgetDefinition,
  GadgetType,
} from '@corelog/core';
import { Dialog } from '@corelog/ui/dialog';
import { Button } from 'primeng/button';

interface CatalogGroup {
  readonly category: GadgetCategory;
  readonly label: string;
  readonly gadgets: readonly GadgetDefinition[];
}

const ORDER: readonly GadgetCategory[] = ['issues', 'charts', 'activity', 'other'];

/** Catalog of the gadgets a dashboard can hold; choosing one asks the owner to configure it. */
@Component({
  selector: 'app-gadget-catalog-dialog',
  imports: [Button, Dialog],
  templateUrl: './gadget-catalog-dialog.html',
  styleUrl: './gadget-catalog-dialog.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class GadgetCatalogDialog {
  readonly open = input(false);

  readonly picked = output<GadgetType>();
  readonly closeRequested = output<void>();

  protected readonly groups = computed<CatalogGroup[]>(() =>
    ORDER.map((category) => ({
      category,
      label: GADGET_CATEGORY_LABELS[category],
      gadgets: GADGET_CATALOG.filter((gadget) => gadget.category === category),
    })).filter((group) => group.gadgets.length > 0),
  );
}
