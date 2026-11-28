import { DOCUMENT } from '@angular/common';
import {
  CdkConnectedOverlay,
  CdkOverlayOrigin,
  type ConnectedPosition,
} from '@angular/cdk/overlay';
import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  HostListener,
  Injector,
  afterNextRender,
  effect,
  computed,
  inject,
  input,
  model,
  output,
  viewChild,
} from '@angular/core';

/** Selector of the rows that keyboard navigation moves through. */
const ITEM_SELECTOR = '[role="menuitem"], [role="menuitemcheckbox"], [role="menuitemradio"]';

let nextPanelId = 0;

/** Gap in px between the trigger and the panel. */
const PANEL_GAP = 4;

/** Panel below the trigger lined up with one edge, flipping above when there is no room below. */
function positionsFor(edge: 'start' | 'end'): ConnectedPosition[] {
  return [
    { originX: edge, originY: 'bottom', overlayX: edge, overlayY: 'top', offsetY: PANEL_GAP },
    { originX: edge, originY: 'top', overlayX: edge, overlayY: 'bottom', offsetY: -PANEL_GAP },
  ];
}

/**
 * Menu panel opened by a trigger. The trigger is projected with the `appMenuTrigger` attribute; the
 * rows are the default content and carry `role="menuitem"` (or `menuitemcheckbox` / `menuitemradio`).
 *
 * Behavior: the trigger gets `aria-haspopup`, `aria-expanded` and `aria-controls`; opening from the
 * keyboard moves focus to the first row; Arrow keys, Home and End move between rows; Escape closes
 * and returns focus to the trigger; Tab and a click outside close it; choosing a row closes it unless
 * the row has `data-keep-open`.
 *
 * The panel is rendered through the CDK overlay, so no scrolling or clipping ancestor of the trigger can cut it.
 */
@Component({
  selector: 'app-dropdown-menu',
  imports: [CdkConnectedOverlay, CdkOverlayOrigin],
  templateUrl: './dropdown-menu.html',
  styleUrl: './dropdown-menu.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class DropdownMenu {
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly document = inject(DOCUMENT);
  private readonly injector = inject(Injector);

  /** Accessible name of the menu. */
  readonly label = input.required<string>();
  /** Which edge of the trigger the panel lines up with. */
  readonly align = input<'start' | 'end'>('start');
  readonly open = model(false);
  protected readonly positions = computed(() => positionsFor(this.align()));
  /** Emits when the panel opens, before its rows are rendered. */
  readonly opened = output<void>();

  protected readonly panelId = `app-menu-${nextPanelId++}`;
  private readonly slot = viewChild<ElementRef<HTMLElement>>('slot');
  private readonly panel = viewChild<ElementRef<HTMLElement>>('panel');

  constructor() {
    effect(() => {
      const open = this.open();
      const trigger = this.triggerElement();
      if (!trigger) return;
      trigger.setAttribute('aria-haspopup', 'menu');
      trigger.setAttribute('aria-expanded', String(open));
      if (open) trigger.setAttribute('aria-controls', this.panelId);
      else trigger.removeAttribute('aria-controls');
    });
  }

  toggle(): void {
    if (this.open()) this.close();
    else this.show(false);
  }

  /** Closes the panel. By default focus goes back to the trigger. */
  close(restoreFocus = true): void {
    if (!this.open()) return;
    this.open.set(false);
    if (restoreFocus) this.triggerElement()?.focus();
  }

  private show(viaKeyboard: boolean): void {
    this.open.set(true);
    this.opened.emit();
    afterNextRender(
      () => {
        if (viaKeyboard) this.items()[0]?.focus();
        else this.panel()?.nativeElement.focus();
      },
      { injector: this.injector },
    );
  }

  protected onTriggerClick(event: MouseEvent): void {
    const trigger = this.triggerElement();
    if (!trigger || !(event.target instanceof Node) || !trigger.contains(event.target)) return;
    if (this.open()) this.close(false);
    // A click without pointer movement (detail 0) comes from the keyboard.
    else this.show(event.detail === 0);
  }

  protected onPanelClick(event: MouseEvent): void {
    const target = event.target instanceof Element ? event.target : null;
    const row = target?.closest(ITEM_SELECTOR);
    if (!row || row.hasAttribute('data-keep-open')) return;
    this.close(row.tagName !== 'A');
  }

  protected onPanelKeydown(event: KeyboardEvent): void {
    const items = this.items();
    const current = this.document.activeElement as HTMLElement | null;
    const index = current ? items.indexOf(current) : -1;
    switch (event.key) {
      case 'ArrowDown':
        items[index < 0 ? 0 : (index + 1) % items.length]?.focus();
        break;
      case 'ArrowUp':
        items[index <= 0 ? items.length - 1 : index - 1]?.focus();
        break;
      case 'ArrowRight':
      case 'ArrowLeft': {
        // Left and right only move inside a horizontal group of rows (the theme choice).
        const group = index >= 0 ? items[index].closest('[role="group"]') : null;
        if (!group) return;
        const siblings = items.filter((item) => group.contains(item));
        const at = siblings.indexOf(items[index]);
        const step = event.key === 'ArrowRight' ? 1 : -1;
        siblings[(at + step + siblings.length) % siblings.length]?.focus();
        break;
      }
      case 'Home':
        items[0]?.focus();
        break;
      case 'End':
        items[items.length - 1]?.focus();
        break;
      case 'Escape':
        this.close();
        break;
      case 'Tab':
        this.close(false);
        return;
      default:
        return;
    }
    event.preventDefault();
    event.stopPropagation();
  }

  @HostListener('keydown', ['$event'])
  protected onHostKeydown(event: KeyboardEvent): void {
    const trigger = this.triggerElement();
    if (!trigger || !(event.target instanceof Node) || !trigger.contains(event.target)) return;
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault();
      if (!this.open()) this.show(true);
    }
  }

  @HostListener('document:click', ['$event'])
  protected onDocumentClick(event: MouseEvent): void {
    const target = event.target;
    const inside =
      target instanceof Node &&
      (this.host.nativeElement.contains(target) || !!this.panel()?.nativeElement.contains(target));
    if (this.open() && !inside) this.close(false);
  }

  @HostListener('document:keydown.escape')
  protected onDocumentEscape(): void {
    // Escape pressed while focus is on the trigger (not inside the panel) also closes.
    if (this.open()) this.close();
  }

  private items(): HTMLElement[] {
    const panel = this.panel()?.nativeElement;
    if (!panel) return [];
    return Array.from(panel.querySelectorAll<HTMLElement>(ITEM_SELECTOR)).filter(
      (item) => !item.hasAttribute('disabled') && item.getAttribute('aria-disabled') !== 'true',
    );
  }

  private triggerElement(): HTMLElement | null {
    return this.slot()?.nativeElement.querySelector<HTMLElement>('button, a, [tabindex]') ?? null;
  }
}
