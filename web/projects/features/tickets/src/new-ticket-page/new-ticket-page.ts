import { HttpErrorResponse } from '@angular/common/http';
import { ChangeDetectionStrategy, Component, Signal, inject, signal } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { Router } from '@angular/router';
import { map } from 'rxjs';
import {
  NotificationService,
  TicketStore,
  asApiError,
  isApiError,
  warnBeforeUnload,
  ISSUES_PATH,
  issuePath,
} from '@corelog/core';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader } from '@corelog/ui/page-header';
import { Section } from '@corelog/ui/section';

import { MentionSearch } from '../shared/mention-search';
import { TicketForm, createTicketForm } from '../ticket-form/ticket-form';

const CREATE_FAILED = 'No se pudo crear la incidencia';

/** Message of a refused creation: the server message, the error text, or a generic fallback. */
function creationErrorMessage(error: unknown): string {
  if (isApiError(error) || error instanceof HttpErrorResponse) {
    return asApiError(error).message || CREATE_FAILED;
  }
  return error instanceof Error && error.message ? error.message : CREATE_FAILED;
}

/**
 * Page to register a new issue. Leaving with typed values asks for confirmation (unsavedChangesGuard
 * on the route). A successful create clears the form before navigating to the new issue.
 */
@Component({
  selector: 'app-new-ticket-page',
  imports: [PageContainer, PageHeader, Section, TicketForm],
  templateUrl: './new-ticket-page.html',
  styleUrl: './new-ticket-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class NewTicketPage {
  private readonly store = inject(TicketStore);
  private readonly router = inject(Router);
  private readonly notify = inject(NotificationService);
  private readonly mentions = inject(MentionSearch);

  protected readonly form = createTicketForm();

  /** True while the form holds edits that have not been saved (read by the unsaved-changes guard). */
  readonly dirty: Signal<boolean> = toSignal(this.form.events.pipe(map(() => this.form.dirty)), {
    initialValue: false,
  });

  protected readonly isSubmitting = signal(false);
  protected readonly searchUsers = this.mentions.search;

  constructor() {
    warnBeforeUnload(this.dirty);
  }

  protected submit(): void {
    if (this.isSubmitting()) return;
    const values = this.form.getRawValue();
    this.isSubmitting.set(true);
    this.store
      .createTicket({
        title: values.title,
        description: values.description,
        priority: values.priority,
        issueType: values.issueType,
      })
      .subscribe({
        next: (ticket) => {
          this.isSubmitting.set(false);
          this.notify.success('Incidencia creada');
          // Reset first so the unsaved-changes guard sees a clean form when the navigation starts.
          this.form.reset({ title: '', description: '', priority: 'medium', issueType: 'task' });
          void this.router.navigateByUrl(issuePath(ticket.id));
        },
        error: (error: unknown) => {
          this.isSubmitting.set(false);
          this.notify.error(creationErrorMessage(error));
        },
      });
  }

  protected cancel(): void {
    void this.router.navigateByUrl(ISSUES_PATH);
  }
}
