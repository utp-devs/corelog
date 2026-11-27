import { formatDate } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  computed,
  inject,
  LOCALE_ID,
  signal,
} from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { ActivatedRoute, RouterLink } from '@angular/router';
import {
  AppRole,
  AuthStore,
  BreakpointService,
  breakpointQuery,
  User,
  warnBeforeUnload,
} from '@corelog/core';
import { PageContainer } from '@corelog/ui/page-container';
import { ToneTag } from '@corelog/ui/tone-tag';
import { UserAvatar } from '@corelog/ui/user-avatar';
import { map } from 'rxjs';

import { ProfileAppearanceSection } from '../sections/appearance-section';
import { ProfileGeneralSection } from '../sections/general-section';
import { ProfileNotificationsSection } from '../sections/notifications-section';
import { ProfileSecuritySection } from '../sections/security-section';
import { ProfileTokensSection } from '../sections/tokens-section';

type SectionKey = 'general' | 'security' | 'notifications' | 'tokens' | 'appearance';

interface SectionItem {
  readonly key: SectionKey;
  readonly label: string;
  readonly icon: string;
}

const SECTIONS: readonly SectionItem[] = [
  { key: 'general', label: 'General', icon: 'pi pi-user' },
  { key: 'security', label: 'Seguridad', icon: 'pi pi-lock' },
  { key: 'notifications', label: 'Notificaciones', icon: 'pi pi-bell' },
  { key: 'tokens', label: 'Tokens', icon: 'pi pi-key' },
  { key: 'appearance', label: 'Apariencia', icon: 'pi pi-palette' },
];

const ROLE_LABELS: Readonly<Record<AppRole, string>> = {
  admin: 'Administrador',
  user: 'Usuario',
};

const MEMBER_SINCE_PATTERN = 'dd/MM/yyyy';

function sectionOf(value: string | null): SectionKey {
  return SECTIONS.some((section) => section.key === value) ? (value as SectionKey) : 'general';
}

/** Profile page: section navigation under /profile/:section (general when no section is given). */
@Component({
  selector: 'app-profile-page',
  imports: [
    PageContainer,
    ProfileAppearanceSection,
    ProfileGeneralSection,
    ProfileNotificationsSection,
    ProfileSecuritySection,
    ProfileTokensSection,
    RouterLink,
    ToneTag,
    UserAvatar,
  ],
  templateUrl: './profile-page.html',
  styleUrl: './profile-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProfilePage {
  private readonly auth = inject(AuthStore);
  private readonly route = inject(ActivatedRoute);
  private readonly breakpoints = inject(BreakpointService);
  private readonly locale = inject(LOCALE_ID);

  /** Section parameter of the route; null when the route has none (the general section). */
  private readonly sectionParam = toSignal(
    this.route.paramMap.pipe(map((params) => params.get('section'))),
    { initialValue: this.route.snapshot.paramMap.get('section') },
  );

  protected readonly sections = SECTIONS;
  protected readonly section = computed(() => sectionOf(this.sectionParam()));
  /** Desktop shows the vertical menu; narrower screens show a horizontal row of tabs. */
  protected readonly isDesktop = this.breakpoints.matches(breakpointQuery('lg'));

  /** Set when a section reports a new account state (saved profile or new photo). */
  private readonly userOverride = signal<User | null>(null);
  protected readonly user = computed(() => this.userOverride() ?? this.auth.user());
  protected readonly shownName = computed(() => {
    const user = this.user();
    return user ? user.displayName || user.name : '';
  });
  protected readonly memberSince = computed(() => {
    const user = this.user();
    return user?.createdAt ? formatDate(user.createdAt, MEMBER_SINCE_PATTERN, this.locale) : null;
  });

  /** Unsaved edits in the general section. Only relevant while that section is shown. */
  protected readonly generalDirty = signal(false);
  /** Read by the unsaved changes guard of the profile routes. */
  readonly dirty = computed(() => this.section() === 'general' && this.generalDirty());

  constructor() {
    warnBeforeUnload(this.dirty);
  }

  protected roleLabel(role: AppRole): string {
    return ROLE_LABELS[role];
  }

  protected onUserChange(user: User): void {
    this.userOverride.set(user);
  }

  /** Links keep the current mount: the section route is a sibling of the route without a section. */
  protected linkTo(key: SectionKey): string[] {
    return this.sectionParam() === null ? [key] : ['..', key];
  }

  protected isActive(key: SectionKey): boolean {
    return this.section() === key;
  }
}
