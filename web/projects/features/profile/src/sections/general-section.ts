import {
  ChangeDetectionStrategy,
  Component,
  computed,
  inject,
  input,
  model,
  output,
  signal,
} from '@angular/core';
import {
  NotificationService,
  PeopleApiService,
  ProfileUpdate,
  profileFieldErrors,
  timezoneOptions,
  User,
} from '@corelog/core';
import { firstValueFrom } from 'rxjs';

import { messageOr } from '../shared/feature-error';
import { ProfileAvatarUploader } from './avatar-uploader';
import { ProfileForm } from './profile-form';

/** General section: photo, personal data and time zone. */
@Component({
  selector: 'app-profile-general-section',
  imports: [ProfileAvatarUploader, ProfileForm],
  templateUrl: './general-section.html',
  styleUrl: './general-section.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProfileGeneralSection {
  private readonly people = inject(PeopleApiService);
  private readonly notifications = inject(NotificationService);

  readonly user = input.required<User>();
  /** Emits the account after the profile or the photo changed. */
  readonly userChange = output<User>();
  /** True while the personal data has unsaved edits. */
  readonly dirty = model(false);

  protected readonly isSaving = signal(false);
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
