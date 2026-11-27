import { describe, expect, it } from 'vitest';

import { ANY_NODE_ID, LayoutEdgeInput, LayoutNodeInput, layoutWorkflow } from './workflow-layout';

const NODES: LayoutNodeInput[] = [
  { id: 'todo', label: 'Por hacer' },
  { id: 'doing', label: 'En curso' },
  { id: 'review', label: 'Revisión' },
  { id: 'done', label: 'Hecho' },
  { id: 'lost', label: 'Sin camino' },
];

const EDGES: LayoutEdgeInput[] = [
  { id: 'start', from: ['todo'], to: 'doing', label: 'Iniciar' },
  { id: 'review', from: ['doing'], to: 'review', label: 'Revisar' },
  { id: 'finish', from: ['review', 'doing'], to: 'done', label: 'Terminar' },
  { id: 'back', from: ['review'], to: 'doing', label: 'Devolver' },
];

function overlap(
  a: { x: number; y: number; width: number; height: number },
  b: { x: number; y: number; width: number; height: number },
): boolean {
  return a.x < b.x + b.width && b.x < a.x + a.width && a.y < b.y + b.height && b.y < a.y + a.height;
}

describe('layoutWorkflow', () => {
  it('is deterministic: the same input gives the same layout', () => {
    expect(layoutWorkflow(NODES, EDGES, 'todo')).toEqual(layoutWorkflow(NODES, EDGES, 'todo'));
  });

  it('puts every node in the column of its breadth-first depth from the initial status', () => {
    const layout = layoutWorkflow(NODES, EDGES, 'todo');
    const column = (id: string): number | undefined =>
      layout.nodes.find((node) => node.id === id)?.column;
    expect(column('todo')).toBe(0);
    expect(column('doing')).toBe(1);
    expect(column('review')).toBe(2);
    expect(column('done')).toBe(2);
    // A status nobody can reach goes after the last column.
    expect(column('lost')).toBe(3);
  });

  it('never overlaps two nodes', () => {
    const layout = layoutWorkflow(NODES, EDGES, 'todo');
    for (let i = 0; i < layout.nodes.length; i += 1) {
      for (let j = i + 1; j < layout.nodes.length; j += 1) {
        expect(overlap(layout.nodes[i], layout.nodes[j])).toBe(false);
      }
    }
  });

  it('keeps every node inside the drawing', () => {
    const layout = layoutWorkflow(NODES, EDGES, 'todo');
    for (const node of layout.nodes) {
      expect(node.x).toBeGreaterThanOrEqual(0);
      expect(node.y).toBeGreaterThanOrEqual(0);
      expect(node.x + node.width).toBeLessThanOrEqual(layout.width);
      expect(node.y + node.height).toBeLessThanOrEqual(layout.height);
    }
  });

  it('draws one curved edge per source and marks the ones that go back', () => {
    const layout = layoutWorkflow(NODES, EDGES, 'todo');
    // "finish" leaves from two statuses, so it is drawn twice.
    expect(layout.edges).toHaveLength(5);
    expect(
      layout.edges.every((edge) => edge.path.startsWith('M ') && edge.path.includes(' C ')),
    ).toBe(true);
    expect(layout.edges.find((edge) => edge.id === 'back')?.backward).toBe(true);
    expect(layout.edges.find((edge) => edge.id === 'start')?.backward).toBe(false);
  });

  it('hangs transitions from any status on a virtual node in the first column', () => {
    const layout = layoutWorkflow(
      [
        { id: 'a', label: 'A' },
        { id: 'b', label: 'B' },
      ],
      [{ id: 'reopen', from: [], to: 'a', label: 'Reabrir' }],
      'a',
    );
    const virtual = layout.nodes.find((node) => node.id === ANY_NODE_ID);
    expect(virtual?.virtual).toBe(true);
    expect(virtual?.column).toBe(0);
    expect(layout.edges[0].from).toBe(ANY_NODE_ID);
  });

  it('ignores edges that point at unknown statuses and handles an empty workflow', () => {
    expect(layoutWorkflow([], [], '').nodes).toEqual([]);
    const layout = layoutWorkflow(
      NODES,
      [{ id: 'x', from: ['todo'], to: 'ghost', label: 'x' }],
      'todo',
    );
    expect(layout.edges).toEqual([]);
  });
});
