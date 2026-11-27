import { ChangeDetectionStrategy, Component, computed, input, signal } from '@angular/core';

import { avatarColorIndex } from './avatar-color';
import { initialsOf } from './initials';

export type AvatarSize = 16 | 20 | 24 | 32 | 40 | 64;

/** Avatar with a photo or initials. The background color is fixed for each id. */
@Component({
  selector: 'app-avatar',
  templateUrl: './avatar.html',
  styleUrl: './avatar.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class Avatar {
  /** Stable user identifier: decides the background color. */
  readonly id = input.required<string>();
  /** Visible name: source of the initials and accessible name. */
  readonly name = input.required<string>();
  /** Profile photo. When it is missing or fails to load, the initials are shown. */
  readonly src = input<string | null | undefined>(null);
  readonly size = input<AvatarSize>(32);
  readonly dataTestId = input<string | undefined>(undefined, { alias: 'data-testid' });

  /** Photo address that failed to load. It stays hidden until the address changes. */
  protected readonly failedSrc = signal<string | null>(null);
  protected readonly showImage = computed(() => {
    const source = this.src();
    return !!source && source !== this.failedSrc();
  });
  protected readonly initials = computed(() => initialsOf(this.name()));
  protected readonly sizeClass = computed(() => `avatar--size-${this.size()}`);
  protected readonly toneClass = computed(() => `avatar--tone-${avatarColorIndex(this.id())}`);

  protected onImageError(): void {
    this.failedSrc.set(this.src() ?? null);
  }
}
