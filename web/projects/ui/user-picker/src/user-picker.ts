import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  DestroyRef,
  computed,
  inject,
  input,
  model,
  output,
  signal,
  viewChild,
} from '@angular/core';
import { Avatar } from '@corelog/ui/avatar';

export interface PickableUser {
  id: string;
  name: string;
  email?: string;
  /** Profile photo address; initials are shown when missing. */
  avatarUrl?: string | null;
}

let nextUserPickerId = 0;

/** Room the list needs below the field before it opens upwards instead. */
const PANEL_SPACE = 300;

/** Fixed position of the open list, taken from the field when it opens. */
interface PanelAnchor {
  readonly left: number;
  readonly width: number;
  readonly top: number | null;
  readonly bottom: number | null;
}

/**
 * User selector: a combobox with search, avatar, name and email in every option, a selected state with
 * the avatar of the chosen person, loading and empty states, a clear button and full keyboard support
 * (arrows move, Enter chooses, Escape closes, Home and End jump). Loading the directory is the parent's job:
 * the `search` output emits the typed text so the parent can query the server; the list shown is also
 * narrowed locally by name or email.
 */
@Component({
  selector: 'app-user-picker',
  imports: [Avatar],
  templateUrl: './user-picker.html',
  styleUrl: './user-picker.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: { '(document:pointerdown)': 'onDocumentPointerDown($event)' },
})
export class UserPicker {
  readonly users = input.required<PickableUser[]>();
  /** Selected user id. */
  readonly value = model<string | undefined>(undefined);
  /** Emits the text typed in the search field. */
  readonly search = output<string>();
  readonly loading = input(false);
  readonly placeholder = input('Buscar persona');
  /** Accessible name of the field when no visible label points at it; defaults to the placeholder. */
  readonly ariaLabel = input<string>();
  readonly allowClear = input(true);
  readonly disabled = input(false);
  /** Why the picker is disabled. Used as its tooltip. */
  readonly disabledReason = input<string>();

  protected readonly id = `corelog-user-picker-${nextUserPickerId++}`;
  protected readonly listboxId = `${this.id}-listbox`;
  protected readonly hostRef = inject<ElementRef<HTMLElement>>(ElementRef);
  protected readonly field = viewChild<ElementRef<HTMLInputElement>>('field');

  /** Text typed by the user while the list is open; null shows the selected name instead. */
  private readonly typed = signal<string | null>(null);
  private readonly destroyRef = inject(DestroyRef);
  /** Where the list is drawn. It is fixed to the viewport so cards with hidden overflow never clip it. */
  protected readonly anchor = signal<PanelAnchor | null>(null);
  private readonly closeOnScroll = (event: Event): void => {
    const target = event.target;
    if (target instanceof Element && target.closest('.user-picker__panel')) {
      return;
    }
    this.close();
  };
  protected readonly open = signal(false);
  protected readonly active = signal(0);

  protected readonly selected = computed<PickableUser | null>(
    () => this.users().find((user) => user.id === this.value()) ?? null,
  );
  protected readonly text = computed(() => this.typed() ?? this.selected()?.name ?? '');
  protected readonly accessibleName = computed(() => this.ariaLabel() ?? this.placeholder());
  protected readonly canClear = computed(
    () => this.allowClear() && !this.disabled() && (this.selected() !== null || this.text() !== ''),
  );

  /** Users that match the typed text (case-insensitive, name or email). */
  protected readonly suggestions = computed<PickableUser[]>(() => {
    const term = (this.typed() ?? '').trim().toLowerCase();
    return this.users().filter(
      (user) =>
        term === '' ||
        user.name.toLowerCase().includes(term) ||
        (user.email ?? '').toLowerCase().includes(term),
    );
  });
  protected readonly showEmpty = computed(
    () => this.open() && !this.loading() && this.suggestions().length === 0,
  );
  protected readonly activeOptionId = computed(() =>
    this.open() && this.suggestions().length > 0 ? this.optionId(this.active()) : null,
  );

  constructor() {
    this.destroyRef.onDestroy(() => {
      window.removeEventListener('scroll', this.closeOnScroll, true);
      window.removeEventListener('resize', this.closeOnScroll);
    });
  }

  protected optionId(index: number): string {
    return `${this.id}-option-${index}`;
  }

  protected onInput(event: Event): void {
    const term = (event.target as HTMLInputElement).value;
    this.typed.set(term);
    this.active.set(0);
    this.openList(term);
  }

  protected onFocus(): void {
    this.field()?.nativeElement.select();
  }

  /** A click on the field opens the list, so the options are one click away. */
  protected onClick(): void {
    if (!this.open() && !this.disabled()) {
      this.openList(this.typed() ?? '');
    }
  }

  protected toggle(): void {
    if (this.disabled()) {
      return;
    }
    if (this.open()) {
      this.close();
    } else {
      this.field()?.nativeElement.focus();
      this.openList(this.typed() ?? '');
    }
  }

  protected clear(): void {
    this.typed.set(null);
    this.value.set(undefined);
    this.close();
    this.field()?.nativeElement.focus();
  }

  protected choose(user: PickableUser): void {
    this.value.set(user.id);
    this.close();
  }

  protected onKeyDown(event: KeyboardEvent): void {
    const count = this.suggestions().length;
    switch (event.key) {
      case 'ArrowDown':
      case 'ArrowUp': {
        event.preventDefault();
        if (!this.open()) {
          this.openList(this.typed() ?? '');
          return;
        }
        if (count > 0) {
          const step = event.key === 'ArrowDown' ? 1 : -1;
          this.active.set((this.active() + step + count) % count);
        }
        return;
      }
      case 'Home':
      case 'End':
        if (this.open() && count > 0) {
          event.preventDefault();
          this.active.set(event.key === 'Home' ? 0 : count - 1);
        }
        return;
      case 'Enter': {
        if (this.open() && count > 0) {
          event.preventDefault();
          this.choose(this.suggestions()[Math.min(this.active(), count - 1)]);
        }
        return;
      }
      case 'Escape':
        if (this.open()) {
          // A dialog or drawer around the picker must stay open: only the list closes.
          event.preventDefault();
          event.stopPropagation();
          this.close();
        }
        return;
      case 'Tab':
        this.close();
        return;
      default:
        return;
    }
  }

  protected onDocumentPointerDown(event: Event): void {
    const target = event.target as Element | null;
    const inside =
      this.hostRef.nativeElement.contains(target) || !!target?.closest?.('.user-picker__panel');
    if (this.open() && !inside) {
      this.close();
    }
  }

  private openList(term: string): void {
    if (!this.open()) {
      this.anchor.set(this.measureAnchor());
      window.addEventListener('scroll', this.closeOnScroll, true);
      window.addEventListener('resize', this.closeOnScroll);
    }
    this.open.set(true);
    this.search.emit(term);
  }

  private measureAnchor(): PanelAnchor {
    const rect = this.hostRef.nativeElement.getBoundingClientRect();
    const below = window.innerHeight - rect.bottom;
    const opensUp = below < PANEL_SPACE && rect.top > below;
    return {
      left: rect.left,
      width: rect.width,
      top: opensUp ? null : rect.bottom,
      bottom: opensUp ? window.innerHeight - rect.top : null,
    };
  }

  /** Closes the list and puts the selected name back in the field (typed text is not a selection). */
  private close(): void {
    window.removeEventListener('scroll', this.closeOnScroll, true);
    window.removeEventListener('resize', this.closeOnScroll);
    this.open.set(false);
    this.typed.set(null);
    this.active.set(0);
  }
}
