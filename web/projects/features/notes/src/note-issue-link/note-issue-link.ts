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
import { RouterLink } from '@angular/router';
import { Issue, IssueApiService, issuePath, shortIssueKeyOf } from '@corelog/core';
import { AppTooltip } from '@corelog/ui/tooltip';
import { AutoComplete } from 'primeng/autocomplete';
import { Subscription } from 'rxjs';

const SEARCH_PAGE_SIZE = 10;

interface IssueOption {
  readonly id: string;
  readonly key: string;
  readonly summary: string;
  readonly label: string;
}

function toOption(issue: Issue): IssueOption {
  const key = shortIssueKeyOf(issue);
  return { id: issue.id, key, summary: issue.summary, label: `${key} · ${issue.summary}` };
}

/**
 * Link of a note to an issue: shows the linked issue (key and summary, with a link to it) or a
 * search field that finds issues by key or summary through the issue API.
 */
@Component({
  selector: 'app-note-issue-link',
  imports: [AppTooltip, AutoComplete, FormsModule, RouterLink],
  templateUrl: './note-issue-link.html',
  styleUrl: './note-issue-link.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class NoteIssueLink {
  private readonly api = inject(IssueApiService);
  private readonly destroyRef = inject(DestroyRef);

  /** Id of the linked issue, or null. */
  readonly issueId = model<string | null>(null);
  readonly disabled = input(false);

  protected readonly options = signal<IssueOption[]>([]);
  protected readonly loading = signal(false);
  protected readonly selected = signal<IssueOption | null>(null);
  protected readonly lookupFailed = signal(false);
  protected readonly path = computed(() => {
    const current = this.selected();
    return current ? issuePath(current.key) : null;
  });

  private request: Subscription | undefined;
  private lookup: Subscription | undefined;

  constructor() {
    // Resolves the key and summary of the issue the note points at.
    effect(() => {
      const id = this.issueId();
      untracked(() => this.resolve(id));
    });
    this.destroyRef.onDestroy(() => {
      this.request?.unsubscribe();
      this.lookup?.unsubscribe();
    });
  }

  protected search(query: string): void {
    this.request?.unsubscribe();
    this.loading.set(true);
    this.request = this.api
      .list({ page: 1, pageSize: SEARCH_PAGE_SIZE, query: query.trim() || undefined })
      .subscribe({
        next: (page) => {
          this.options.set(page.items.map(toOption));
          this.loading.set(false);
        },
        error: () => {
          this.options.set([]);
          this.loading.set(false);
        },
      });
  }

  protected pick(option: IssueOption | null): void {
    if (!option || typeof option !== 'object') return;
    this.selected.set(option);
    this.issueId.set(option.id);
  }

  protected unlink(): void {
    this.selected.set(null);
    this.issueId.set(null);
  }

  private resolve(id: string | null): void {
    this.lookup?.unsubscribe();
    this.lookupFailed.set(false);
    if (!id) {
      this.selected.set(null);
      return;
    }
    if (this.selected()?.id === id) return;
    this.lookup = this.api.getById(id).subscribe({
      next: (issue) => this.selected.set(toOption(issue)),
      error: () => {
        this.selected.set(null);
        this.lookupFailed.set(true);
      },
    });
  }
}
