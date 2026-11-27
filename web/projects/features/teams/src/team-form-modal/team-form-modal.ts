import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  effect,
  input,
  model,
  output,
  signal,
  untracked,
  viewChild,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormControl, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { PrimeTemplate } from 'primeng/api';
import { Button } from 'primeng/button';
import { Dialog } from 'primeng/dialog';
import { InputText } from 'primeng/inputtext';
import { Textarea } from 'primeng/textarea';
import {
  UNSAVED_CANCEL_TEXT,
  UNSAVED_CONFIRM_TEXT,
  UNSAVED_DESCRIPTION,
  UNSAVED_TITLE,
  isApiError,
  resolveAvatarColor,
  warnBeforeUnload,
} from '@corelog/core';
import { ColorPicker } from '@corelog/ui/color-picker';
import { ConfirmModal } from '@corelog/ui/confirm-modal';

import { messageOf } from '../shared/team-messages';
import {
  TEAM_DESCRIPTION_LENGTH_MESSAGE,
  TEAM_DESCRIPTION_MAX_LENGTH,
  TEAM_NAME_HELP,
  TEAM_NAME_LENGTH_MESSAGE,
  TEAM_NAME_TAKEN_MESSAGE,
  TEAM_SAVE_FALLBACK_MESSAGE,
  TeamFormInitial,
  TeamFormValues,
  teamNameValidator,
} from './team-form';

/**
 * Create and edit form of a team, in a dialog. Server errors are shown where they belong: team.name_taken on
 * the name field, any other failure (for example team.creation_not_allowed) as an alert inside the dialog.
 * Closing with unsaved edits asks first. `dirty` tells the page while the open form holds unsaved edits.
 */
@Component({
  selector: 'app-team-form-modal',
  imports: [
    ReactiveFormsModule,
    Button,
    ColorPicker,
    ConfirmModal,
    Dialog,
    InputText,
    PrimeTemplate,
    Textarea,
  ],
  templateUrl: './team-form-modal.html',
  styleUrl: './team-form-modal.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TeamFormModal {
  readonly open = input(false);
  readonly title = input.required<string>();
  readonly initial = input.required<TeamFormInitial>();
  /** Saves the team. Rejects with the normalized ApiError when the server refuses it. */
  readonly onSubmit = input.required<(values: TeamFormValues) => Promise<unknown>>();
  readonly cancelled = output<void>();
  /** True while the open form holds unsaved edits. Two-way bound by the page. */
  readonly dirty = model(false);

  protected readonly unsavedTitle = UNSAVED_TITLE;
  protected readonly unsavedDescription = UNSAVED_DESCRIPTION;
  protected readonly unsavedConfirm = UNSAVED_CONFIRM_TEXT;
  protected readonly unsavedCancel = UNSAVED_CANCEL_TEXT;
  protected readonly nameHelp = TEAM_NAME_HELP;

  protected readonly form = new FormGroup({
    name: new FormControl('', { nonNullable: true, validators: [teamNameValidator] }),
    description: new FormControl('', {
      nonNullable: true,
      validators: [Validators.maxLength(TEAM_DESCRIPTION_MAX_LENGTH)],
    }),
    color: new FormControl<string | null>(null),
  });

  protected readonly submitted = signal(false);
  protected readonly submitting = signal(false);
  protected readonly serverError = signal<string | null>(null);
  protected readonly discardOpen = signal(false);

  private readonly nameInput = viewChild<ElementRef<HTMLInputElement>>('nameInput');

  constructor() {
    // A closed dialog never guards: the flag is cleared when it closes.
    effect(() => {
      const open = this.open();
      untracked(() => {
        if (open) {
          this.resetForm();
        } else {
          this.dirty.set(false);
        }
      });
    });
    this.form.events.pipe(takeUntilDestroyed()).subscribe(() => {
      this.dirty.set(this.open() && this.form.dirty);
    });
    warnBeforeUnload(this.dirty);
  }

  protected nameError(): string | null {
    const control = this.form.controls.name;
    if (control.hasError('server')) return control.getError('server');
    return this.submitted() && control.hasError('length') ? TEAM_NAME_LENGTH_MESSAGE : null;
  }

  protected descriptionError(): string | null {
    const control = this.form.controls.description;
    if (control.hasError('server')) return control.getError('server');
    return this.submitted() && control.hasError('maxlength')
      ? TEAM_DESCRIPTION_LENGTH_MESSAGE
      : null;
  }

  /** Cancel, the close icon and Escape all come through here: unsaved edits ask first. */
  protected requestCancel(): void {
    if (this.form.dirty) {
      this.discardOpen.set(true);
    } else {
      this.cancelled.emit();
    }
  }

  protected discard(): void {
    this.discardOpen.set(false);
    this.cancelled.emit();
  }

  /** Dialog reports a user close (close icon, Escape, backdrop) through its visibility. */
  protected onVisibleChange(visible: boolean): void {
    if (!visible) this.requestCancel();
  }

  protected onDialogShow(): void {
    this.nameInput()?.nativeElement.focus();
  }

  protected async submit(): Promise<void> {
    this.submitted.set(true);
    this.serverError.set(null);
    // Re-runs the validators, which drops server errors from the previous attempt.
    this.form.updateValueAndValidity();
    if (this.form.invalid) return;

    const raw = this.form.getRawValue();
    this.submitting.set(true);
    try {
      await this.onSubmit()({
        name: raw.name.trim(),
        description: raw.description.trim(),
        avatarColor: resolveAvatarColor(raw.color),
      });
    } catch (error) {
      this.showSaveError(error);
    } finally {
      this.submitting.set(false);
    }
  }

  private showSaveError(error: unknown): void {
    if (isApiError(error) && error.code === 'team.name_taken') {
      const name = this.form.controls.name;
      name.setErrors({ ...name.errors, server: TEAM_NAME_TAKEN_MESSAGE });
      name.markAsTouched();
      return;
    }
    this.serverError.set(messageOf(error, TEAM_SAVE_FALLBACK_MESSAGE));
  }

  private resetForm(): void {
    const initial = this.initial();
    this.form.reset({
      name: initial.name,
      description: initial.description,
      color: initial.avatarColor,
    });
    this.submitted.set(false);
    this.serverError.set(null);
    this.discardOpen.set(false);
  }
}
