import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

import { CATALOG_ICON_FALLBACK, CATALOG_ICON_PATHS } from './catalog-icon-paths';

/**
 * Glyph of a catalog palette key, drawn in the palette color. Decorative: the name of the item it
 * belongs to is always rendered next to it (or as a visually hidden label), so the glyph is hidden
 * from assistive technology.
 */
@Component({
  selector: 'app-catalog-icon',
  templateUrl: './catalog-icon.html',
  styleUrl: './catalog-icon.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: { '[attr.data-color]': 'color()', 'aria-hidden': 'true' },
})
export class CatalogIcon {
  /** Icon key of the palette (task, bug, arrow-up, ...). Unknown keys draw a circle. */
  readonly icon = input<string | null | undefined>();
  /** Color key of the palette (blue, red, ...). Unknown or empty keys use the neutral gray. */
  readonly color = input<string | null | undefined>();

  protected readonly paths = computed(
    () => CATALOG_ICON_PATHS[this.icon() ?? ''] ?? CATALOG_ICON_PATHS[CATALOG_ICON_FALLBACK],
  );
}
