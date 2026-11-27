import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  inject,
  input,
  output,
  signal,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormsModule } from '@angular/forms';
import { Button } from 'primeng/button';
import { Message } from 'primeng/message';
import { Select } from 'primeng/select';
import {
  InvitationCreated,
  NotificationService,
  OrganizationStore,
  ROLE_LABELS,
  OrgRole,
  isApiError,
} from '@corelog/core';
import { FieldRow } from '@corelog/ui/field-row';

import { copyText } from '../shared/copy-text';
import { InviteRow, InviteRowErrors, validateInviteRows } from './invite-rows';

const INVITABLE_ROLE_OPTIONS: { value: OrgRole; label: string }[] = [
  { value: 'admin', label: ROLE_LABELS.admin },
  { value: 'member', label: ROLE_LABELS.member },
  { value: 'guest', label: ROLE_LABELS.guest },
];

/** Wizard step 3 (optional): invitation emails with roles. No email is sent; the accept links are shown to copy. */
@Component({
  selector: 'app-invite-step',
  imports: [Button, Message, Select, FieldRow, FormsModule],
  templateUrl: './invite-step.html',
  styleUrl: './invite-step.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class InviteStep {
  private readonly orgs = inject(OrganizationStore);
  private readonly notify = inject(NotificationService);
  private readonly destroyRef = inject(DestroyRef);
  private nextId = 2;

  readonly orgId = input.required<string>();
  /** Shows the "Atrás" action; the wizard passes it only when there is a previous step. */
  readonly showBack = input(false);
  readonly back = output<void>();
  readonly continued = output<void>();

  protected readonly roleOptions = INVITABLE_ROLE_OPTIONS;
  protected readonly rows = signal<InviteRow[]>([{ id: 1, email: '', role: 'member' }]);
  protected readonly rowErrors = signal<InviteRowErrors>({});
  protected readonly submitting = signal(false);
  protected readonly serverError = signal<string | null>(null);
  protected readonly created = signal<InvitationCreated[] | null>(null);

  protected roleLabel(role: OrgRole): string {
    return ROLE_LABELS[role];
  }

  protected updateRow(id: number, patch: Partial<Pick<InviteRow, 'email' | 'role'>>): void {
    this.rows.update((current) =>
      current.map((row) => (row.id === id ? { ...row, ...patch } : row)),
    );
  }

  protected onEmailInput(id: number, event: Event): void {
    this.updateRow(id, { email: (event.target as HTMLInputElement).value });
  }

  protected removeRow(id: number): void {
    this.rows.update((current) =>
      current.length > 1 ? current.filter((row) => row.id !== id) : current,
    );
  }

  protected addRow(): void {
    this.rows.update((current) => [...current, { id: this.nextId, email: '', role: 'member' }]);
    this.nextId += 1;
  }

  protected onSubmit(event: Event): void {
    event.preventDefault();
    this.submit();
  }

  protected submit(): void {
    this.serverError.set(null);
    const { items, errors } = validateInviteRows(this.rows());
    this.rowErrors.set(errors);
    if (Object.keys(errors).length > 0) return;
    if (items.length === 0) {
      this.continued.emit();
      return;
    }

    this.submitting.set(true);
    this.orgs
      .createInvitations(this.orgId(), items)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (list) => {
          this.submitting.set(false);
          this.created.set(list);
        },
        error: (error: unknown) => {
          this.submitting.set(false);
          this.serverError.set(
            isApiError(error)
              ? error.message
              : 'No se pudieron crear las invitaciones. Inténtalo de nuevo.',
          );
        },
      });
  }

  protected async copyLink(url: string): Promise<void> {
    if (await copyText(url)) {
      this.notify.success('Enlace copiado');
    } else {
      this.notify.error('No se pudo copiar el enlace. Selecciónalo y cópialo manualmente.');
    }
  }
}
