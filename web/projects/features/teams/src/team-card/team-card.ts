import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { RouterLink } from '@angular/router';
import { Team } from '@corelog/core';
import { Lozenge } from '@corelog/ui/lozenge';

import { TeamTile } from '../team-tile/team-tile';
import { memberCountText } from '../shared/team-messages';

/** Card of a team in the list. The whole card is the link to the team page. */
@Component({
  selector: 'app-team-card',
  imports: [RouterLink, TeamTile, Lozenge],
  templateUrl: './team-card.html',
  styleUrl: './team-card.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TeamCard {
  readonly team = input.required<Team>();

  protected readonly isMaintainer = computed(() => this.team().myRole === 'maintainer');
  protected readonly memberCount = computed(() => memberCountText(this.team().memberCount));
}
