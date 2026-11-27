import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  effect,
  inject,
  input,
  model,
  signal,
  untracked,
} from '@angular/core';
import { FormsModule } from '@angular/forms';
import { Select } from 'primeng/select';
import { Subscription, finalize } from 'rxjs';
import { Ticket, TicketApiService } from '@corelog/core';

/** Delay before a search is sent, in milliseconds. Keeps one request per pause, not per key. */
export const ISSUE_SEARCH_DEBOUNCE_MS = 300;
const PAGE_SIZE = 20;

interface IssueOption {
  readonly value: string;
  readonly label: string;
}

/**
 * Issue selector with a title search, used to link issues. Opens with the latest issues.
 * The issue being viewed is never offered as its own link target.
 */
@Component({
  selector: 'app-issue-picker',
  imports: [FormsModule, Select],
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

  protected onPick(id: string | null): void {
    this.value.set(id ?? '');
  }

  private toOptions(items: readonly Ticket[]): IssueOption[] {
    return items
      .filter((ticket) => ticket.id !== this.excludeId())
      .map((ticket) => ({
        value: ticket.id,
        label: `${ticket.title} · ${ticket.id.slice(0, 8)}`,
      }));
  }

  private cancelPending(): void {
    clearTimeout(this.timer);
    this.request?.unsubscribe();
    this.request = undefined;
  }
}
