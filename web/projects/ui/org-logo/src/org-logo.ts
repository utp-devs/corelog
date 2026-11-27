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
import { OrgLogoService, OrgLogoWatch } from '@corelog/core';
import { initialsOf, paletteIndex } from '@corelog/ui/avatar';

export type OrgLogoSize = 24 | 32;

interface WatchState {
  readonly key: string;
  readonly watch: OrgLogoWatch;
}

/**
 * Organization logo fetched with the session, or a colored tile with the initials of the name
 * (project avatar style) when there is no logo. The tile is decorative: the name is announced elsewhere.
 */
@Component({
  selector: 'app-org-logo',
  templateUrl: './org-logo.html',
  styleUrl: './org-logo.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class OrgLogo {
  private readonly logos = inject(OrgLogoService);

  readonly orgId = input.required<string>();
  /** Origin of the initials shown when there is no logo. */
  readonly name = input.required<string>();
  /** Organization.hasLogo. False means no request is made and the initials are shown. */
  readonly hasLogo = input(false, { transform: booleanAttribute });
  /** Changes after an upload or removal so the logo is fetched again. */
  readonly revision = input(0);
  readonly size = input<OrgLogoSize>(24);

  private readonly watchState = signal<WatchState | null>(null);
  private readonly requestKey = computed(
    () => `${this.orgId()}|${this.hasLogo()}|${this.revision()}`,
  );

  /** A logo that belongs to another organization or revision is never shown. */
  protected readonly src = computed(() => {
    const state = this.watchState();
    return state !== null && state.key === this.requestKey() ? state.watch.url() : null;
  });
  protected readonly initials = computed(() => initialsOf(this.name()));
  protected readonly palette = computed(() => paletteIndex(this.orgId()));

  constructor() {
    effect((onCleanup) => {
      const key = this.requestKey();
      const watch = this.logos.watch(this.orgId(), this.hasLogo(), this.revision());
      this.watchState.set({ key, watch });
      onCleanup(() => watch.release());
    });
  }
}
