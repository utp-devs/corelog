import { formatDate } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  computed,
  inject,
  input,
  LOCALE_ID,
  model,
  output,
  signal,
} from '@angular/core';
import {
  AppRole,
  NotificationService,
  PeopleApiService,
  ProfileUpdate,
  profileFieldErrors,
  timezoneOptions,
  User,
} from '@corelog/core';
import { Section } from '@corelog/ui/section';
import { ToneTag } from '@corelog/ui/tone-tag';
import { firstValueFrom } from 'rxjs';

import { messageOr } from '../shared/feature-error';
import { ProfileAvatarUploader } from './avatar-uploader';
import { ProfileForm } from './profile-form';

const ROLE_LABELS: Readonly<Record<AppRole, string>> = {
  admin: 'Administrador',
  user: 'Usuario',
};

const MEMBER_SINCE_PATTERN = 'dd/MM/yyyy';

/** General section: photo, account summary, personal data and time zone. */
@Component({
  selector: 'app-profile-general-section',
  imports: [ProfileAvatarUploader, ProfileForm, Section, ToneTag],
  templateUrl: './general-section.html',
  styleUrl: './general-section.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProfileGeneralSection {
  private readonly people = inject(PeopleApiService);
  private readonly notifications = inject(NotificationService);
  private readonly locale = inject(LOCALE_ID);

  readonly user = input.required<User>();
  /** Emits the account after the profile or the photo changed. */
  readonly userChange = output<User>();
  /** True while the personal data has unsaved edits. */
  readonly dirty = model(false);

  protected readonly isSaving = signal(false);
  protected readonly shownName = computed(() => this.user().displayName || this.user().name);
  protected readonly roleLabel = computed(() => ROLE_LABELS[this.user().role]);
  protected readonly memberSince = computed(() => {
    const created = this.user().createdAt;
    return created ? formatDate(created, MEMBER_SINCE_PATTERN, this.locale) : null;
  });
  protected readonly timezones = computed(() => timezoneOptions(this.user().timezone));

  /** Saves the personal data. Field errors are returned to the form; other failures are reported here. */
  protected readonly save = (payload: ProfileUpdate): Promise<void> => this.saveProfile(payload);

  private async saveProfile(payload: ProfileUpdate): Promise<void> {
    this.isSaving.set(true);
    try {
      const saved = await firstValueFrom(this.people.updateMe(payload));
      this.userChange.emit(saved);
      this.notifications.success('Perfil actualizado');
    } catch (error) {
      if (Object.keys(profileFieldErrors(error)).length === 0) {
        this.notifications.error(messageOr(error, 'No se pudo actualizar el perfil'));
      }
      throw error;
    } finally {
      this.isSaving.set(false);
    }
  }
}
