import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { auditDiff, type AuditEntry } from '@corelog/core';
import { SidePanel } from '@corelog/ui/side-panel';

const CHANGE_LABEL = {
  added: 'Nuevo',
  removed: 'Eliminado',
  changed: 'Cambió',
  same: 'Sin cambios',
} as const;

/** Detail of one audit entry: who, when, from where and what changed, shown as text only. */
@Component({
  selector: 'app-audit-drawer',
  imports: [SidePanel],
  templateUrl: './audit-drawer.html',
  styleUrl: './audit-drawer.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class AuditDrawer {
  readonly entry = input<AuditEntry | null>(null);
  readonly closeRequested = output<void>();

  protected readonly changeLabel = CHANGE_LABEL;
  protected readonly rows = computed(() => {
    const entry = this.entry();
    return entry ? auditDiff(entry.before, entry.after) : [];
  });
  protected readonly changedRows = computed(() =>
    this.rows().filter((row) => row.change !== 'same'),
  );
  protected readonly unchangedCount = computed(
    () => this.rows().length - this.changedRows().length,
  );

  protected formatDate(iso: string): string {
    return new Date(iso).toLocaleString('es-ES', {
      dateStyle: 'full',
      timeStyle: 'medium',
    });
  }
}
