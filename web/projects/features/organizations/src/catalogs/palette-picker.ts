import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  computed,
  input,
  model,
  viewChildren,
} from '@angular/core';
import { CATALOG_COLORS, CATALOG_ICONS } from '@corelog/core';
import { CatalogIcon } from '@corelog/ui/catalog-icon';

export type PaletteKind = 'icon' | 'color';

const COLOR_NAMES: Readonly<Record<string, string>> = {
  gray: 'Gris',
  red: 'Rojo',
  orange: 'Naranja',
  amber: 'Ámbar',
  green: 'Verde',
  teal: 'Turquesa',
  blue: 'Azul',
  indigo: 'Índigo',
  purple: 'Morado',
  pink: 'Rosa',
};

const ICON_NAMES: Readonly<Record<string, string>> = {
  task: 'Tarea',
  bug: 'Error',
  story: 'Historia',
  epic: 'Épica',
  subtask: 'Subtarea',
  star: 'Estrella',
  flag: 'Bandera',
  bolt: 'Rayo',
  alert: 'Alerta',
  check: 'Visto bueno',
  layers: 'Capas',
  tag: 'Etiqueta',
  book: 'Libro',
  rocket: 'Cohete',
  shield: 'Escudo',
  clock: 'Reloj',
  circle: 'Círculo',
  'arrow-up': 'Flecha hacia arriba',
  'arrow-down': 'Flecha hacia abajo',
  minus: 'Guion',
  'chevrons-up': 'Doble flecha hacia arriba',
  'chevrons-down': 'Doble flecha hacia abajo',
};

interface PaletteOption {
  readonly key: string;
  readonly name: string;
}

/**
 * Single choice over the icon or the color palette of the catalogs, as a radio group: one stop in the Tab
 * order, arrow keys move the choice. Icons are drawn in the chosen `tint`.
 */
@Component({
  selector: 'app-palette-picker',
  imports: [CatalogIcon],
  templateUrl: './palette-picker.html',
  styleUrl: './palette-picker.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class PalettePicker {
  readonly kind = input.required<PaletteKind>();
  /** Accessible name of the group ("Icono", "Color"). */
  readonly label = input.required<string>();
  /** Color key used to draw the icons of an icon picker. */
  readonly tint = input<string | null>(null);
  readonly value = model<string | null>(null);

  private readonly buttons = viewChildren<ElementRef<HTMLButtonElement>>('option');

  protected readonly options = computed<readonly PaletteOption[]>(() =>
    this.kind() === 'icon'
      ? CATALOG_ICONS.map((key) => ({ key, name: ICON_NAMES[key] ?? key }))
      : CATALOG_COLORS.map((key) => ({ key, name: COLOR_NAMES[key] ?? key })),
  );
  /** The option that takes the Tab stop: the selected one, else the first. */
  protected readonly stop = computed(() => {
    const index = this.options().findIndex((option) => option.key === this.value());
    return index < 0 ? 0 : index;
  });

  protected choose(key: string): void {
    this.value.set(key);
  }

  protected onKeydown(event: KeyboardEvent, index: number): void {
    const count = this.options().length;
    let next: number;
    switch (event.key) {
      case 'ArrowRight':
      case 'ArrowDown':
        next = (index + 1) % count;
        break;
      case 'ArrowLeft':
      case 'ArrowUp':
        next = (index - 1 + count) % count;
        break;
      case 'Home':
        next = 0;
        break;
      case 'End':
        next = count - 1;
        break;
      default:
        return;
    }
    event.preventDefault();
    this.value.set(this.options()[next].key);
    this.buttons()[next]?.nativeElement.focus();
  }
}
