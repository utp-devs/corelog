import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { ActivatedRoute } from '@angular/router';
import { AuthStore, User, warnBeforeUnload } from '@corelog/core';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader } from '@corelog/ui/page-header';
import { map } from 'rxjs';

import { ProfileAppearanceSection } from '../sections/appearance-section';
import { ProfileGeneralSection } from '../sections/general-section';
import { ProfileNotificationsSection } from '../sections/notifications-section';
import { ProfileSecuritySection } from '../sections/security-section';

type SectionKey = 'general' | 'security' | 'notifications' | 'appearance';

interface SectionMeta {
  readonly title: string;
  readonly description: string;
}

/** Title and description shown above each section. The shell sidebar is the navigation between them. */
const SECTIONS: Readonly<Record<SectionKey, SectionMeta>> = {
  general: {
    title: 'General',
    description: 'Tu foto, tus datos personales y tu zona horaria.',
  },
  security: {
    title: 'Seguridad',
    description: 'Contraseña, sesiones abiertas y eliminación de la cuenta.',
  },
  notifications: {
    title: 'Notificaciones',
    description: 'Elige cómo quieres recibir los avisos.',
  },
  appearance: {
    title: 'Apariencia',
    description: 'Tema y densidad de la interfaz en este navegador.',
  },
};

function sectionOf(value: string | null): SectionKey {
  return value !== null && Object.hasOwn(SECTIONS, value) ? (value as SectionKey) : 'general';
}

/** Profile page: shows the section of /profile/:section (general when no section is given). */
@Component({
  selector: 'app-profile-page',
  imports: [
    PageContainer,
    PageHeader,
    ProfileAppearanceSection,
    ProfileGeneralSection,
    ProfileNotificationsSection,
    ProfileSecuritySection,
  ],
  templateUrl: './profile-page.html',
  styleUrl: './profile-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProfilePage {
  private readonly auth = inject(AuthStore);
  private readonly route = inject(ActivatedRoute);

  /** Section parameter of the route; null when the route has none (the general section). */
  private readonly sectionParam = toSignal(
    this.route.paramMap.pipe(map((params) => params.get('section'))),
    { initialValue: this.route.snapshot.paramMap.get('section') },
  );

  protected readonly section = computed(() => sectionOf(this.sectionParam()));
  protected readonly meta = computed(() => SECTIONS[this.section()]);
  protected readonly user = this.auth.user;

  /** Unsaved edits in the general section. Only relevant while that section is shown. */
  protected readonly generalDirty = signal(false);
  /** Read by the unsaved changes guard of the profile routes. */
  readonly dirty = computed(() => this.section() === 'general' && this.generalDirty());

  constructor() {
    warnBeforeUnload(this.dirty);
  }

  /** A section saved the profile or the photo: the signed-in account is updated in the store. */
  protected onUserChange(user: User): void {
    this.auth.setUser(user);
  }
}
