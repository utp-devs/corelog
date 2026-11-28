import { CdkDragHandle } from '@angular/cdk/drag-drop';
import { NgTemplateOutlet } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  computed,
  effect,
  inject,
  input,
  output,
  untracked,
} from '@angular/core';
import {
  AsyncState,
  DashboardGadget,
  DashboardV2ApiService,
  GadgetDataEnvelope,
  gadgetDefinition,
} from '@corelog/core';
import { ChartCard } from '@corelog/ui/chart-card';
import { GadgetFrame, GadgetFrameState } from '@corelog/ui/gadget-frame';
import { RichTextView } from '@corelog/ui/rich-text-view';
import { AppTooltip } from '@corelog/ui/tooltip';
import type { MenuItem } from 'primeng/api';
import { Menu } from 'primeng/menu';

import { GadgetIssues } from './gadget-issues';
import { GadgetLists } from './gadget-lists';
import { GadgetMatrix } from './gadget-matrix';
import { formatClock } from './gadget-format';
import { isChartGadget, toChartModel } from './gadget-chart-model';

/**
 * One gadget of a dashboard: loads its data with the permissions of the viewer, shows the right
 * renderer for its type with loading, error and empty states, and in edit mode the controls to move,
 * configure and remove it.
 */
@Component({
  selector: 'app-gadget-host',
  imports: [
    AppTooltip,
    CdkDragHandle,
    ChartCard,
    GadgetFrame,
    GadgetIssues,
    GadgetLists,
    GadgetMatrix,
    Menu,
    NgTemplateOutlet,
    RichTextView,
  ],
  templateUrl: './gadget-host.html',
  styleUrl: './gadget-host.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class GadgetHost {
  private readonly api = inject(DashboardV2ApiService);
  private readonly read = new AsyncState<GadgetDataEnvelope>();

  readonly dashboardId = input.required<string>();
  readonly gadget = input.required<DashboardGadget>();
  readonly editing = input(false);
  /** The gadget is new or reconfigured and not saved yet, so the API cannot compute it. */
  readonly unsaved = input(false);
  readonly column = input(0);
  readonly columnCount = input(1);
  readonly index = input(0);
  readonly count = input(1);

  readonly configure = output<void>();
  readonly remove = output<void>();
  readonly moveTo = output<number>();
  readonly nudge = output<-1 | 1>();

  protected readonly definition = computed(() => gadgetDefinition(this.gadget().type));
  protected readonly isText = computed(() => this.gadget().type === 'text');
  protected readonly content = computed(() => this.read.value()?.content ?? null);
  protected readonly chart = computed(() => {
    const content = this.content();
    return content ? toChartModel(content) : null;
  });
  protected readonly isChart = computed(() => isChartGadget(this.gadget().type));
  protected readonly markdown = computed(() => {
    const value = this.gadget().config['markdown'];
    return typeof value === 'string' ? value : '';
  });

  protected readonly state = computed<GadgetFrameState>(() => {
    if (this.isText() || this.unsaved()) return 'ready';
    if (this.read.error() && !this.read.hasValue()) return 'error';
    return this.read.hasValue() ? 'ready' : 'loading';
  });
  protected readonly refreshing = computed(() => this.read.loading() && this.read.hasValue());
  protected readonly errorMessage = computed(() =>
    this.read.error()?.status === 403
      ? 'No tienes acceso a los datos de este gadget.'
      : 'No se pudo cargar este gadget. Intenta de nuevo.',
  );
  protected readonly subtitle = computed(() => {
    const at = this.read.value()?.generatedAt;
    return at && !this.editing() ? `Actualizado a las ${formatClock(at)}` : undefined;
  });

  /** Chart cards take over the frame once their data is ready. */
  protected readonly showChartCard = computed(
    () => this.isChart() && this.state() === 'ready' && !this.unsaved() && this.chart() !== null,
  );

  protected readonly menuItems = computed<MenuItem[]>(() => {
    const items: MenuItem[] = [
      { label: 'Configurar', icon: 'pi pi-cog', command: () => this.configure.emit() },
    ];
    if (this.index() > 0) {
      items.push({ label: 'Subir', icon: 'pi pi-arrow-up', command: () => this.nudge.emit(-1) });
    }
    if (this.index() < this.count() - 1) {
      items.push({ label: 'Bajar', icon: 'pi pi-arrow-down', command: () => this.nudge.emit(1) });
    }
    for (let target = 0; target < this.columnCount(); target += 1) {
      if (target === this.column()) continue;
      items.push({
        label: `Mover a la columna ${target + 1}`,
        icon: 'pi pi-arrow-right-arrow-left',
        command: () => this.moveTo.emit(target),
      });
    }
    items.push({ separator: true });
    items.push({
      label: 'Quitar del panel',
      icon: 'pi pi-trash',
      command: () => this.remove.emit(),
    });
    return items;
  });

  constructor() {
    // Moving a gadget replaces its object but not its identity or configuration: no new request.
    const identity = computed(() => {
      const gadget = this.gadget();
      return gadget.id + gadget.type + JSON.stringify(gadget.config);
    });
    effect(() => {
      const dashboardId = this.dashboardId();
      identity();
      const unsaved = this.unsaved();
      untracked(() => {
        const gadget = this.gadget();
        const skip = unsaved || gadget.type === 'text' || gadget.id.startsWith('draft-');
        if (skip) {
          this.read.reset();
          return;
        }
        this.load(dashboardId, gadget);
      });
    });
    inject(DestroyRef).onDestroy(() => this.read.cancel());
  }

  protected refresh(): void {
    this.load(this.dashboardId(), this.gadget());
  }

  private load(dashboardId: string, gadget: DashboardGadget): void {
    this.read.load(this.api.gadgetData(dashboardId, gadget.id, gadget.type));
  }
}
