import { DatePipe, NgTemplateOutlet } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  LOCALE_ID,
  TemplateRef,
  computed,
  inject,
  input,
  model,
  output,
} from '@angular/core';
import { RouterLink } from '@angular/router';
import {
  BreakpointService,
  issueStatusView,
  IssueStatusCategory,
  Issue,
  issueKeyOf,
  shortIssueKeyOf,
  issuePath,
} from '@corelog/core';
import {
  DataTable,
  DataTableCell,
  DataTableColumn,
  DataTablePageChange,
  DataTablePagination,
  RowMenuItem,
} from '@corelog/ui/data-table';
import { BulkAction } from '@corelog/ui/bulk-bar';
import { IssueKey } from '@corelog/ui/issue-key';
import { Lozenge } from '@corelog/ui/lozenge';
import { PriorityTag } from '@corelog/ui/priority-tag';
import { AppTooltip } from '@corelog/ui/tooltip';
import { UserChip } from '@corelog/ui/user-chip';

import { relativeTime } from './relative-time';

/** Below this width the updated-date column is left out so the remaining columns fit. */
const NARROW_TABLE_QUERY = '(max-width: 1023.98px)';

const ALL_COLUMNS: DataTableColumn[] = [
  { key: 'key', title: 'Clave', hideable: true },
  { key: 'summary', title: 'Resumen', field: 'summary', ellipsis: true },
  { key: 'type', title: 'Tipo', hideable: true },
  { key: 'status', title: 'Estado', hideable: true },
  { key: 'priority', title: 'Prioridad', hideable: true },
  { key: 'assignee', title: 'Responsable', hideable: true },
  { key: 'reporter', title: 'Informante', hideable: true },
  { key: 'project', title: 'Proyecto', hideable: true },
  { key: 'labels', title: 'Etiquetas', hideable: true },
  { key: 'points', title: 'Puntos', hideable: true, align: 'right' },
  { key: 'due', title: 'Vencimiento', hideable: true },
  { key: 'created', title: 'Creada', hideable: true },
  { key: 'updated', title: 'Actualizado', hideable: true },
];

/** The columns shown when the caller does not choose them. */
const DEFAULT_COLUMN_KEYS: readonly string[] = [
  'key',
  'summary',
  'status',
  'priority',
  'assignee',
  'updated',
];

/** Issue list on DataTable: a table on desktop, cards below 768 px. */
@Component({
  selector: 'app-issue-list',
  imports: [
    DataTable,
    DataTableCell,
    AppTooltip,
    DatePipe,
    IssueKey,
    Lozenge,
    NgTemplateOutlet,
    PriorityTag,
    RouterLink,
    UserChip,
  ],
  templateUrl: './issue-list.html',
  styleUrl: './issue-list.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class IssueList {
  private readonly datePipe = new DatePipe(inject(LOCALE_ID));
  private readonly breakpoints = inject(BreakpointService);
  private readonly narrow = this.breakpoints.matches(NARROW_TABLE_QUERY);

  readonly issues = input.required<Issue[]>();
  readonly loading = input(false);
  readonly error = input<string | null>(null);
  readonly pagination = input<DataTablePagination | null>(null);
  readonly emptyState = input<TemplateRef<void> | null>(null);
  readonly rowActions = input<((row: Issue) => RowMenuItem[]) | null>(null);
  /** The table grows to the remaining height of its section. */
  readonly fill = input(false);
  /** Rows can be ticked; the bulk actions show above the table while any is. */
  readonly selectable = input(false);
  readonly selectedKeys = model<string[]>([]);
  readonly bulkActions = input<BulkAction[]>([]);
  /** Keys of the columns to show, in table order. The caller owns the choice, so the table has no chooser. */
  readonly columnKeys = input<readonly string[] | null>(null);

  readonly open = output<Issue>();
  readonly retry = output<void>();
  readonly pageChange = output<DataTablePageChange>();

  protected readonly columns = computed<DataTableColumn[]>(() => {
    const chosen = this.columnKeys();
    const wanted = new Set(chosen ?? DEFAULT_COLUMN_KEYS);
    return ALL_COLUMNS.filter(
      (column) =>
        wanted.has(column.key) && !(chosen === null && this.narrow() && column.key === 'updated'),
    ).map((column) => (chosen === null ? column : { ...column, hideable: false }));
  });

  protected statusLabel(issue: Issue): string {
    return issueStatusView(issue).label;
  }

  protected statusCategory(issue: Issue): IssueStatusCategory {
    return issueStatusView(issue).category;
  }

  protected updatedText(issue: Issue): string {
    return (
      relativeTime(issue.updatedAt) ??
      this.datePipe.transform(issue.updatedAt, 'dd/MM/yyyy') ??
      issue.updatedAt
    );
  }

  protected shortKeyOf(issue: Issue): string {
    return shortIssueKeyOf(issue);
  }

  protected keyOf(issue: Issue): string {
    return issueKeyOf(issue);
  }

  protected pathOf(issue: Issue): string {
    return issuePath(issue.key);
  }

  protected dateText(value: string | null): string {
    return value ? (this.datePipe.transform(value, 'dd/MM/yyyy') ?? value) : '';
  }

  protected labelsText(issue: Issue): string {
    return ((issue as { labels?: readonly string[] }).labels ?? []).join(', ');
  }

  protected isResolved(issue: Issue): boolean {
    return issue.status === 'resolved' || issue.status === 'closed';
  }
}
