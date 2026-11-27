import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  computed,
  inject,
  input,
  output,
  signal,
  viewChildren,
} from '@angular/core';
import { IssueTransition, WorkflowStatusRef, workflowCategoryVisual } from '@corelog/core';

let nextTransitionButtonId = 0;

/**
 * Shows the current workflow status of an issue and, as a menu, the transitions the person can run from
 * it. Each option names the target status with the color of its category. States: loading, error with
 * retry, empty (no transition available) and busy while a transition runs. The owner runs the transition.
 */
@Component({
  selector: 'app-transition-button',
  templateUrl: './transition-button.html',
  styleUrl: './transition-button.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: {
    '(document:pointerdown)': 'onDocumentPointerDown($event)',
    '(keydown.escape)': 'close(true)',
  },
})
export class TransitionButton {
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly items = viewChildren<ElementRef<HTMLButtonElement>>('item');

  /** Current status of the issue. */
  readonly current = input.required<Pick<WorkflowStatusRef, 'name' | 'category'>>();
  readonly transitions = input<readonly IssueTransition[]>([]);
  readonly loading = input(false);
  /** Message of the failed load of the transitions; shows a retry option. */
  readonly error = input<string | null>(null);
  /** A transition is running. */
  readonly busy = input(false);
  /** Blocks the button with a reason (for example an archived project). */
  readonly disabledReason = input<string>();

  readonly selected = output<IssueTransition>();
  readonly retry = output<void>();

  protected readonly id = `cl-transition-${nextTransitionButtonId++}`;
  protected readonly menuId = `${this.id}-menu`;
  protected readonly open = signal(false);

  protected readonly category = computed(() => workflowCategoryVisual(this.current().category));
  protected readonly empty = computed(
    () => !this.loading() && this.error() === null && this.transitions().length === 0,
  );
  protected readonly disabled = computed(
    () => this.busy() || this.disabledReason() !== undefined || this.empty() || this.loading(),
  );
  protected readonly hint = computed(() => {
    if (this.disabledReason()) return this.disabledReason();
    if (this.empty()) return 'No hay transiciones disponibles desde este estado.';
    return null;
  });

  protected categoryOf(transition: IssueTransition): string {
    return workflowCategoryVisual(transition.to.category);
  }

  protected toggle(): void {
    if (this.disabled() && this.error() === null) return;
    this.open.update((value) => !value);
    if (this.open()) queueMicrotask(() => this.items()[0]?.nativeElement.focus());
  }

  protected close(refocus = false): void {
    if (!this.open()) return;
    this.open.set(false);
    if (refocus) {
      this.host.nativeElement.querySelector<HTMLElement>('.transition-button__trigger')?.focus();
    }
  }

  protected choose(transition: IssueTransition): void {
    this.close(true);
    this.selected.emit(transition);
  }

  protected retryLoad(): void {
    this.close();
    this.retry.emit();
  }

  protected onMenuKeydown(event: KeyboardEvent): void {
    const buttons = this.items().map((item) => item.nativeElement);
    const index = buttons.indexOf(document.activeElement as HTMLButtonElement);
    let next = -1;
    if (event.key === 'ArrowDown') next = (index + 1) % buttons.length;
    else if (event.key === 'ArrowUp') next = (index - 1 + buttons.length) % buttons.length;
    else if (event.key === 'Home') next = 0;
    else if (event.key === 'End') next = buttons.length - 1;
    else if (event.key === 'Tab') this.close();
    if (next >= 0) {
      event.preventDefault();
      buttons[next]?.focus();
    }
  }

  protected onDocumentPointerDown(event: Event): void {
    if (this.open() && !this.host.nativeElement.contains(event.target as Node)) this.close();
  }
}
