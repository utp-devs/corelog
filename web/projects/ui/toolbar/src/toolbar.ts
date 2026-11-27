import { BreakpointObserver } from '@angular/cdk/layout';
import { NgTemplateOutlet } from '@angular/common';
import {
  booleanAttribute,
  ChangeDetectionStrategy,
  Component,
  computed,
  contentChild,
  Directive,
  ElementRef,
  inject,
  input,
  signal,
  viewChild,
} from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { map } from 'rxjs';

const OVERFLOW_QUERY = '(max-width: 1279.98px)';

/** Marks the element projected into the left (filters and search) slot of a toolbar. */
@Directive({
  selector: '[appToolbarLeft]',
})
export class ToolbarLeft {}

/** Marks the element projected into the right (primary actions) slot of a toolbar. */
@Directive({
  selector: '[appToolbarRight]',
})
export class ToolbarRight {}

/**
 * Marks the element projected into the secondary actions slot. Shown inline from 1280 px; below it
 * the actions move into a "Más" menu.
 */
@Directive({
  selector: '[appToolbarSecondary]',
})
export class ToolbarSecondary {}

/**
 * Horizontal action bar: left filters, right actions, wraps on small screens, optional "Más" overflow.
 * Slots are projected through the appToolbarLeft, appToolbarRight and appToolbarSecondary attributes.
 */
@Component({
  selector: 'app-toolbar',
  imports: [NgTemplateOutlet],
  templateUrl: './toolbar.html',
  styleUrl: './toolbar.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: {
    '(document:keydown.escape)': 'onEscape()',
    '(document:click)': 'onDocumentClick($event)',
  },
})
export class Toolbar {
  /** Sticks under the top bar (offset by the top bar height) while the page scrolls. */
  readonly sticky = input(false, { transform: booleanAttribute });

  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly breakpoints = inject(BreakpointObserver);
  private readonly trigger = viewChild<ElementRef<HTMLButtonElement>>('trigger');

  private readonly leftSlot = contentChild(ToolbarLeft);
  private readonly rightSlot = contentChild(ToolbarRight);
  private readonly secondarySlot = contentChild(ToolbarSecondary);

  protected readonly hasLeft = computed(() => this.leftSlot() !== undefined);
  protected readonly hasRight = computed(() => this.rightSlot() !== undefined);
  protected readonly hasSecondary = computed(() => this.secondarySlot() !== undefined);

  protected readonly overflow = toSignal(
    this.breakpoints.observe(OVERFLOW_QUERY).pipe(map((state) => state.matches)),
    { initialValue: this.breakpoints.isMatched(OVERFLOW_QUERY) },
  );
  protected readonly moveToMenu = computed(() => this.hasSecondary() && this.overflow());
  protected readonly menuOpen = signal(false);

  protected toggleMenu(): void {
    this.menuOpen.update((open) => !open);
  }

  protected closeMenu(restoreFocus: boolean): void {
    if (!this.menuOpen()) return;
    this.menuOpen.set(false);
    if (restoreFocus) this.trigger()?.nativeElement.focus();
  }

  /** Escape closes the overflow menu and returns focus to its trigger, like a dropdown. */
  protected onEscape(): void {
    this.closeMenu(true);
  }

  /** A click outside the toolbar closes the overflow menu. */
  protected onDocumentClick(event: Event): void {
    const target = event.target;
    if (target instanceof Node && this.host.nativeElement.contains(target)) return;
    this.closeMenu(false);
  }
}
