import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  input,
  output,
  signal,
  untracked,
} from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { FormControl, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { WebhookApiService, asApiError, type Webhook } from '@corelog/core';
import { CoreqlField } from '@corelog/ui/coreql-field';
import { Dialog } from '@corelog/ui/dialog';
import { SecretReveal } from '@corelog/ui/secret-reveal';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';
import { ToggleSwitch } from 'primeng/toggleswitch';
import { firstValueFrom } from 'rxjs';

import {
  ALL_EVENTS,
  EVENT_GROUPS,
  checkWebhookUrl,
  groupCount,
  isEventSelected,
  toggleAll,
  toggleEvent,
} from './webhook-rules';

export interface ProjectOption {
  readonly id: string;
  readonly name: string;
}

/**
 * Create or edit a webhook. Creating ends in the one-time reveal of the signing secret: the dialog stays open on
 * the reveal and closes only after the person confirms the copy. The project of a webhook cannot change later.
 */
@Component({
  selector: 'app-webhook-dialog',
  imports: [
    Button,
    CoreqlField,
    Dialog,
    InputText,
    ReactiveFormsModule,
    SecretReveal,
    ToggleSwitch,
  ],
  templateUrl: './webhook-dialog.html',
  styleUrl: './webhook-dialog.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class WebhookDialog {
  private readonly api = inject(WebhookApiService);

  readonly open = input(false);
  readonly organizationId = input.required<string>();
  /** Webhook being edited; null creates a new one. */
  readonly webhook = input<Webhook | null>(null);
  readonly projects = input<readonly ProjectOption[]>([]);

  /** A webhook was created or changed. */
  readonly saved = output<Webhook>();
  readonly closed = output<void>();

  protected readonly groups = EVENT_GROUPS;
  protected readonly allEvents = ALL_EVENTS;
  protected readonly events = signal<readonly string[]>([]);
  protected readonly filter = signal('');
  protected readonly saving = signal(false);
  protected readonly error = signal<string | null>(null);
  /** Signing secret of the webhook just created; cleared when the reveal is dismissed. */
  protected readonly secret = signal<string | null>(null);

  protected readonly form = new FormGroup({
    name: new FormControl('', { nonNullable: true, validators: [Validators.required] }),
    url: new FormControl('', { nonNullable: true, validators: [Validators.required] }),
    projectId: new FormControl('', { nonNullable: true }),
    enabled: new FormControl(true, { nonNullable: true }),
  });

  private readonly urlValue = toSignal(this.form.controls.url.valueChanges, { initialValue: '' });
  private readonly nameValue = toSignal(this.form.controls.name.valueChanges, { initialValue: '' });
  protected readonly urlTouched = signal(false);
  protected readonly urlCheck = computed(() => checkWebhookUrl(this.urlValue()));
  protected readonly editing = computed(() => this.webhook() !== null);
  protected readonly canSave = computed(
    () => this.nameValue().trim() !== '' && this.urlCheck().valid && this.events().length > 0,
  );

  constructor() {
    effect(() => {
      if (!this.open()) return;
      const current = this.webhook();
      untracked(() => this.reset(current));
    });
  }

  private reset(current: Webhook | null): void {
    this.form.reset({
      name: current?.name ?? '',
      url: current?.url ?? '',
      projectId: current?.projectId ?? '',
      enabled: current?.enabled ?? true,
    });
    this.events.set(current?.events ?? [ALL_EVENTS]);
    this.filter.set(current?.filterQuery ?? '');
    this.error.set(null);
    this.secret.set(null);
    this.urlTouched.set(false);
  }

  protected selected(value: string): boolean {
    return isEventSelected(this.events(), value);
  }

  protected toggle(value: string): void {
    this.events.update((list) => toggleEvent(list, value));
  }

  protected toggleEverything(): void {
    this.events.update((list) => toggleAll(list));
  }

  protected countOf(index: number): number {
    return groupCount(this.events(), this.groups[index]);
  }

  protected requestClose(): void {
    // While the secret is shown it only closes through the confirmation.
    if (this.secret()) return;
    this.closed.emit();
  }

  protected async save(): Promise<void> {
    if (!this.canSave() || this.saving()) return;
    const value = this.form.getRawValue();
    this.saving.set(true);
    this.error.set(null);
    try {
      const current = this.webhook();
      if (current) {
        const updated = await firstValueFrom(
          this.api.update(this.organizationId(), current.id, {
            name: value.name.trim(),
            url: value.url.trim(),
            events: this.events(),
            filterQuery: this.filter(),
            enabled: value.enabled,
          }),
        );
        this.saved.emit(updated);
        this.closed.emit();
      } else {
        const created = await firstValueFrom(
          this.api.create(this.organizationId(), {
            name: value.name.trim(),
            url: value.url.trim(),
            events: this.events(),
            filterQuery: this.filter(),
            projectId: value.projectId || null,
            enabled: value.enabled,
          }),
        );
        this.saved.emit(created.webhook);
        this.secret.set(created.secret);
      }
    } catch (failure) {
      this.error.set(asApiError(failure).message);
    } finally {
      this.saving.set(false);
    }
  }

  protected onRevealDismissed(): void {
    this.secret.set(null);
    this.closed.emit();
  }
}
