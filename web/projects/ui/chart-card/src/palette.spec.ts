import { CHART_SERIES, resolveTokenColor, seriesColor } from './palette';

describe('chart palette', () => {
  it('returns token references only and cycles through the series', () => {
    for (const color of CHART_SERIES) {
      expect(color).toMatch(/^var\(--cl-[a-z-]+\)$/);
    }
    expect(seriesColor(0)).toBe(seriesColor(CHART_SERIES.length));
    expect(seriesColor(-1)).toBe(CHART_SERIES[CHART_SERIES.length - 1]);
  });

  it('resolves a token reference to the color the scope computes', () => {
    const root = document.documentElement;
    root.style.setProperty('--cl-brand', '#0052cc');
    try {
      expect(resolveTokenColor('var(--cl-brand)', root)).toBe('#0052cc');
    } finally {
      root.style.removeProperty('--cl-brand');
    }
  });

  it('returns plain values unchanged', () => {
    expect(resolveTokenColor('#123456', document.documentElement)).toBe('#123456');
  });
});
