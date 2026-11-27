import { TestBed, ComponentFixture } from '@angular/core/testing';
import { AvatarApiService, PeopleApiService, User } from '@corelog/core';
import { provideCorelogTesting } from '@corelog/core/testing';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { of, throwError } from 'rxjs';

import { ProfileAvatarUploader } from './avatar-uploader';

const user: User = {
  id: 'user-1',
  name: 'Ana Torres',
  email: 'ana@corelog.test',
  role: 'user',
  teamId: null,
  displayName: '',
  jobTitle: '',
  department: '',
  avatarKey: null,
  timezone: 'UTC',
  createdAt: '2026-01-10T00:00:00Z',
  lastLoginAt: null,
  emailVerifiedAt: null,
};

interface Mocks {
  readonly uploadMyAvatar: ReturnType<typeof vi.fn>;
  readonly deleteMyAvatar: ReturnType<typeof vi.fn>;
  readonly getMe: ReturnType<typeof vi.fn>;
}

function setup(): Mocks {
  const mocks: Mocks = {
    uploadMyAvatar: vi.fn(() => of('/users/user-1/avatar')),
    deleteMyAvatar: vi.fn(() => of(undefined)),
    getMe: vi.fn(() => of({ ...user, avatarKey: 'k-1' })),
  };
  TestBed.configureTestingModule({
    providers: [
      ...provideCorelogTesting(),
      {
        provide: AvatarApiService,
        useValue: {
          getAvatar: vi.fn(() => of(null)),
          uploadMyAvatar: mocks.uploadMyAvatar,
          deleteMyAvatar: mocks.deleteMyAvatar,
        },
      },
      { provide: PeopleApiService, useValue: { getMe: mocks.getMe } },
    ],
  });
  return mocks;
}

function create(current: User = user): ComponentFixture<ProfileAvatarUploader> {
  const fixture = TestBed.createComponent(ProfileAvatarUploader);
  fixture.componentRef.setInput('user', current);
  fixture.detectChanges();
  return fixture;
}

function image(name: string, type: string, size: number): File {
  const file = new File(['x'], name, { type });
  Object.defineProperty(file, 'size', { value: size });
  return file;
}

function pick(fixture: ComponentFixture<ProfileAvatarUploader>, file: File): void {
  const input = fixture.nativeElement.querySelector('input[type="file"]') as HTMLInputElement;
  Object.defineProperty(input, 'files', { value: [file], configurable: true });
  input.dispatchEvent(new Event('change'));
  fixture.detectChanges();
}

function buttonNamed(
  fixture: ComponentFixture<ProfileAvatarUploader>,
  name: string,
): HTMLButtonElement | undefined {
  const buttons = Array.from(
    fixture.nativeElement.querySelectorAll('button') as NodeListOf<HTMLButtonElement>,
  );
  return buttons.find((button) => button.textContent?.trim() === name);
}

/** Polls until the text shows up; the messages follow the asynchronous upload or removal. */
async function expectText(
  fixture: ComponentFixture<ProfileAvatarUploader>,
  text: string,
): Promise<void> {
  await vi.waitFor(() => {
    fixture.detectChanges();
    expect(fixture.nativeElement.textContent).toContain(text);
  });
}

describe('AvatarUploader', () => {
  beforeEach(() => {
    TestBed.resetTestingModule();
    URL.createObjectURL = vi.fn(() => 'blob:preview');
    URL.revokeObjectURL = vi.fn();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('rejects a file that is not PNG, JPEG or WebP without uploading it', async () => {
    const mocks = setup();
    const fixture = create();

    pick(fixture, image('notas.gif', 'image/gif', 1000));

    await expectText(fixture, 'Formato no válido');
    expect(buttonNamed(fixture, 'Guardar foto')).toBeUndefined();
    expect(mocks.uploadMyAvatar).not.toHaveBeenCalled();
  });

  it('rejects a file larger than 2 MB', async () => {
    const mocks = setup();
    const fixture = create();

    pick(fixture, image('grande.png', 'image/png', 2 * 1024 * 1024 + 1));

    await expectText(fixture, 'supera los 2 MB');
    expect(mocks.uploadMyAvatar).not.toHaveBeenCalled();
  });

  it('shows a preview, uploads the chosen file and refreshes the user', async () => {
    const mocks = setup();
    const fixture = create();
    const userChange = vi.fn();
    fixture.componentInstance.userChange.subscribe(userChange);

    const file = image('foto.png', 'image/png', 1024);
    pick(fixture, file);
    await expectText(fixture, 'Vista previa');

    buttonNamed(fixture, 'Guardar foto')?.click();

    await vi.waitFor(() => expect(mocks.uploadMyAvatar).toHaveBeenCalledWith(file));
    await vi.waitFor(() =>
      expect(userChange).toHaveBeenCalledWith(expect.objectContaining({ avatarKey: 'k-1' })),
    );
  });

  it('maps the server code user.avatar_too_large to a Spanish message', async () => {
    const mocks = setup();
    mocks.uploadMyAvatar.mockReturnValueOnce(
      throwError(() => ({
        status: 413,
        code: 'user.avatar_too_large',
        message: 'too big',
        details: [],
      })),
    );
    const fixture = create();

    pick(fixture, image('foto.png', 'image/png', 1024));
    await expectText(fixture, 'Guardar foto');
    buttonNamed(fixture, 'Guardar foto')?.click();

    await expectText(fixture, 'La imagen supera los 2 MB. Elige una más ligera.');
  });

  it('maps the server code user.avatar_invalid to a Spanish message', async () => {
    const mocks = setup();
    mocks.uploadMyAvatar.mockReturnValueOnce(
      throwError(() => ({ status: 422, code: 'user.avatar_invalid', message: 'bad', details: [] })),
    );
    const fixture = create();

    pick(fixture, image('foto.webp', 'image/webp', 1024));
    await expectText(fixture, 'Guardar foto');
    buttonNamed(fixture, 'Guardar foto')?.click();

    await expectText(fixture, 'La imagen no es válida. Usa un PNG, JPEG o WebP.');
  });

  it('removes the current photo when the user has one', async () => {
    const mocks = setup();
    const fixture = create({ ...user, avatarKey: 'k-1' });
    const userChange = vi.fn();
    fixture.componentInstance.userChange.subscribe(userChange);

    buttonNamed(fixture, 'Quitar foto')?.click();

    await vi.waitFor(() => expect(mocks.deleteMyAvatar).toHaveBeenCalled());
    await vi.waitFor(() => expect(userChange).toHaveBeenCalled());
  });

  it('does not offer removal when there is no photo', () => {
    setup();
    const fixture = create();
    expect(buttonNamed(fixture, 'Quitar foto')).toBeUndefined();
  });
});
