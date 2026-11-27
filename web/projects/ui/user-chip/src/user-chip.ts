import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { NgTemplateOutlet } from '@angular/common';
import { Avatar } from '@corelog/ui/avatar';

export interface UserChipUser {
  id: string;
  name: string;
  /** Role or job title shown under the name. */
  role?: string | null;
  avatarUrl?: string | null;
}

/** Avatar with name and, optionally, role and a link to the profile. */
@Component({
  selector: 'app-user-chip',
  imports: [Avatar, NgTemplateOutlet],
  templateUrl: './user-chip.html',
  styleUrl: './user-chip.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class UserChip {
  readonly user = input.required<UserChipUser>();
  /** When present, the chip is a link to the profile. */
  readonly href = input<string>();
  readonly size = input<20 | 24 | 32 | 40>(24);
  readonly dataTestId = input<string | undefined>(undefined, { alias: 'data-testid' });
}
