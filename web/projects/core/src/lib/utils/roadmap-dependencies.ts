/** Rectangle of a bar inside the timeline canvas, in pixels. */
export interface BarRect {
  readonly x: number;
  readonly y: number;
  readonly width: number;
  readonly height: number;
}

export interface DependencyPath {
  readonly from: string;
  readonly to: string;
  /** SVG path data of the connector. */
  readonly d: string;
  /** Points of the arrow head at the target end. */
  readonly arrow: string;
}

const STUB = 10;
const ARROW = 5;

/**
 * Elbow connector from the end of the blocking bar to the start of the blocked bar. When the blocked bar
 * starts before the blocking one ends, the connector loops around so it never crosses the bars.
 */
export function dependencyPath(from: BarRect, to: BarRect): { d: string; arrow: string } {
  const x1 = from.x + from.width;
  const y1 = from.y + from.height / 2;
  const x2 = to.x;
  const y2 = to.y + to.height / 2;
  const arrow = `${x2},${y2} ${x2 - ARROW - 2},${y2 - ARROW} ${x2 - ARROW - 2},${y2 + ARROW}`;
  if (x2 >= x1 + STUB * 2) {
    const mid = x1 + STUB;
    return { d: `M${x1},${y1} H${mid} V${y2} H${x2 - ARROW - 2}`, arrow };
  }
  // Loop: leave to the right, drop to the gap between the rows, return to the left of the target.
  const gapY =
    y2 > y1 ? from.y + from.height + (to.y - (from.y + from.height)) / 2 : y1 - from.height;
  const rightX = x1 + STUB;
  const leftX = x2 - STUB;
  return {
    d: `M${x1},${y1} H${rightX} V${gapY} H${leftX} V${y2} H${x2 - ARROW - 2}`,
    arrow,
  };
}

/**
 * Connectors of the `blocks` dependencies whose two bars are drawn. Dependencies with a missing bar
 * (filtered out, undated or outside the range) are skipped.
 */
export function computeDependencyPaths(
  dependencies: readonly { readonly from: string; readonly to: string; readonly kind: string }[],
  rects: ReadonlyMap<string, BarRect>,
): DependencyPath[] {
  const result: DependencyPath[] = [];
  for (const dependency of dependencies) {
    if (dependency.kind !== 'blocks' || dependency.from === dependency.to) continue;
    const from = rects.get(dependency.from);
    const to = rects.get(dependency.to);
    if (!from || !to) continue;
    result.push({ from: dependency.from, to: dependency.to, ...dependencyPath(from, to) });
  }
  return result;
}
