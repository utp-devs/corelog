import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  OnDestroy,
  inject,
  input,
  output,
  signal,
  viewChild,
} from '@angular/core';
import { firstValueFrom } from 'rxjs';
import {
  AVATAR_MAX_BYTES,
  AvatarApiService,
  NotificationService,
  PeopleApiService,
  User,
  avatarErrorMessage,
  validateAvatarFile,
} from '@corelog/core';
import { Avatar } from '@corelog/ui/avatar';
import { UserAvatar } from '@corelog/ui/user-avatar';
import { Button } from 'primeng/button';

import { messageOr } from '../shared/feature-error';

interface Preview {
  readonly file: File;
  readonly url: string;
}

const MEGABYTE = 1024 * 1024;

/** Photo of the user with upload (PNG, JPEG or WebP up to 2 MB), preview and removal. */
@Component({
  selector: 'app-profile-avatar-uploader',
  imports: [Avatar, Button, UserAvatar],
  templateUrl: './avatar-uploader.html',
  styleUrl: './avatar-uploader.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProfileAvatarUploader implements OnDestroy {
  private readonly avatars = inject(AvatarApiService);
  private readonly people = inject(PeopleApiService);
  private readonly notifications = inject(NotificationService);
  private readonly fileInput = viewChild.required<ElementRef<HTMLInputElement>>('fileInput');

  readonly user = input.required<User>();
  /** Emits the account after the photo was saved or removed. */
  readonly userChange = output<User>();

  protected readonly sizeLimitMb = AVATAR_MAX_BYTES / MEGABYTE;
  protected readonly preview = signal<Preview | null>(null);
  protected readonly error = signal<string | null>(null);
  protected readonly busy = signal<'upload' | 'remove' | null>(null);

  ngOnDestroy(): void {
    this.revokePreview(this.preview());
  }

  protected chooseFile(): void {
    this.fileInput().nativeElement.click();
  }

  protected onPick(event: Event): void {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    input.value = '';
    if (!file) return;

    const problem = validateAvatarFile(file);
    if (problem) {
      this.error.set(problem);
      this.replacePreview(null);
      return;
    }
    this.error.set(null);
    this.replacePreview({ file, url: URL.createObjectURL(file) });
  }

  protected cancelPreview(): void {
    this.replacePreview(null);
  }

  protected async upload(): Promise<void> {
    const current = this.preview();
    if (!current) return;
    this.busy.set('upload');
    try {
      await firstValueFrom(this.avatars.uploadMyAvatar(current.file));
      await this.refreshUser();
      this.replacePreview(null);
      this.notifications.success('Foto de perfil actualizada');
    } catch (failure) {
      this.error.set(avatarErrorMessage(failure) ?? messageOr(failure, 'No se pudo subir la foto'));
    } finally {
      this.busy.set(null);
    }
  }

  protected async remove(): Promise<void> {
    this.busy.set('remove');
    try {
      await firstValueFrom(this.avatars.deleteMyAvatar());
      await this.refreshUser();
      this.error.set(null);
      this.notifications.success('Foto de perfil eliminada');
    } catch (failure) {
      this.error.set(messageOr(failure, 'No se pudo quitar la foto'));
    } finally {
      this.busy.set(null);
    }
  }

  private async refreshUser(): Promise<void> {
    this.userChange.emit(await firstValueFrom(this.people.getMe()));
  }

  /** The object URL of the preview is released when it is replaced or when the component goes away. */
  private replacePreview(next: Preview | null): void {
    this.revokePreview(this.preview());
    this.preview.set(next);
  }

  private revokePreview(current: Preview | null): void {
    if (current) URL.revokeObjectURL(current.url);
  }
}
