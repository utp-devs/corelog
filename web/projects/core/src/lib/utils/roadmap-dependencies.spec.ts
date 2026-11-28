import { computeDependencyPaths, dependencyPath, type BarRect } from './roadmap-dependencies';

const rect = (x: number, y: number, width = 100): BarRect => ({ x, y, width, height: 20 });

describe('roadmap dependencies', () => {
  it('draws an elbow from the end of the blocker to the start of the blocked bar', () => {
    const { d, arrow } = dependencyPath(rect(0, 0), rect(200, 40));
    expect(d.startsWith('M100,10 H110 V50')).toBe(true);
    expect(arrow.startsWith('200,50')).toBe(true);
  });

  it('loops around when the blocked bar starts before the blocker ends', () => {
    const { d } = dependencyPath(rect(0, 0), rect(50, 40));
    expect(d.split(' ').filter((part) => part.startsWith('V')).length).toBe(2);
  });

  it('keeps only blocks links whose two bars are drawn', () => {
    const rects = new Map([
      ['a', rect(0, 0)],
      ['b', rect(200, 40)],
    ]);
    const paths = computeDependencyPaths(
      [
        { from: 'a', to: 'b', kind: 'blocks' },
        { from: 'a', to: 'missing', kind: 'blocks' },
        { from: 'a', to: 'b', kind: 'relates' },
        { from: 'a', to: 'a', kind: 'blocks' },
      ],
      rects,
    );
    expect(paths.map((p) => `${p.from}>${p.to}`)).toEqual(['a>b']);
  });
});
