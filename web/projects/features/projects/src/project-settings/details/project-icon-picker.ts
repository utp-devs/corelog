import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  forwardRef,
  signal,
  viewChildren,
} from '@angular/core';
import { ControlValueAccessor, NG_VALUE_ACCESSOR } from '@angular/forms';

export interface ProjectIconOption {
  /** PrimeIcons class stored in the project; empty means "show the key initials". */
  readonly value: string;
  readonly label: string;
}

export const PROJECT_ICON_OPTIONS: readonly ProjectIconOption[] = [
  { value: '', label: 'Sin icono (iniciales de la clave)' },
  { value: 'pi pi-briefcase', label: 'Maletín' },
  { value: 'pi pi-compass', label: 'Brújula' },
  { value: 'pi pi-star', label: 'Estrella' },
  { value: 'pi pi-bolt', label: 'Rayo' },
  { value: 'pi pi-heart', label: 'Corazón' },
  { value: 'pi pi-globe', label: 'Mundo' },
  { value: 'pi pi-shield', label: 'Escudo' },
  { value: 'pi pi-code', label: 'Código' },
  { value: 'pi pi-chart-bar', label: 'Gráfico' },
  { value: 'pi pi-cog', label: 'Engranaje' },
  { value: 'pi pi-flag', label: 'Bandera' },
  { value: 'pi pi-bookmark', label: 'Marcador' },
  { value: 'pi pi-cloud', label: 'Nube' },
  { value: 'pi pi-database', label: 'Base de datos' },
  { value: 'pi pi-mobile', label: 'Móvil' },
  { value: 'pi pi-shopping-cart', label: 'Carrito' },
];

/** Icon of a project as a keyboard-navigable radio group (arrows move and choose). */
@Component({
  selector: 'app-project-icon-picker',
  templateUrl: './project-icon-picker.html',
  styleUrl: './project-icon-picker.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  providers: [
    { provide: NG_VALUE_ACCESSOR, useExisting: forwardRef(() => ProjectIconPicker), multi: true },
  ],
})
export class ProjectIconPicker implements ControlValueAccessor {
  protected readonly options = PROJECT_ICON_OPTIONS;
  protected readonly value = signal('');
  protected readonly disabled = signal(false);

  private readonly buttons = viewChildren<ElementRef<HTMLButtonElement>>('option');
  private onChangeFn: (value: string) => void = () => {};
  private onTouchedFn: () => void = () => {};

  writeValue(value: string | null): void {
    this.value.set(value ?? '');
  }

  registerOnChange(fn: (value: string) => void): void {
    this.onChangeFn = fn;
  }

  registerOnTouched(fn: () => void): void {
    this.onTouchedFn = fn;
  }

  setDisabledState(isDisabled: boolean): void {
    this.disabled.set(isDisabled);
  }

  /** The tab stop is the chosen option; an unknown stored value falls back to the first one. */
  protected isTabStop(option: ProjectIconOption): boolean {
    const known = this.options.some((item) => item.value === this.value());
    return known ? option.value === this.value() : option.value === '';
  }

  protected choose(option: ProjectIconOption): void {
    if (this.disabled()) return;
    this.value.set(option.value);
    this.onChangeFn(option.value);
    this.onTouchedFn();
  }

  protected onKeydown(event: KeyboardEvent, index: number): void {
    const last = this.options.length - 1;
    let next: number | null = null;
    switch (event.key) {
      case 'ArrowRight':
      case 'ArrowDown':
        next = index === last ? 0 : index + 1;
        break;
      case 'ArrowLeft':
      case 'ArrowUp':
        next = index === 0 ? last : index - 1;
        break;
      case 'Home':
        next = 0;
        break;
      case 'End':
        next = last;
        break;
    }
    if (next === null) return;
    event.preventDefault();
    this.choose(this.options[next]);
    this.buttons()[next]?.nativeElement.focus();
  }
}
