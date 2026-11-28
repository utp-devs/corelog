import { computed, signal } from '@angular/core';
import {
  Dashboard,
  DashboardGadget,
  DashboardLayout,
  GadgetConfig,
  GadgetType,
  MAX_GADGETS,
  addGadget,
  applyLayout,
  cleanGadgetConfig,
  gadgetDefinition,
  gadgetsByColumn,
  layoutColumnCount,
  locateGadget,
  moveGadget,
  newDraftId,
  removeGadget,
  sameGadgets,
  updateGadget,
} from '@corelog/core';

/**
 * Working copy of the layout and the gadgets of a dashboard while it is edited. Nothing reaches the API
 * until the owner saves: `dirty` tells whether the copy differs from the stored dashboard.
 */
export class DashboardEditSession {
  private readonly baselineState = signal<{
    layout: DashboardLayout;
    gadgets: readonly DashboardGadget[];
  }>({ layout: '2', gadgets: [] });

  readonly layout = signal<DashboardLayout>('2');
  readonly gadgets = signal<readonly DashboardGadget[]>([]);

  readonly columns = computed(() => gadgetsByColumn(this.gadgets(), this.layout()));
  readonly columnCount = computed(() => layoutColumnCount(this.layout()));
  readonly canAdd = computed(() => this.gadgets().length < MAX_GADGETS);

  readonly dirty = computed(() => {
    const base = this.baselineState();
    return this.layout() !== base.layout || !sameGadgets(this.gadgets(), base.gadgets);
  });

  /** Starts (or restarts) from the stored dashboard. */
  begin(dashboard: Pick<Dashboard, 'layout' | 'gadgets'>): void {
    const gadgets = applyLayout(dashboard.gadgets, dashboard.layout);
    this.baselineState.set({ layout: dashboard.layout, gadgets });
    this.layout.set(dashboard.layout);
    this.gadgets.set(gadgets);
  }

  /** After a save the stored dashboard is the new baseline. */
  commit(dashboard: Pick<Dashboard, 'layout' | 'gadgets'>): void {
    this.begin(dashboard);
  }

  discard(): void {
    const base = this.baselineState();
    this.layout.set(base.layout);
    this.gadgets.set(base.gadgets);
  }

  setLayout(layout: DashboardLayout): void {
    this.layout.set(layout);
    this.gadgets.update((list) => applyLayout(list, layout));
  }

  /** Adds a gadget of a type with its default configuration to the column with the fewest gadgets. */
  add(type: GadgetType, config?: GadgetConfig, title?: string): DashboardGadget | null {
    if (!this.canAdd()) return null;
    const definition = gadgetDefinition(type);
    const columns = this.columns();
    let target = 0;
    columns.forEach((column, index) => {
      if (column.length < columns[target].length) target = index;
    });
    const draft = {
      id: newDraftId(),
      type,
      title: title?.trim() || definition.label,
      config: cleanGadgetConfig(type, config ?? definition.defaultConfig),
    };
    this.gadgets.update((list) => addGadget(list, this.layout(), draft, target));
    return this.gadgets().find((gadget) => gadget.id === draft.id) ?? null;
  }

  remove(id: string): void {
    this.gadgets.update((list) => removeGadget(list, this.layout(), id));
  }

  update(id: string, title: string, config: GadgetConfig): void {
    const type = this.gadgets().find((gadget) => gadget.id === id)?.type;
    if (!type) return;
    this.gadgets.update((list) =>
      updateGadget(list, id, {
        title: title.trim() || gadgetDefinition(type).label,
        config: cleanGadgetConfig(type, config),
      }),
    );
  }

  move(id: string, column: number, index: number): void {
    this.gadgets.update((list) => moveGadget(list, this.layout(), id, column, index));
  }

  /** Moves a gadget one place up or down inside its column. */
  nudge(id: string, delta: -1 | 1): boolean {
    const spot = locateGadget(this.gadgets(), this.layout(), id);
    if (!spot) return false;
    const size = this.columns()[spot.column].length;
    const index = spot.index + delta;
    if (index < 0 || index >= size) return false;
    this.move(id, spot.column, index);
    return true;
  }

  /** Column and index of a gadget, for announcements. */
  where(id: string): { column: number; index: number } | null {
    return locateGadget(this.gadgets(), this.layout(), id);
  }

  /** The stored gadget the draft started from, or null for a new gadget. */
  stored(id: string): DashboardGadget | null {
    return this.baselineState().gadgets.find((gadget) => gadget.id === id) ?? null;
  }

  /** Whether the data of the gadget on screen would not match what is saved (new or reconfigured). */
  isUnsaved(gadget: DashboardGadget): boolean {
    const stored = this.stored(gadget.id);
    return !stored || JSON.stringify(stored.config) !== JSON.stringify(gadget.config);
  }
}
