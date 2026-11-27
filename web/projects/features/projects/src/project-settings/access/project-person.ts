import {
  ChangeDetectionStrategy,
  Component,
  booleanAttribute,
  computed,
  effect,
  inject,
  input,
  signal,
} from '@angular/core';
import { AvatarUrlService, AvatarUrlWatch } from '@corelog/core';
import { Avatar } from '@corelog/ui/avatar';

interface WatchState {
  readonly key: string;
  readonly watch: AvatarUrlWatch;
}

/** Avatar, name and email of a person, with the "(tú)" marker for the signed-in user. */
@Component({
  selector: 'app-project-person',
  imports: [Avatar],
  templateUrl: './project-person.html',
  styleUrl: './project-person.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProjectPerson {
  private readonly avatars = inject(AvatarUrlService);

  readonly userId = input.required<string>();
  readonly name = input.required<string>();
  readonly email = input<string>();
  readonly hasAvatar = input(false, { transform: booleanAttribute });
  readonly isSelf = input(false, { transform: booleanAttribute });

  private readonly watchState = signal<WatchState | null>(null);
  private readonly requestKey = computed(() => `${this.userId()}|${this.hasAvatar()}`);

  protected readonly src = computed(() => {
    const state = this.watchState();
    return state !== null && state.key === this.requestKey() ? state.watch.url() : null;
  });

  constructor() {
    effect((onCleanup) => {
      const key = this.requestKey();
      const watch = this.avatars.watch(this.userId(), this.hasAvatar());
      this.watchState.set({ key, watch });
      onCleanup(() => watch.release());
    });
  }
}
