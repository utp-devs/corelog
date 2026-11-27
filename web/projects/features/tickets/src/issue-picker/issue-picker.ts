import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  computed,
  effect,
  inject,
  input,
  model,
  signal,
  untracked,
} from '@angular/core';
import { FormsModule } from '@angular/forms';
import { AutoComplete } from 'primeng/autocomplete';
import { Subscription, finalize } from 'rxjs';
import { IssueStatus, Ticket, TicketApiService, issueKeyOf } from '@corelog/core';
import { StatusTag } from '@corelog/ui/status-tag';

import { shortIssueKey } from '../ticket-list/relative-time';

/** Delay before a search is sent, in milliseconds. Keeps one request per pause, not per key. */
export const ISSUE_SEARCH_DEBOUNCE_MS = 300;
const PAGE_SIZE = 20;

interface IssueOption {
  readonly value: string;
  /** Text of the field once chosen: key and title. */
  readonly label: string;
  readonly key: string;
  readonly title: string;
  readonly status: IssueStatus;
}

/**
 * Issue selector with a server search, used to link issues. Opens with the latest issues and lists each
 * result with its key, title and status. The results are shown exactly as the server returns them (no
 * second filter on the client), and the issue being viewed is never offered as its own link target.
 */
@Component({
  selector: 'app-issue-picker',
  imports: [AutoComplete, FormsModule, StatusTag],
  templateUrl: './issue-picker.html',
  styleUrl: './issue-picker.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class IssuePicker {
  private readonly api = inject(TicketApiService);
  private readonly destroyRef = inject(DestroyRef);

  /** Id of the issue being viewed. */
  readonly excludeId = input.required<string>();
  /** Selected issue id, or an empty string when nothing is selected. */
  readonly value = model<string>('');

  protected readonly options = signal<IssueOption[]>([]);
  protected readonly loading = signal(false);
  /** Last option picked by the user; keeps its label when the list no longer contains it. */
  private readonly picked = signal<IssueOption | null>(null);
  /** The option shown in the field: the picked one, else the listed option with the current id. */
  protected readonly selectedOption = computed<IssueOption | null>(() => {
    const id = this.value();
    if (!id) return null;
    const picked = this.picked();
    if (picked?.value === id) return picked;
    return (
      this.options().find((option) => option.value === id) ?? {
        value: id,
        label: shortIssueKey(issueKeyOf(id)),
        key: shortIssueKey(issueKeyOf(id)),
        title: '',
        status: 'open',
      }
    );
  });

  private timer: ReturnType<typeof setTimeout> | undefined;
  private request: Subscription | undefined;

  constructor() {
    // Initial suggestions load when the picker is shown for an issue.
    effect(() => {
      this.excludeId();
      untracked(() => this.search(''));
    });
    this.destroyRef.onDestroy(() => this.cancelPending());
  }

  protected search(query: string): void {
    this.cancelPending();
    this.timer = setTimeout(() => {
      this.loading.set(true);
      this.request = this.api
        .list({ page: 1, pageSize: PAGE_SIZE, query: query.trim() || undefined })
        .pipe(finalize(() => this.loading.set(false)))
        .subscribe({
          next: (page) => this.options.set(this.toOptions(page.items)),
          error: () => this.options.set([]),
        });
    }, ISSUE_SEARCH_DEBOUNCE_MS);
  }

  /** Results come from the server search: the list is shown as returned, without a client-side filter. */
  protected onPick(option: IssueOption | null): void {
    this.picked.set(option);
    this.value.set(option?.value ?? '');
  }

  private toOptions(items: readonly Ticket[]): IssueOption[] {
    return items
      .filter((ticket) => ticket.id !== this.excludeId())
      .map((ticket) => {
        const key = shortIssueKey(issueKeyOf(ticket.id));
        return {
          value: ticket.id,
          label: `${key} · ${ticket.title}`,
          key,
          title: ticket.title,
          status: ticket.status,
        };
      });
  }

  private cancelPending(): void {
    clearTimeout(this.timer);
    this.request?.unsubscribe();
    this.request = undefined;
  }
}
