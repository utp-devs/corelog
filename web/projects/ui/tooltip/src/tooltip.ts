import {
  DOCUMENT,
  Directive,
  ElementRef,
  booleanAttribute,
  effect,
  inject,
  input,
} from '@angular/core';
import { Tooltip } from 'primeng/tooltip';

/** Pause before the tooltip appears, in milliseconds. */
export const TOOLTIP_DELAY_MS = 400;

const COARSE_POINTER_QUERY = '(pointer: coarse)';
let nextId = 0;

/**
 * Tooltip for icon-only controls and truncated text. It wraps the PrimeNG tooltip with one set of
 * defaults: 400 ms delay, shown on hover and on keyboard focus, placed below the host, closed with
 * Escape, and linked to the host with `aria-describedby` while it is visible. On touch devices
 * (coarse pointer) it stays off, because a long press would compete with the control's own action.
 *
 * Usage: `<button appTooltip="Notificaciones" aria-label="Notificaciones">`.
 */
@Directive({
  selector: '[appTooltip]',
  hostDirectives: [
    { directive: Tooltip, inputs: ['pTooltip: appTooltip', 'tooltipPosition: appTooltipPosition'] },
  ],
})
export class AppTooltip {
  private readonly primeTooltip = inject(Tooltip, { self: true });
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef).nativeElement;
  private readonly tooltipId = `app-tooltip-${nextId++}`;

  /** Turns the tooltip off, for instance while the control explains itself some other way. */
  readonly appTooltipDisabled = input(false, { transform: booleanAttribute });

  constructor() {
    const coarse =
      inject(DOCUMENT).defaultView?.matchMedia?.(COARSE_POINTER_QUERY).matches ?? false;
    this.primeTooltip.setOption({
      tooltipEvent: 'both',
      tooltipPosition: 'bottom',
      showDelay: TOOLTIP_DELAY_MS,
      hideOnEscape: true,
      disabled: coarse,
    });
    this.primeTooltip.tooltipPosition = 'bottom';
    this.linkDescription();

    effect(() => {
      const disabled = this.appTooltipDisabled() || coarse;
      this.primeTooltip.setOption({ disabled });
      if (disabled) this.primeTooltip.deactivate();
    });
  }

  /** Points `aria-describedby` at the tooltip element while it exists. */
  private linkDescription(): void {
    const show = this.primeTooltip.show.bind(this.primeTooltip);
    const remove = this.primeTooltip.remove.bind(this.primeTooltip);
    this.primeTooltip.show = () => {
      show();
      const container = this.primeTooltip.container as HTMLElement | null | undefined;
      if (container) {
        container.id = this.tooltipId;
        this.host.setAttribute('aria-describedby', this.tooltipId);
      }
    };
    this.primeTooltip.remove = () => {
      remove();
      if (this.host.getAttribute('aria-describedby') === this.tooltipId) {
        this.host.removeAttribute('aria-describedby');
      }
    };
  }
}
