import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  input,
  signal,
} from '@angular/core';
import { AvatarUrlService, type AvatarUrlWatch } from '@corelog/core';
import { Avatar, type AvatarSize } from '@corelog/ui/avatar';

interface WatchedAvatar {
  readonly key: string;
  readonly watch: AvatarUrlWatch;
}

/**
 * Avatar of a user: the photo (fetched with the session, only when the user has one) or initials.
 * Pass the user's avatar key as `avatarKey` so a changed photo is loaded again.
 */
@Component({
  selector: 'app-user-avatar',
  imports: [Avatar],
  templateUrl: './user-avatar.html',
  styleUrl: './user-avatar.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class UserAvatar {
  private readonly avatarUrls = inject(AvatarUrlService);

  readonly id = input.required<string>();
  readonly name = input.required<string>();
  /** When false no image is requested and initials are shown. Defaults to whether an avatar key exists. */
  readonly hasAvatar = input<boolean>();
  readonly avatarKey = input<string | null>(null);
  readonly size = input<AvatarSize>(32);
  readonly dataTestId = input<string | undefined>(undefined, { alias: 'data-testid' });

  private readonly request = computed(() => {
    const version = this.avatarKey() ?? '';
    const withPhoto = this.hasAvatar() ?? version !== '';
    return { id: this.id(), withPhoto, version, key: `${this.id()}|${withPhoto}|${version}` };
  });

  private readonly watched = signal<WatchedAvatar | null>(null);

  /** Object URL of the photo, or null while loading, without a photo, or when the fetch failed. */
  protected readonly src = computed(() => {
    const current = this.watched();
    const request = this.request();
    if (!current || current.key !== request.key) return null;
    return current.watch.url();
  });

  constructor() {
    effect((onCleanup) => {
      const request = this.request();
      const watch = this.avatarUrls.watch(request.id, request.withPhoto, request.version);
      this.watched.set({ key: request.key, watch });
      onCleanup(() => watch.release());
    });
  }
}
