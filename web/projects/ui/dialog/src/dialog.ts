import {
  ChangeDetectionStrategy,
  Component,
  DOCUMENT,
  ElementRef,
  Injector,
  OnDestroy,
  afterNextRender,
  effect,
  inject,
  input,
  output,
  untracked,
  viewChild,
} from '@angular/core';

export type DialogSize = 'sm' | 'md' | 'lg';

const FOCUSABLE =
  'button:not([disabled]), [href], input:not([disabled]):not([type="hidden"]), select:not([disabled]), textarea:not([disabled]), [contenteditable="true"], [tabindex]:not([tabindex="-1"])';

let nextDialogId = 0;
/** Number of open dialogs: the page scroll stays locked until the last one closes. */
let openDialogs = 0;

/**
 * Shared dialog shell: backdrop, panel with title (and optional subtitle), close button, scrollable body
 * and a footer for the action buttons. Content goes in the default slot; buttons go in the footer slot
 * by adding the `dialogFooter` attribute to each of them.
 *
 * The shell owns focus (moves into the dialog on open, keeps Tab inside, returns to the opener on close),
 * Escape and the backdrop click. It never closes itself: it emits `closeRequested` and the owner
 * decides, so a form can ask before discarding edits. Focus goes to the element marked `data-autofocus`,
 * else the first field of the body, else the first footer button.
 */
@Component({
  selector: 'app-dialog',
  templateUrl: './dialog.html',
  styleUrl: './dialog.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class Dialog implements OnDestroy {
  private readonly injector = inject(Injector);
  private readonly document = inject(DOCUMENT);
  private readonly panel = viewChild<ElementRef<HTMLElement>>('panel');

  readonly open = input(false);
  /** Title of the dialog, also its accessible name. */
  readonly heading = input.required<string>();
  readonly subtitle = input<string>();
  readonly size = input<DialogSize>('md');
  /** Stack above other overlays, for a confirmation opened from a drawer or another dialog. */
  readonly stacked = input(false);
  /** Escape and a click on the backdrop ask to close. The close button always does. */
  readonly dismissable = input(true);

  readonly closeRequested = output<void>();

  protected readonly titleId = `cl-dialog-title-${nextDialogId++}`;

  private previousFocus: HTMLElement | null = null;
  private locked = false;

  constructor() {
    effect(() => {
      const isOpen = this.open();
      untracked(() => (isOpen ? this.onOpened() : this.onClosed()));
    });
  }

  ngOnDestroy(): void {
    this.onClosed();
  }

  protected requestClose(): void {
    this.closeRequested.emit();
  }

  protected onBackdropMouseDown(event: MouseEvent): void {
    if (this.dismissable() && event.target === event.currentTarget) {
      this.requestClose();
    }
  }

  protected onKeydown(event: KeyboardEvent): void {
    if (event.key === 'Escape') {
      // A nested control (open select, date picker) that used Escape already handled it.
      if (event.defaultPrevented || !this.dismissable()) return;
      event.preventDefault();
      event.stopPropagation();
      this.requestClose();
      return;
    }
    if (event.key === 'Tab') {
      this.trapTab(event);
    }
  }

  private onOpened(): void {
    if (this.locked) return;
    const active = this.document.activeElement;
    this.previousFocus = active instanceof HTMLElement ? active : null;
    this.locked = true;
    if (openDialogs++ === 0) {
      this.document.body.style.overflow = 'hidden';
    }
    afterNextRender(() => this.focusInitial(), { injector: this.injector });
  }

  private onClosed(): void {
    if (!this.locked) return;
    this.locked = false;
    if (--openDialogs === 0) {
      this.document.body.style.removeProperty('overflow');
    }
    const previous = this.previousFocus;
    this.previousFocus = null;
    if (previous?.isConnected) previous.focus();
  }

  private focusInitial(): void {
    const panel = this.panel()?.nativeElement;
    if (!panel) return;
    const target =
      panel.querySelector<HTMLElement>('[data-autofocus]') ??
      panel.querySelector<HTMLElement>(
        '.dialog__body input:not([disabled]), .dialog__body textarea',
      ) ??
      panel.querySelector<HTMLElement>('.dialog__footer button:not([disabled])');
    (target ?? panel).focus();
  }

  /** Keeps Tab and Shift+Tab cycling between the focusable elements of the panel. */
  private trapTab(event: KeyboardEvent): void {
    const panel = this.panel()?.nativeElement;
    if (!panel) return;
    const focusable = Array.from(panel.querySelectorAll<HTMLElement>(FOCUSABLE));
    if (focusable.length === 0) {
      event.preventDefault();
      panel.focus();
      return;
    }
    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    const active = this.document.activeElement;
    if (event.shiftKey && (active === first || active === panel)) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && active === last) {
      event.preventDefault();
      first.focus();
    }
  }
}
