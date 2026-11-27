/** Id of the virtual node that stands for "any status" in the diagram. */
export const ANY_NODE_ID = '*';

export interface LayoutNodeInput {
  readonly id: string;
  readonly label: string;
}

export interface LayoutEdgeInput {
  readonly id: string;
  /** Source node ids; empty means any status. */
  readonly from: readonly string[];
  readonly to: string;
  readonly label: string;
}

export interface LayoutNode {
  readonly id: string;
  readonly label: string;
  readonly x: number;
  readonly y: number;
  readonly width: number;
  readonly height: number;
  readonly column: number;
  readonly virtual: boolean;
}

export interface LayoutEdge {
  readonly id: string;
  readonly from: string;
  readonly to: string;
  readonly label: string;
  /** SVG path data of the curve, ending at the target border (the arrowhead is a marker). */
  readonly path: string;
  readonly labelX: number;
  readonly labelY: number;
  /** True when the edge goes back to an earlier or the same column. */
  readonly backward: boolean;
}

export interface WorkflowLayout {
  readonly nodes: readonly LayoutNode[];
  readonly edges: readonly LayoutEdge[];
  readonly width: number;
  readonly height: number;
}

export interface LayoutOptions {
  readonly nodeWidth: number;
  readonly nodeHeight: number;
  readonly columnGap: number;
  readonly rowGap: number;
  readonly padding: number;
}

export const DEFAULT_LAYOUT_OPTIONS: LayoutOptions = {
  nodeWidth: 156,
  nodeHeight: 48,
  columnGap: 112,
  rowGap: 40,
  padding: 56,
};

/** Room above and below the nodes for the curves that go back to an earlier column. */
const ARC_LIFT = 52;

interface Point {
  readonly x: number;
  readonly y: number;
}

function bezierPoint(p0: Point, p1: Point, p2: Point, p3: Point, t: number): Point {
  const u = 1 - t;
  const a = u * u * u;
  const b = 3 * u * u * t;
  const c = 3 * u * t * t;
  const d = t * t * t;
  return {
    x: a * p0.x + b * p1.x + c * p2.x + d * p3.x,
    y: a * p0.y + b * p1.y + c * p2.y + d * p3.y,
  };
}

function round(value: number): number {
  return Math.round(value * 10) / 10;
}

/**
 * Layered layout of a workflow: one column per breadth-first depth from the initial status, rows in the
 * order the statuses are given, curved edges with a label position. Pure and deterministic: the same input
 * always produces the same output, and no two nodes overlap. Transitions from any status hang from a
 * virtual node in the first column. Statuses that cannot be reached go to a column after the last one.
 */
