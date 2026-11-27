import { DatePipe, NgTemplateOutlet } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  TemplateRef,
  computed,
  inject,
  input,
  output,
} from '@angular/core';
import {
  BreakpointService,
  ISSUE_STATUS_META,
  IssueStatusCategory,
  Ticket,
  issueKeyOf,
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
import { IssueKey } from '@corelog/ui/issue-key';
import { Lozenge } from '@corelog/ui/lozenge';
import { PriorityTag } from '@corelog/ui/priority-tag';
import { UserChip } from '@corelog/ui/user-chip';

/** Below this width the updated-date column is left out so the remaining columns fit. */
const NARROW_TABLE_QUERY = '(max-width: 1023.98px)';

const ALL_COLUMNS: DataTableColumn[] = [
  { key: 'key', title: 'Clave' },
  { key: 'summary', title: 'Resumen' },
  { key: 'status', title: 'Estado' },
  { key: 'priority', title: 'Prioridad', hideable: true },
  { key: 'assignee', title: 'Responsable', hideable: true },
  { key: 'updated', title: 'Actualizado', hideable: true },
];

/** Issue list on DataTable: a table on desktop, cards below 768 px. */
@Component({
  selector: 'app-ticket-list',
  imports: [
    DataTable,
    DataTableCell,
    DatePipe,
    IssueKey,
    Lozenge,
    NgTemplateOutlet,
    PriorityTag,
    UserChip,
  ],
  templateUrl: './ticket-list.html',
  styleUrl: './ticket-list.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TicketList {
  private readonly breakpoints = inject(BreakpointService);
  private readonly narrow = this.breakpoints.matches(NARROW_TABLE_QUERY);

  readonly tickets = input.required<Ticket[]>();
  readonly loading = input(false);
  readonly error = input<string | null>(null);
  readonly pagination = input<DataTablePagination | null>(null);
  readonly emptyState = input<TemplateRef<void> | null>(null);
  readonly rowActions = input<((row: Ticket) => RowMenuItem[]) | null>(null);
  /** The table grows to the remaining height of its section. */
  readonly fill = input(false);

  readonly open = output<Ticket>();
  readonly retry = output<void>();
  readonly pageChange = output<DataTablePageChange>();

  protected readonly columns = computed<DataTableColumn[]>(() =>
    this.narrow() ? ALL_COLUMNS.filter((column) => column.key !== 'updated') : ALL_COLUMNS,
  );

  protected statusLabel(ticket: Ticket): string {
    return ISSUE_STATUS_META[ticket.status].label;
  }

  protected statusCategory(ticket: Ticket): IssueStatusCategory {
    return ISSUE_STATUS_META[ticket.status].category;
  }

  protected keyOf(ticket: Ticket): string {
    return issueKeyOf(ticket.id);
  }

  protected pathOf(ticket: Ticket): string {
    return issuePath(ticket.id);
  }

  protected isResolved(ticket: Ticket): boolean {
    return ticket.status === 'resolved' || ticket.status === 'closed';
  }
}
