import { ChangeDetectionStrategy, Component, input } from '@angular/core';

export type Tone = 'neutral' | 'blue' | 'gold' | 'green' | 'orange' | 'red' | 'purple';

/** Semantic color chip. The colors come from --cl-* tokens, so dark mode needs no extra rules. */
@Component({
  selector: 'app-tone-tag',
  templateUrl: './tone-tag.html',
  styleUrl: './tone-tag.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ToneTag {
  readonly label = input.required<string>();
  readonly tone = input.required<Tone>();
}