export function layoutWorkflow(
  nodes: readonly LayoutNodeInput[],
  edges: readonly LayoutEdgeInput[],
  initialId: string,
  options: LayoutOptions = DEFAULT_LAYOUT_OPTIONS,
): WorkflowLayout {
  const known = new Set(nodes.map((node) => node.id));
  const usable = edges.filter(
    (edge) => known.has(edge.to) && edge.from.every((source) => known.has(source)),
  );
  const hasAny = usable.some((edge) => edge.from.length === 0);

  // Adjacency for the breadth-first search; "any" edges are reachable from every status.
  const outgoing = new Map<string, string[]>();
  for (const node of nodes) outgoing.set(node.id, []);
  for (const edge of usable) {
    if (edge.from.length === 0) {
      for (const node of nodes) if (node.id !== edge.to) outgoing.get(node.id)?.push(edge.to);
    } else {
      for (const source of edge.from) outgoing.get(source)?.push(edge.to);
    }
  }

  const depth = new Map<string, number>();
  const start = known.has(initialId) ? initialId : nodes[0]?.id;
  if (start !== undefined) {
    depth.set(start, hasAny ? 1 : 0);
    const queue = [start];
    for (let index = 0; index < queue.length; index += 1) {
      const current = queue[index];
      for (const next of outgoing.get(current) ?? []) {
        if (!depth.has(next)) {
          depth.set(next, (depth.get(current) ?? 0) + 1);
          queue.push(next);
        }
      }
    }
  }
  const reachedMax = Math.max(hasAny ? 1 : 0, ...depth.values());
  for (const node of nodes) {
    if (!depth.has(node.id)) depth.set(node.id, reachedMax + 1);
  }

  const columns = new Map<number, LayoutNodeInput[]>();
  const place = (column: number, node: LayoutNodeInput): void => {
    const list = columns.get(column) ?? [];
    list.push(node);
    columns.set(column, list);
  };
  if (hasAny) place(0, { id: ANY_NODE_ID, label: 'Cualquiera' });
  for (const node of nodes) place(depth.get(node.id) ?? 0, node);

  const columnIndexes = [...columns.keys()].sort((a, b) => a - b);
  const tallest = Math.max(1, ...[...columns.values()].map((list) => list.length));
  const bodyHeight = tallest * options.nodeHeight + (tallest - 1) * options.rowGap;
  const top = options.padding + ARC_LIFT;

  const placed = new Map<string, LayoutNode>();
  columnIndexes.forEach((column, position) => {
    const list = columns.get(column) ?? [];
    const columnHeight = list.length * options.nodeHeight + (list.length - 1) * options.rowGap;
    const offset = (bodyHeight - columnHeight) / 2;
    list.forEach((node, row) => {
      placed.set(node.id, {
        id: node.id,
        label: node.label,
        x: options.padding + position * (options.nodeWidth + options.columnGap),
        y: top + offset + row * (options.nodeHeight + options.rowGap),
        width: options.nodeWidth,
        height: options.nodeHeight,
        column: position,
        virtual: node.id === ANY_NODE_ID,
      });
    });
  });

  const laidEdges: LayoutEdge[] = [];
  for (const edge of usable) {
    const sources = edge.from.length === 0 ? [ANY_NODE_ID] : edge.from;
    sources.forEach((source, index) => {
      const a = placed.get(source);
      const b = placed.get(edge.to);
      if (!a || !b) return;
      laidEdges.push(routeEdge(edge, source, index, a, b, options));
    });
  }

  const columnsUsed = columnIndexes.length;
  const width =
    options.padding * 2 + columnsUsed * options.nodeWidth + (columnsUsed - 1) * options.columnGap;
  const height = top + bodyHeight + options.padding + ARC_LIFT;
  return {
    nodes: [...placed.values()],
    edges: laidEdges,
    width: round(Math.max(width, options.padding * 2 + options.nodeWidth)),
    height: round(height),
  };
}

function routeEdge(
  edge: LayoutEdgeInput,
  source: string,
  index: number,
  a: LayoutNode,
  b: LayoutNode,
  options: LayoutOptions,
): LayoutEdge {
  const id = edge.from.length > 1 ? `${edge.id}:${source}` : edge.id;
  let p0: Point;
  let p1: Point;
  let p2: Point;
  let p3: Point;
  const backward = b.column <= a.column;
  if (!backward) {
    // Leave from the right side and enter the left side; ends spread a little so edges do not stack.
    const spread = (index % 3) * 6 - 6;
    p0 = { x: a.x + a.width, y: a.y + a.height / 2 + spread };
    p3 = { x: b.x, y: b.y + b.height / 2 + spread };
    const reach = Math.max(options.columnGap / 2, (p3.x - p0.x) / 2);
    p1 = { x: p0.x + reach, y: p0.y };
    p2 = { x: p3.x - reach, y: p3.y };
  } else {
    // Go over the top: leave from the upper side of the source, come down on the upper side of the target.
    const lift = ARC_LIFT + (index % 2) * 10;
    p0 = { x: a.x + a.width / 2, y: a.y };
    p3 = { x: b.x + b.width / 2, y: b.y };
    const upper = Math.min(p0.y, p3.y) - lift;
    p1 = { x: p0.x, y: upper };
    p2 = { x: p3.x, y: upper };
  }
  const mid = bezierPoint(p0, p1, p2, p3, 0.5);
  return {
    id,
    from: source,
    to: edge.to,
    label: edge.label,
    path: `M ${round(p0.x)} ${round(p0.y)} C ${round(p1.x)} ${round(p1.y)}, ${round(p2.x)} ${round(p2.y)}, ${round(p3.x)} ${round(p3.y)}`,
    labelX: round(mid.x),
    labelY: round(mid.y),
    backward,
  };
}
