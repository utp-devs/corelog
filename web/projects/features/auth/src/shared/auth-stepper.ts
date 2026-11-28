import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';

/** One step of the registration: the text under its circle. */
export interface AuthStep {
  readonly label: string;
}

/**
 * Progress indicator of the multi-step access forms: numbered circles joined by a line (completed, current and
 * pending states). Completed steps are buttons that go back; the current one carries `aria-current="step"`.
 * On phones the circles collapse into the text "Paso N de M" over a progress bar.
 */
@Component({
  selector: 'app-auth-stepper',
  templateUrl: './auth-stepper.html',
  styleUrl: './auth-stepper.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class AuthStepper {
  readonly steps = input.required<readonly AuthStep[]>();
  /** Zero-based index of the step being filled. */
  readonly current = input(0);
  /** Name of the navigation landmark. */
  readonly label = input('Progreso del registro');
  /** Emits the index of a completed step the person chose to go back to. */
  readonly select = output<number>();

  protected readonly total = computed(() => this.steps().length);
  protected readonly progress = computed(() => `${((this.current() + 1) / this.total()) * 100}%`);
  protected readonly summary = computed(
    () => `Paso ${this.current() + 1} de ${this.total()}: ${this.steps()[this.current()]?.label}`,
  );
}
