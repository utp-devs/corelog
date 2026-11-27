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

export type SidePanelSize = 'md' | 'lg';

const FOCUSABLE =
  'button:not([disabled]), [href], input:not([disabled]):not([type="hidden"]), select:not([disabled]), textarea:not([disabled]), [contenteditable="true"], [tabindex]:not([tabindex="-1"])';

let nextPanelId = 0;
/** Open panels, the last one is on top: only it reacts to Escape. */
const openStack: SidePanel[] = [];
/** Number of open panels: the page scroll stays locked until the last one closes. */
let openPanels = 0;

/**
 * Panel that slides in from the right edge over the page (a full-screen sheet on phones), with a
 * title, a close button, a scrollable body and a footer slot (`panelFooter` attribute).
 *
 * It owns focus (moves in on open, keeps Tab inside, returns to the opener on close), Escape and the
 * backdrop click. It never closes itself: it emits `closeRequested` and the owner decides. Focus goes
 * to the element marked `data-autofocus`, else the first field of the body.
 */
@Component({
  selector: 'app-side-panel',
  templateUrl: './side-panel.html',
  styleUrl: './side-panel.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class SidePanel implements OnDestroy {
  private readonly injector = inject(Injector);
  private readonly document = inject(DOCUMENT);
  private readonly panel = viewChild<ElementRef<HTMLElement>>('panel');

  readonly open = input(false);
  /** Title, also the accessible name of the panel. */
  readonly heading = input.required<string>();
  readonly subtitle = input<string>();
  readonly size = input<SidePanelSize>('md');

  readonly closeRequested = output<void>();

  protected readonly titleId = `cl-side-panel-title-${nextPanelId++}`;

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
    if (event.target === event.currentTarget) this.requestClose();
  }

  protected onKeydown(event: KeyboardEvent): void {
    if (event.key === 'Tab') this.trapTab(event);
  }

  /** Escape closes the top panel wherever the focus is (a disabled button can leave it on the page). */
  private readonly onDocumentKeydown = (event: KeyboardEvent): void => {
    if (event.key !== 'Escape' || event.defaultPrevented) return;
    // A nested control (open menu, picker) that used Escape already handled it.
    if (openStack[openStack.length - 1] !== this) return;
    event.preventDefault();
    this.requestClose();
  };

  private onOpened(): void {
    if (this.locked) return;
    const active = this.document.activeElement;
    this.previousFocus = active instanceof HTMLElement ? active : null;
    this.locked = true;
    openStack.push(this);
    this.document.addEventListener('keydown', this.onDocumentKeydown);
    if (openPanels++ === 0) this.document.body.style.overflow = 'hidden';
    afterNextRender(() => this.focusInitial(), { injector: this.injector });
  }

  private onClosed(): void {
    if (!this.locked) return;
    this.locked = false;
    openStack.splice(openStack.indexOf(this), 1);
    this.document.removeEventListener('keydown', this.onDocumentKeydown);
    if (--openPanels === 0) this.document.body.style.removeProperty('overflow');
    const previous = this.previousFocus;
    this.previousFocus = null;
    if (previous?.isConnected) previous.focus();
  }

  private focusInitial(): void {
    const panel = this.panel()?.nativeElement;
    if (!panel) return;
    const marked = panel.querySelector<HTMLElement>('[data-autofocus]');
    // The marker may sit on a wrapper (a component host): focus the control inside it.
    const target =
      (marked?.matches(FOCUSABLE) ? marked : marked?.querySelector<HTMLElement>(FOCUSABLE)) ??
      panel.querySelector<HTMLElement>(
        '.side-panel__body input:not([disabled]), .side-panel__body textarea',
      );
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
