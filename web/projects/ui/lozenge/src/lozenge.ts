import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

export type LozengeCategory = 'todo' | 'progress' | 'done';
export type LozengeTone = 'info' | 'success' | 'warning' | 'danger' | 'neutral' | 'discovery';

/** Compact chip for a workflow state or a semantic tone. The projected text is always visible. */
@Component({
  selector: 'app-lozenge',
  templateUrl: './lozenge.html',
  styleUrl: './lozenge.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class Lozenge {
  /** Workflow category. Takes priority over the tone. */
  readonly category = input<LozengeCategory>();
  readonly tone = input<LozengeTone>('neutral');
  readonly dataTestId = input<string | undefined>(undefined, { alias: 'data-testid' });

  protected readonly variant = computed<LozengeCategory | LozengeTone>(
    () => this.category() ?? this.tone(),
  );
}
