import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';

/** One step: the text under its circle. */
export interface StepperStep {
  readonly label: string;
}

/**
 * Progress indicator of a multi-step flow: numbered circles joined by a line (completed, current and pending).
 * Completed steps are buttons that go back; the current one carries `aria-current="step"`. On phones the circles
 * collapse into the text "Paso N de M" over a progress bar.
 */
@Component({
  selector: 'app-stepper',
  templateUrl: './stepper.html',
  styleUrl: './stepper.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class Stepper {
  readonly steps = input.required<readonly StepperStep[]>();
  /** Zero-based index of the step being filled. */
  readonly current = input(0);
  /** Name of the navigation landmark. */
  readonly label = input('Progreso');
  /** When false, completed steps are not links back (a step that cannot be undone). */
  readonly backable = input(true);
  /** Emits the index of a completed step the person chose to go back to. */
  readonly select = output<number>();

  protected readonly total = computed(() => this.steps().length);
  protected readonly progress = computed(() => `${((this.current() + 1) / this.total()) * 100}%`);
}
