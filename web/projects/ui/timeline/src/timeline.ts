import {
  CdkDrag,
  CdkDragDrop,
  CdkDragHandle,
  CdkDragPreview,
  CdkDropList,
} from '@angular/cdk/drag-drop';
import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import {
  BarRect,
  DependencyPath,
  TimelineScale,
  barGeometry,
  computeDependencyPaths,
  packLanes,
} from '@corelog/core';
import { AppTooltip } from '@corelog/ui/tooltip';

export type TimelineCategory = 'todo' | 'in_progress' | 'done';

/** One row of the timeline: a titled item with optional dates and progress. */
export interface TimelineRow {
  readonly id: string;
  readonly title: string;
  /** Short text before the title, for example the issue key. */
  readonly key?: string;
  readonly category: TimelineCategory;
  /** 0 to 100. */
  readonly percent: number;
  readonly start: string | null;
  readonly end: string | null;
  /** Text read by assistive technology and shown in the tooltip, for example "12 nov - 3 dic, 40 %". */
  readonly description: string;
}

/** A period drawn above the rows (a version flag or a sprint band). */
export interface TimelineBand {
  readonly id: string;
  readonly label: string;
  readonly kind: 'version' | 'sprint';
  readonly start: string | null;
  readonly end: string | null;
}

export interface TimelineDependency {
  readonly from: string;
  readonly to: string;
  readonly kind: string;
}

/** Height of a row and of a bar, in pixels. The geometry of the dependency arrows depends on them. */
export const TIMELINE_ROW_HEIGHT = 44;
export const TIMELINE_BAR_HEIGHT = 26;
const BAND_HEIGHT = 28;
const BAND_PAD = 8;

interface PlacedBar {
  readonly row: TimelineRow;
  readonly x: number;
  readonly y: number;
  readonly width: number;
  readonly clippedStart: boolean;
  readonly clippedEnd: boolean;
}

interface PlacedBand {
  readonly band: TimelineBand;
  readonly x: number;
  readonly y: number;
  readonly width: number;
}

/**
 * Timeline with a time axis, a sticky column of titles, bars colored by status category with a progress
 * strip, bands for versions and sprints, a line for today and dependency arrows. Rows can be reordered by
 * dragging the handle or with the move buttons. Scrolls horizontally inside its own box.
 */
@Component({
  selector: 'app-timeline',
  imports: [AppTooltip, CdkDrag, CdkDragHandle, CdkDragPreview, CdkDropList],
  templateUrl: './timeline.html',
  styleUrl: './timeline.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class Timeline {
  readonly rows = input.required<readonly TimelineRow[]>();
  readonly scale = input.required<TimelineScale>();
  readonly bands = input<readonly TimelineBand[]>([]);
  readonly dependencies = input<readonly TimelineDependency[]>([]);
  readonly reorderable = input(false);
  /** Accessible name of the scrollable region. */
  readonly label = input('Línea de tiempo');
  /** Title of the first column. */
  readonly columnTitle = input('Épicas');

  /** A row or bar was chosen (click, Enter or Space). */
  readonly rowActivated = output<string>();
  /** The new order of the row ids after a drag or a move button. */
  readonly reordered = output<readonly string[]>();

  protected readonly rowHeight = TIMELINE_ROW_HEIGHT;
  protected readonly barHeight = TIMELINE_BAR_HEIGHT;
  protected readonly bandHeight = BAND_HEIGHT;

  private readonly packedBands = computed(() => packLanes(this.bands(), 2));

  /** Height of the band area, 0 when there are no bands to draw. */
  protected readonly bandsHeight = computed(() => {
    const scale = this.scale();
    const visible = this.packedBands().packed.some(
      ({ item }) => barGeometry(item.start, item.end, scale) !== null,
    );
    const lanes = this.packedBands().lanes;
    return !visible || lanes === 0 ? 0 : lanes * BAND_HEIGHT + BAND_PAD * 2;
  });

  protected readonly placedBands = computed<PlacedBand[]>(() => {
    const scale = this.scale();
    return this.packedBands()
      .packed.map(({ item, lane }) => {
        const geometry = barGeometry(item.start, item.end, scale);
        return geometry
          ? {
              band: item,
              x: geometry.x,
              width: geometry.width,
              y: BAND_PAD + lane * BAND_HEIGHT,
            }
          : null;
      })
      .filter((entry): entry is PlacedBand => entry !== null);
  });

  protected readonly bars = computed<PlacedBar[]>(() => {
    const scale = this.scale();
    const offset = this.bandsHeight();
    const result: PlacedBar[] = [];
    this.rows().forEach((row, index) => {
      const geometry = barGeometry(row.start, row.end, scale);
      if (!geometry) return;
      result.push({
        row,
        x: geometry.x,
        width: geometry.width,
        y: offset + index * TIMELINE_ROW_HEIGHT + (TIMELINE_ROW_HEIGHT - TIMELINE_BAR_HEIGHT) / 2,
        clippedStart: geometry.clippedStart,
        clippedEnd: geometry.clippedEnd,
      });
    });
    return result;
  });

  protected readonly paths = computed<DependencyPath[]>(() => {
    const rects = new Map<string, BarRect>(
      this.bars().map((bar) => [
        bar.row.id,
        { x: bar.x, y: bar.y, width: bar.width, height: TIMELINE_BAR_HEIGHT },
      ]),
    );
    return computeDependencyPaths(this.dependencies(), rects);
  });

  protected readonly canvasHeight = computed(
    () => this.bandsHeight() + this.rows().length * TIMELINE_ROW_HEIGHT,
  );

  protected drop(event: CdkDragDrop<readonly TimelineRow[]>): void {
    if (event.previousIndex === event.currentIndex) return;
    const ids = this.rows().map((row) => row.id);
    const [moved] = ids.splice(event.previousIndex, 1);
    ids.splice(event.currentIndex, 0, moved);
    this.reordered.emit(ids);
  }

  protected move(index: number, delta: -1 | 1): void {
    const target = index + delta;
    const ids = this.rows().map((row) => row.id);
    if (target < 0 || target >= ids.length) return;
    [ids[index], ids[target]] = [ids[target], ids[index]];
    this.reordered.emit(ids);
  }
}
