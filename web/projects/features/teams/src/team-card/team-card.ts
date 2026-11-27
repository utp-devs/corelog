import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { RouterLink } from '@angular/router';
import { Team } from '@corelog/core';

import { TeamTile } from '../team-tile/team-tile';
import { memberCountText } from '../shared/team-messages';

/**
 * Card of a team in the list. The whole card is the link to the team page. It shows the member count only:
 * the API exposes no team lead, so no person is presented as the one in charge.
 */
@Component({
  selector: 'app-team-card',
  imports: [RouterLink, TeamTile],
  templateUrl: './team-card.html',
  styleUrl: './team-card.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TeamCard {
  readonly team = input.required<Team>();

  protected readonly memberCount = computed(() => memberCountText(this.team().memberCount));
}
