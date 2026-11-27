import { ChangeDetectionStrategy, Component } from '@angular/core';
import { EmptyState } from '@corelog/ui/empty-state';

/** Placeholder until API access tokens are available. */
@Component({
  selector: 'app-profile-tokens-section',
  imports: [EmptyState],
  templateUrl: './tokens-section.html',
  styleUrl: './tokens-section.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProfileTokensSection {}
