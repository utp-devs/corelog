import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { Avatar } from '@corelog/ui/avatar';

export interface AvatarUser {
  id: string;
  name: string;
}

/** Group of initial avatars for assignees and members. */
@Component({
  selector: 'app-avatar-group',
  imports: [Avatar],
  templateUrl: './avatar-group.html',
  styleUrl: './avatar-group.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class AvatarGroup {
  readonly users = input.required<AvatarUser[]>();
  /** How many avatars to show before the rest is summarized as "+N". */
  readonly max = input(4);

  protected readonly items = computed(() =>
    this.users()
      .slice(0, this.max())
      .map((user) => ({ id: user.id, name: user.name })),
  );
  protected readonly hidden = computed(() => this.users().length - this.items().length);
}
