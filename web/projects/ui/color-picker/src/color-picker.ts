import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  computed,
  forwardRef,
  input,
  model,
  signal,
  viewChildren,
} from '@angular/core';
import { ControlValueAccessor, NG_VALUE_ACCESSOR } from '@angular/forms';
import { InputText } from 'primeng/inputtext';

import {
  HEX_COLOR_ERROR,
  PALETTE_SIZE,
  isHexColor,
  nextPaletteIndex,
  paletteIndexOf,
  paletteName,
  paletteValue,
} from './color-value';

let nextColorPickerId = 0;

/**
 * Color picker over the palette tokens, as a keyboard-navigable radio group, plus an optional
 * hex input for custom colors. Works with `[(value)]` and with reactive forms.
 */
@Component({
  selector: 'app-color-picker',
  imports: [InputText],
  templateUrl: './color-picker.html',
  styleUrl: './color-picker.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => ColorPicker),
      multi: true,
    },
  ],
})
export class ColorPicker implements ControlValueAccessor {
  /** Accessible name of the group, for example "Color de la etiqueta". */
  readonly label = input.required<string>();
  /** 'palette-N' for a palette color, '#RRGGBB' for a custom color, null when there is none. */
  readonly value = model<string | null>(null);
  /** Shows the hex field for custom colors. */
  readonly allowCustom = input(true);

  protected readonly paletteIndexes = Array.from({ length: PALETTE_SIZE }, (_, i) => i + 1);
  protected readonly helpError = HEX_COLOR_ERROR;
  protected readonly disabled = signal(false);
  protected readonly selected = computed(() => paletteIndexOf(this.value()));
  protected readonly focusable = computed(() => this.selected() ?? 1);
  protected readonly customValue = computed(() => {
    const value = this.value();
    return value && isHexColor(value) ? value : null;
  });

  protected readonly draft = signal(this.initialDraft());
  protected readonly invalid = computed(() => this.draft() !== '' && !isHexColor(this.draft()));
  protected readonly inputId = `corelog-color-${nextColorPickerId++}`;
  protected readonly errorId = `${this.inputId}-error`;

  private readonly swatches = viewChildren<ElementRef<HTMLButtonElement>>('swatch');
  private onChangeFn: (value: string | null) => void = () => {};
  private onTouchedFn: () => void = () => {};

  protected nameOf(index: number): string {
    return paletteName(index);
  }

  protected select(index: number): void {
    this.emit(paletteValue(index));
  }

  protected onSwatchKeyDown(event: KeyboardEvent, index: number): void {
    const next = nextPaletteIndex(event.key, index);
    if (next === null) {
      return;
    }
    event.preventDefault();
    this.emit(paletteValue(next));
    this.swatches()[next - 1]?.nativeElement.focus();
  }

  protected onDraftInput(event: Event): void {
    const raw = (event.target as HTMLInputElement).value;
    this.draft.set(raw);
    const next = raw.trim();
    if (isHexColor(next)) {
      this.emit(next);
    }
  }

  protected onBlur(): void {
    this.onTouchedFn();
  }

  writeValue(value: string | null): void {
    this.value.set(value);
  }

  registerOnChange(fn: (value: string | null) => void): void {
    this.onChangeFn = fn;
  }

  registerOnTouched(fn: () => void): void {
    this.onTouchedFn = fn;
  }

  setDisabledState(isDisabled: boolean): void {
    this.disabled.set(isDisabled);
  }

  private emit(value: string): void {
    this.value.set(value);
    this.onChangeFn(value);
  }

  private initialDraft(): string {
    const value = this.value();
    return value && isHexColor(value) ? value : '';
  }
}
