import { mkdirSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import type { Page } from '@playwright/test';

import { waitForReady } from './wait.ts';

/** Page template declared by the screen. */
export type LayoutTemplate = 'wide' | 'reading' | 'fluid' | 'split' | 'centered';

/** Every number the audit compares against. Tuned once against the reference screens; only tightened afterwards. */
export const AUDIT_THRESHOLDS = {
  /** Grid cell size for the dead-space raster. */
  cellPx: 24,
  /** Largest empty rectangle may cover at most this share of the visible main area. */
  maxEmptyAreaRatio: 0.3,
  /** An empty band wider than this share of main AND taller than emptyRectMinHeightPx fails. */
  emptyRectMinWidthRatio: 0.55,
  emptyRectMinHeightPx: 220,
  /** Empty states (`[data-empty-state]`) may hold a band of this height or less. */
  emptyStateMaxHeightPx: 280,
  /** wide/fluid/split pages must fill this share of main at or above wideBreakpointPx. */
  minWideRatio: 0.85,
  wideBreakpointPx: 1280,
  /** reading pages: min(readingMaxWidthPx, main width) - 2 * gutterPx. */
  readingMaxWidthPx: 1200,
  gutterPx: 24,
  /** Siblings overlapping by no more than this in both axes are not reported. */
  overlapTolerancePx: 2,
  /** Interactive elements below this size fail on viewports narrower than mobileMaxWidthPx. */
  tapTargetMinPx: 32,
  mobileMaxWidthPx: 768,
  /** Cards in one row of [data-card-grid] may differ in height by this much. */
  cardRowHeightTolerancePx: 2,
  /** Console errors and uncaught exceptions allowed on a screen. */
  maxConsoleErrors: 0,
  maxPageErrors: 0,
} as const;

export interface ExpectedFailure {
  readonly status: number;
  readonly urlPattern: RegExp;
}

export interface AuditOptions {
  readonly template: LayoutTemplate;
  /** Pixels at the top of main that are not counted as dead space (e.g. a fixed header spacer). */
  readonly allowEmptyAbove?: number;
  /** HTTP failures the test declares on purpose; any other 4xx/5xx is a violation. */
  readonly expectedFailures?: readonly ExpectedFailure[];
  /** Short name used in the violation screenshot path and attachments. */
  readonly name?: string;
}

export interface EmptyRectMetric {
  readonly x: number;
  readonly y: number;
  readonly width: number;
  readonly height: number;
  /** Share of the region it was measured in. */
  readonly areaRatio: number;
}

export interface LayoutAuditMetrics {
  readonly viewport: { readonly width: number; readonly height: number };
  readonly template: LayoutTemplate;
  readonly readySignal: boolean;
  readonly mainLandmark: boolean;
  readonly mainWidthPx: number;
  readonly pageWidthRatio: number | null;
  readonly overflowPx: number;
  readonly coveragePct: number | null;
  readonly largestEmptyRect: EmptyRectMetric | null;
  readonly overlapPairs: number;
  readonly clippedText: number;
  readonly clippedControls: number;
  readonly smallTapTargets: number;
  readonly cardRowMaxDiffPx: number;
  readonly stickyFooterChecked: boolean;
  readonly consoleErrors: number;
  readonly pageErrors: number;
  readonly unexpectedResponses: number;
}

export interface AuditResult {
  readonly metrics: LayoutAuditMetrics;
  readonly violations: string[];
  /** Set by assertLayoutAudit / the audit fixture when violations exist. */
  screenshot?: string;
}

// ---------------------------------------------------------------------------------------------
// Hygiene tracking: console errors, uncaught exceptions and HTTP failures per page.
// ---------------------------------------------------------------------------------------------

export interface HttpFailure {
  readonly status: number;
  readonly method: string;
  readonly url: string;
}

export interface HygieneLog {
  readonly consoleErrors: string[];
  readonly pageErrors: string[];
  readonly failures: HttpFailure[];
  /** Empties the log, e.g. before visiting the next screen on the same page. */
  clear(): void;
}

const hygieneLogs = new WeakMap<Page, HygieneLog>();

/** Starts recording console errors, page errors and 4xx/5xx responses. Idempotent per page. */
export function trackHygiene(page: Page): HygieneLog {
  const existing = hygieneLogs.get(page);
  if (existing) return existing;

  const consoleErrors: string[] = [];
  const pageErrors: string[] = [];
  const failures: HttpFailure[] = [];
  page.on('console', (message) => {
    if (message.type() === 'error') consoleErrors.push(message.text());
  });
  page.on('pageerror', (error) => pageErrors.push(error.message));
  page.on('response', (response) => {
    if (response.status() >= 400) {
      failures.push({
        status: response.status(),
        method: response.request().method(),
        url: response.url(),
      });
    }
  });

  const log: HygieneLog = {
    consoleErrors,
    pageErrors,
    failures,
    clear: () => {
      consoleErrors.length = 0;
      pageErrors.length = 0;
      failures.length = 0;
    },
  };
  hygieneLogs.set(page, log);
  return log;
}

// ---------------------------------------------------------------------------------------------
// In-browser measurements (checks 1-8). Serialized by Playwright: must stay self-contained.
// ---------------------------------------------------------------------------------------------

interface PageInput {
  readonly template: LayoutTemplate;
  readonly allowEmptyAbove: number;
  readonly thresholds: typeof AUDIT_THRESHOLDS;
}

interface PageOutput {
  readonly mainLandmark: boolean;
  readonly mainWidthPx: number;
  readonly pageWidthRatio: number | null;
  readonly overflowPx: number;
  readonly coveragePct: number | null;
  readonly largestEmptyRect: EmptyRectMetric | null;
  readonly overlapPairs: number;
  readonly clippedText: number;
  readonly clippedControls: number;
  readonly smallTapTargets: number;
  readonly cardRowMaxDiffPx: number;
  readonly stickyFooterChecked: boolean;
  readonly violations: string[];
}

function measureLayout(input: PageInput): PageOutput {
  interface Box {
    left: number;
    top: number;
    right: number;
    bottom: number;
    width: number;
    height: number;
  }
  interface Region {
    left: number;
    top: number;
    right: number;
    bottom: number;
  }
  interface Paint {
    box: Box;
    owns: boolean;
  }
  interface CellRect {
    x: number;
    y: number;
    width: number;
    height: number;
  }
  interface RegionReport {
    coveredCells: number;
    totalCells: number;
    largest: EmptyRectMetric | null;
    band: CellRect | null;
    bandWidthRatio: number;
  }

  const th = input.thresholds;
  const violations: string[] = [];
  const vw = window.innerWidth;
  const vh = window.innerHeight;
  const mainEl = document.querySelector<HTMLElement>('main, [role="main"]');
  const main: HTMLElement = mainEl ?? document.body;
  const round = (value: number): number => Math.round(value);
  const pct = (ratio: number): string => `${round(ratio * 100)}%`;

  const boxOf = (el: Element): Box => {
    const r = el.getBoundingClientRect();
    return {
      left: r.left,
      top: r.top,
      right: r.right,
      bottom: r.bottom,
      width: r.width,
      height: r.height,
    };
  };
  // Visually hidden pattern (sr-only): a box of 1px or less that clips its content. Not user-facing.
  const isVisuallyHidden = (el: Element): boolean => {
    const r = el.getBoundingClientRect();
    return r.width <= 1 || r.height <= 1;
  };
  const isVisible = (el: Element): boolean => {
    const r = el.getBoundingClientRect();
    if (r.width <= 0 || r.height <= 0) return false;
    const cs = getComputedStyle(el);
    return (
      cs.display !== 'none' &&
      cs.visibility !== 'hidden' &&
      cs.visibility !== 'collapse' &&
      Number(cs.opacity) > 0
    );
  };
  const label = (el: Element): string => {
    const id = el.id ? `#${el.id}` : '';
    const testId = el.getAttribute('data-testid');
    const text = (el.textContent ?? '').trim().replace(/\s+/g, ' ').slice(0, 40);
    return `${el.tagName.toLowerCase()}${id}${testId ? `[data-testid="${testId}"]` : ''}${text ? ` "${text}"` : ''}`;
  };
  const ownText = (el: Element): boolean =>
    Array.from(el.childNodes).some(
      (node) => node.nodeType === Node.TEXT_NODE && (node.textContent ?? '').trim() !== '',
    );
  const interactiveSelector =
    'a[href], button, input:not([type="hidden"]), select, textarea, [role="button"], [role="link"], [role="tab"], [role="menuitem"], [role="checkbox"], [role="switch"]';
  const overlapLength = (a0: number, a1: number, b0: number, b1: number): number =>
    Math.min(a1, b1) - Math.max(a0, b0);
  const clipToViewport = (r: Box): Region => ({
    left: Math.max(r.left, 0),
    top: Math.max(r.top, 0),
    right: Math.min(r.right, vw),
    bottom: Math.min(r.bottom, vh),
  });

  // Check 1: horizontal overflow.
  const docEl = document.documentElement;
  const overflowPx = Math.max(0, docEl.scrollWidth - docEl.clientWidth);
  const offenders: string[] = [];
  for (const el of Array.from(main.querySelectorAll('*'))) {
    if (!isVisible(el) || el.closest('[data-scroll-x]')) continue;
    const r = el.getBoundingClientRect();
    if (r.right > vw + 1)
      offenders.push(`${label(el)} ends ${round(r.right - vw)}px past the viewport`);
  }
  if (overflowPx > 1 || offenders.length > 0) {
    const details = [
      ...(overflowPx > 1 ? [`document is ${overflowPx}px wider than the viewport`] : []),
      ...offenders.slice(0, 3),
    ];
    violations.push(`horizontal overflow: ${details.join('; ')}`);
  }

  // Check 2: width use of the page container.
  const mainRect = main.getBoundingClientRect();
  const mainWidthPx = mainRect.width;
  const firstChild = main.firstElementChild;
  // The page root may sit below framework host elements (routed components), so the first marked
  // descendant of main is measured; without a marker the first child of main is used.
  const pageEl =
    main.querySelector<HTMLElement>('[data-page]') ??
    (firstChild instanceof HTMLElement ? firstChild : null);
  let pageWidthRatio: number | null = null;
  if (pageEl && mainWidthPx > 0) {
    const pageWidth = pageEl.getBoundingClientRect().width;
    pageWidthRatio = pageWidth / mainWidthPx;
    if (input.template === 'reading') {
      const required = Math.min(th.readingMaxWidthPx, mainWidthPx) - 2 * th.gutterPx;
      if (pageWidth < required) {
        violations.push(
          `width: reading page is ${round(pageWidth)}px wide (needs at least ${round(required)}px)`,
        );
      }
    } else if (
      (input.template === 'wide' || input.template === 'fluid' || input.template === 'split') &&
      vw >= th.wideBreakpointPx &&
      pageWidthRatio < th.minWideRatio
    ) {
      violations.push(
        `width: page uses ${pct(pageWidthRatio)} of the content width at ${vw}px (needs at least ${pct(th.minWideRatio)})`,
      );
    }
  }

  // Check 3: dead space. Raster of the visible area; paints are elements with own text, replaced
  // content, background, border or background image.
  const paints: Paint[] = [];
  for (const el of Array.from(main.querySelectorAll('*'))) {
    if (!isVisible(el)) continue;
    const tag = el.tagName.toLowerCase();
    const cs = getComputedStyle(el);
    const replaced = [
      'img',
      'svg',
      'canvas',
      'video',
      'picture',
      'iframe',
      'input',
      'textarea',
      'select',
      'button',
    ].includes(tag);
    const text = ownText(el);
    const hasBackground =
      cs.backgroundColor !== 'rgba(0, 0, 0, 0)' && cs.backgroundColor !== 'transparent';
    const hasBackgroundImage = cs.backgroundImage !== 'none';
    const hasBorder = ['top', 'right', 'bottom', 'left'].some(
      (side) =>
        parseFloat(cs.getPropertyValue(`border-${side}-width`)) > 0 &&
        cs.getPropertyValue(`border-${side}-style`) !== 'none',
    );
    if (replaced || text || hasBackground || hasBackgroundImage || hasBorder) {
      paints.push({ box: boxOf(el), owns: replaced || text });
    }
  }

  const emptyStates = Array.from(main.querySelectorAll('[data-empty-state]'))
    .filter(isVisible)
    .map(boxOf);
  const inEmptyState = (band: Box): boolean =>
    band.height < th.emptyStateMaxHeightPx &&
    emptyStates.some(
      (es) =>
        es.left >= band.left - 1 &&
        es.right <= band.right + 1 &&
        es.top >= band.top - 1 &&
        es.bottom <= band.bottom + 1,
    );

  const analyzeRegion = (region: Region, skipTopPx: number): RegionReport => {
    const width = region.right - region.left;
    const height = region.bottom - region.top;
    const cols = Math.ceil(width / th.cellPx);
    const rows = Math.ceil(height / th.cellPx);
    const regionArea = width * height;
    const covered = new Uint8Array(cols * rows);

    for (const paint of paints) {
      const b = paint.box;
      if (!paint.owns) {
        // Full-size wrappers (background only) would hide every gap: ignore them.
        const ox = overlapLength(b.left, b.right, region.left, region.right);
        const oy = overlapLength(b.top, b.bottom, region.top, region.bottom);
        if (ox > 0 && oy > 0 && ox * oy >= 0.9 * regionArea) continue;
      }
      const x0 = Math.max(b.left, region.left) - region.left;
      const x1 = Math.min(b.right, region.right) - region.left;
      const y0 = Math.max(b.top, region.top) - region.top;
      const y1 = Math.min(b.bottom, region.bottom) - region.top;
      if (x1 <= x0 || y1 <= y0) continue;
      const c0 = Math.floor(x0 / th.cellPx);
      const c1 = Math.min(cols - 1, Math.ceil(x1 / th.cellPx) - 1);
      const r0 = Math.floor(y0 / th.cellPx);
      const r1 = Math.min(rows - 1, Math.ceil(y1 / th.cellPx) - 1);
      for (let r = r0; r <= r1; r++) for (let c = c0; c <= c1; c++) covered[r * cols + c] = 1;
    }
    for (let r = 0; r < rows && r * th.cellPx < skipTopPx; r++) {
      for (let c = 0; c < cols; c++) covered[r * cols + c] = 1;
    }

    let coveredCells = 0;
    for (const value of covered) coveredCells += value;

    // Largest empty rectangle (histogram method over the cell grid).
    let best = { c: 0, r: 0, w: 0, h: 0, area: 0 };
    const heights = new Array<number>(cols).fill(0);
    for (let r = 0; r < rows; r++) {
      for (let c = 0; c < cols; c++) heights[c] = covered[r * cols + c] ? 0 : heights[c] + 1;
      const stack: Array<{ start: number; h: number }> = [];
      for (let c = 0; c <= cols; c++) {
        const h = c < cols ? heights[c] : 0;
        let start = c;
        while (stack.length > 0 && stack[stack.length - 1].h >= h) {
          const top = stack.pop();
          if (!top) break;
          const w = c - top.start;
          if (top.h * w > best.area)
            best = { c: top.start, r: r - top.h + 1, w, h: top.h, area: top.h * w };
          start = top.start;
        }
        stack.push({ start, h });
      }
    }
    let largest: EmptyRectMetric | null = null;
    if (best.area > 0) {
      const w = Math.min(best.w * th.cellPx, width - best.c * th.cellPx);
      const h = Math.min(best.h * th.cellPx, height - best.r * th.cellPx);
      largest = {
        x: region.left + best.c * th.cellPx,
        y: region.top + best.r * th.cellPx,
        width: w,
        height: h,
        areaRatio: (w * h) / regionArea,
      };
    }

    // Empty band: a window of rows fully free over a run of columns wide and tall enough.
    const bandRows = Math.ceil(th.emptyRectMinHeightPx / th.cellPx);
    let band: CellRect | null = null;
    let bandWidthRatio = 0;
    if (rows >= bandRows) {
      for (let r0 = 0; r0 + bandRows <= rows; r0++) {
        let run = 0;
        let bestRun = 0;
        let bestEnd = -1;
        for (let c = 0; c < cols; c++) {
          let free = true;
          for (let r = r0; r < r0 + bandRows && free; r++) if (covered[r * cols + c]) free = false;
          run = free ? run + 1 : 0;
          if (run > bestRun) {
            bestRun = run;
            bestEnd = c;
          }
        }
        const ratio = Math.min(bestRun * th.cellPx, width) / width;
        if (ratio >= th.emptyRectMinWidthRatio && ratio > bandWidthRatio) {
          bandWidthRatio = ratio;
          const x = (bestEnd - bestRun + 1) * th.cellPx;
          band = {
            x: region.left + x,
            y: region.top + r0 * th.cellPx,
            width: Math.min(bestRun * th.cellPx, width - x),
            height: Math.min(bandRows * th.cellPx, height - r0 * th.cellPx),
          };
        }
      }
    }

    return { coveredCells, totalCells: cols * rows, largest, band, bandWidthRatio };
  };

  let coveragePct: number | null = null;
  let largestEmptyRect: EmptyRectMetric | null = null;
  if (input.template !== 'centered') {
    const visibleMain = clipToViewport(mainRect);
    // Fluid boards are measured per column; everything else against the visible main area.
    const columns =
      input.template === 'fluid'
        ? Array.from(main.querySelectorAll('[data-board-column]')).filter(isVisible)
        : [];
    const regions: Array<{ name: string; region: Region; skipTop: number }> =
      columns.length > 0
        ? columns.map((col, index) => ({
            name: `column ${index + 1}`,
            region: clipToViewport(boxOf(col)),
            skipTop: 0,
          }))
        : [{ name: 'main', region: visibleMain, skipTop: input.allowEmptyAbove }];

    for (const { name, region, skipTop } of regions) {
      if (region.right - region.left <= 0 || region.bottom - region.top <= 0) continue;
      const report = analyzeRegion(region, skipTop);
      if (name === 'main') {
        coveragePct =
          report.totalCells > 0
            ? Math.round((report.coveredCells / report.totalCells) * 1000) / 10
            : null;
      }
      const prefix = name === 'main' ? 'dead space' : `dead space (${name})`;
      if (report.largest && report.largest.areaRatio > th.maxEmptyAreaRatio) {
        violations.push(
          `${prefix}: largest empty block is ${round(report.largest.width)}x${round(report.largest.height)}px, ` +
            `${pct(report.largest.areaRatio)} of the visible area (limit ${pct(th.maxEmptyAreaRatio)})`,
        );
      }
      if (report.band) {
        const band = report.band;
        const bandBox: Box = {
          left: band.x,
          top: band.y,
          right: band.x + band.width,
          bottom: band.y + band.height,
          width: band.width,
          height: band.height,
        };
        if (!inEmptyState(bandBox)) {
          violations.push(
            `${prefix}: empty band ${round(band.width)}x${round(band.height)}px spans ` +
              `${pct(report.bandWidthRatio)} of the width (limit ${pct(th.emptyRectMinWidthRatio)} wide and ` +
              `${th.emptyRectMinHeightPx}px tall)`,
          );
        }
      }
      if (
        report.largest &&
        (largestEmptyRect === null || report.largest.areaRatio > largestEmptyRect.areaRatio)
      ) {
        largestEmptyRect = report.largest;
      }
    }
  }

  // Check 4: overlap between in-flow siblings.
  const parents = new Set<Element>(
    Array.from(main.querySelectorAll('*')).filter((el) => el.children.length > 1),
  );
  if (main.children.length > 1) parents.add(main);
  let overlapPairs = 0;
  const overlapMessages: string[] = [];
  // Table sections and SVG internals are drawn by their container, not laid out as siblings.
  const internalTags = ['colgroup', 'col', 'thead', 'tbody', 'tfoot', 'tr', 'td', 'th', 'caption'];
  const isInternal = (el: Element): boolean =>
    el instanceof SVGElement ||
    internalTags.includes(el.tagName.toLowerCase()) ||
    el.closest('svg') !== null;
  for (const parent of Array.from(parents)) {
    if (isInternal(parent)) continue;
    const kids = Array.from(parent.children).filter((kid) => {
      if (isInternal(kid) || !isVisible(kid) || kid.closest('[data-overlay]')) return false;
      const cs = getComputedStyle(kid);
      // Inline fragments of text that wraps report the union of their line boxes: they share a line
      // box by definition, so only boxed (block, flex, inline-block) siblings are compared.
      if (cs.display === 'inline') return false;
      return cs.position !== 'absolute' && cs.position !== 'fixed';
    });
    for (let i = 0; i < kids.length; i++) {
      for (let j = i + 1; j < kids.length; j++) {
        if (kids[i].contains(kids[j]) || kids[j].contains(kids[i])) continue;
        const a = boxOf(kids[i]);
        const b = boxOf(kids[j]);
        const ox = overlapLength(a.left, a.right, b.left, b.right);
        const oy = overlapLength(a.top, a.bottom, b.top, b.bottom);
        if (ox > th.overlapTolerancePx && oy > th.overlapTolerancePx) {
          overlapPairs++;
          if (overlapMessages.length < 3) {
            overlapMessages.push(
              `${label(kids[i])} and ${label(kids[j])} overlap by ${round(ox)}x${round(oy)}px`,
            );
          }
        }
      }
    }
  }
  if (overlapPairs > 0) {
    violations.push(
      `overlap: ${overlapMessages.join('; ')}${overlapPairs > 3 ? ` (+${overlapPairs - 3} more)` : ''}`,
    );
  }

  // Check 5a: clipped text (overflow hidden, text wider than its box).
  let clippedText = 0;
  const clippedTextMessages: string[] = [];
  for (const el of Array.from(main.querySelectorAll('*'))) {
    if (!isVisible(el)) continue;
    const cs = getComputedStyle(el);
    if (cs.overflowX !== 'hidden' && cs.overflowX !== 'clip') continue;
    if (cs.display === 'inline' || el.children.length > 0 || !ownText(el)) continue;
    // Visually hidden text (1px clip box, e.g. screen reader hints) is not clipped content.
    if (isVisuallyHidden(el)) continue;
    if (el.closest('.p-tag, .p-badge')) continue;
    if (el.scrollWidth - el.clientWidth <= 2) continue;
    const hasTooltip =
      el.hasAttribute('title') || el.hasAttribute('aria-label') || el.hasAttribute('data-tooltip');
    // An ellipsis cut of a few pixels is cosmetic; anything larger needs a title for the full text.
    const hiddenPx = el.scrollWidth - el.clientWidth;
    if (cs.textOverflow === 'ellipsis' && (hasTooltip || hiddenPx <= 8)) continue;
    clippedText++;
    if (clippedTextMessages.length < 3) {
      const reason = cs.textOverflow === 'ellipsis' ? 'ellipsis without a title' : 'no ellipsis';
      clippedTextMessages.push(
        `${label(el)} hides ${round(el.scrollWidth - el.clientWidth)}px (${reason})`,
      );
    }
  }
  if (clippedText > 0) {
    violations.push(
      `clipped text: ${clippedTextMessages.join('; ')}${clippedText > 3 ? ` (+${clippedText - 3} more)` : ''}`,
    );
  }

  // Check 5b: interactive controls partly outside their horizontal scroll container.
  const interactives = Array.from(main.querySelectorAll(interactiveSelector)).filter(
    (el) => isVisible(el) && !isVisuallyHidden(el),
  );
  let clippedControls = 0;
  const clippedControlMessages: string[] = [];
  for (const el of interactives) {
    let container: Element | null = el.parentElement;
    while (container && container !== document.body) {
      const overflowX = getComputedStyle(container).overflowX;
      if (
        overflowX === 'hidden' ||
        overflowX === 'auto' ||
        overflowX === 'scroll' ||
        overflowX === 'clip'
      )
        break;
      container = container.parentElement;
    }
    if (!container || container === document.body || container.hasAttribute('data-scroll-x'))
      continue;
    const r = boxOf(el);
    const c = boxOf(container);
    if (r.left < c.left - 1 || r.right > c.right + 1) {
      clippedControls++;
      if (clippedControlMessages.length < 3)
        clippedControlMessages.push(`${label(el)} is cut by ${label(container)}`);
    }
  }
  if (clippedControls > 0) {
    violations.push(`clipped control: ${clippedControlMessages.join('; ')}`);
  }

  // Check 6: tap targets (narrow viewports only).
  let smallTapTargets = 0;
  let firstSmall = '';
  if (vw < th.mobileMaxWidthPx) {
    for (const el of interactives) {
      if (el.closest('[aria-hidden="true"]')) continue;
      const r = boxOf(el);
      if (r.width < th.tapTargetMinPx || r.height < th.tapTargetMinPx) {
        smallTapTargets++;
        if (!firstSmall) firstSmall = `${label(el)} ${round(r.width)}x${round(r.height)}px`;
      }
    }
    if (smallTapTargets > 0) {
      violations.push(
        `tap targets: ${smallTapTargets} interactive element(s) smaller than ${th.tapTargetMinPx}x${th.tapTargetMinPx}px (first: ${firstSmall})`,
      );
    }
  }

  // Check 7: equal card heights within one row of [data-card-grid].
  let cardRowMaxDiffPx = 0;
  for (const grid of Array.from(main.querySelectorAll('[data-card-grid]'))) {
    const rows: Array<{ top: number; heights: number[] }> = [];
    for (const card of Array.from(grid.children).filter(isVisible)) {
      const box = boxOf(card);
      const row = rows.find((candidate) => Math.abs(candidate.top - box.top) <= 4);
      if (row) row.heights.push(box.height);
      else rows.push({ top: box.top, heights: [box.height] });
    }
    for (const row of rows) {
      if (row.heights.length < 2) continue;
      const diff = Math.max(...row.heights) - Math.min(...row.heights);
      cardRowMaxDiffPx = Math.max(cardRowMaxDiffPx, diff);
      if (diff > th.cardRowHeightTolerancePx) {
        violations.push(
          `card heights: a row of ${row.heights.length} cards in ${label(grid)} differs by ${round(diff)}px ` +
            `(limit ${th.cardRowHeightTolerancePx}px)`,
        );
      }
    }
  }

  // Check 8: sticky safety. Scroll to the bottom and make sure the last content sits above the footer.
  const footer = document.querySelector<HTMLElement>('[data-sticky-footer]');
  let stickyFooterChecked = false;
  if (footer && isVisible(footer)) {
    stickyFooterChecked = true;
    const savedY = window.scrollY;
    window.scrollTo(0, document.documentElement.scrollHeight);
    const content = Array.from(main.querySelectorAll('*')).filter(
      (el) =>
        !footer.contains(el) && isVisible(el) && (ownText(el) || el.matches(interactiveSelector)),
    );
    const last = content[content.length - 1];
    if (last) {
      const lb = boxOf(last);
      const limit = Math.min(boxOf(footer).top, vh);
      if (lb.bottom > limit + 1) {
        violations.push(
          `sticky footer: last content ${label(last)} ends ${round(lb.bottom - limit)}px below the area above the footer`,
        );
      }
    }
    window.scrollTo(0, savedY);
  }

  return {
    mainLandmark: mainEl !== null,
    mainWidthPx: round(mainWidthPx),
    pageWidthRatio: pageWidthRatio === null ? null : Math.round(pageWidthRatio * 1000) / 1000,
    overflowPx,
    coveragePct,
    largestEmptyRect,
    overlapPairs,
    clippedText,
    clippedControls,
    smallTapTargets,
    cardRowMaxDiffPx: round(cardRowMaxDiffPx),
    stickyFooterChecked,
    violations,
  };
}

// ---------------------------------------------------------------------------------------------
// Node-side API
// ---------------------------------------------------------------------------------------------

/** Runs every check on the current page state. Never throws on violations; returns them. */
export async function collectLayoutAudit(page: Page, options: AuditOptions): Promise<AuditResult> {
  const readySignal = await waitForReady(page);
  const viewport = await page.evaluate(() => ({
    width: window.innerWidth,
    height: window.innerHeight,
  }));
  const measured = await page.evaluate(measureLayout, {
    template: options.template,
    allowEmptyAbove: options.allowEmptyAbove ?? 0,
    thresholds: AUDIT_THRESHOLDS,
  });

  const violations = [...measured.violations];
  const log = hygieneLogs.get(page);
  const expected = options.expectedFailures ?? [];
  // Browser "Failed to load resource" messages for statuses the test declared are not errors.
  const declaredStatuses = new Set(expected.map((item) => item.status));
  const consoleErrors = (log?.consoleErrors ?? []).filter((text) => {
    const match = /status of (\d{3})/.exec(text);
    return !(match && declaredStatuses.has(Number(match[1])));
  });
  const pageErrors = log?.pageErrors ?? [];
  const unexpected = (log?.failures ?? []).filter(
    (failure) =>
      !expected.some((item) => item.status === failure.status && item.urlPattern.test(failure.url)),
  );

  if (consoleErrors.length > AUDIT_THRESHOLDS.maxConsoleErrors) {
    violations.push(`console errors: ${consoleErrors.length} (first: "${consoleErrors[0]}")`);
  }
  if (pageErrors.length > AUDIT_THRESHOLDS.maxPageErrors) {
    violations.push(
      `page errors: ${pageErrors.length} uncaught exception(s) (first: "${pageErrors[0]}")`,
    );
  }
  // 5xx is a hard failure (its own rule id, never baselined); other undeclared 4xx are "unexpected responses".
  const serverErrors = unexpected.filter((item) => item.status >= 500);
  const clientErrors = unexpected.filter((item) => item.status < 500);
  if (serverErrors.length > 0) {
    const list = serverErrors
      .slice(0, 3)
      .map((item) => `${item.status} ${item.method} ${item.url}`);
    violations.push(`server errors: ${serverErrors.length} 5xx response(s) (${list.join('; ')})`);
  }
  if (clientErrors.length > 0) {
    const list = clientErrors
      .slice(0, 3)
      .map((item) => `${item.status} ${item.method} ${item.url}`);
    violations.push(`unexpected responses: ${clientErrors.length} (${list.join('; ')})`);
  }

  const metrics: LayoutAuditMetrics = {
    viewport,
    template: options.template,
    readySignal,
    mainLandmark: measured.mainLandmark,
    mainWidthPx: measured.mainWidthPx,
    pageWidthRatio: measured.pageWidthRatio,
    overflowPx: measured.overflowPx,
    coveragePct: measured.coveragePct,
    largestEmptyRect: measured.largestEmptyRect,
    overlapPairs: measured.overlapPairs,
    clippedText: measured.clippedText,
    clippedControls: measured.clippedControls,
    smallTapTargets: measured.smallTapTargets,
    cardRowMaxDiffPx: measured.cardRowMaxDiffPx,
    stickyFooterChecked: measured.stickyFooterChecked,
    consoleErrors: consoleErrors.length,
    pageErrors: pageErrors.length,
    unexpectedResponses: unexpected.length,
  };
  return { metrics, violations };
}

/** Human-readable failure text: one line per violation plus the screenshot path when there is one. */
export function formatAuditFailure(result: AuditResult, name = 'page'): string {
  const { viewport, template } = result.metrics;
  const lines = [
    `Layout audit failed for ${name} (${viewport.width}x${viewport.height}, template ${template}):`,
  ];
  for (const violation of result.violations) lines.push(`  - ${violation}`);
  if (result.screenshot) lines.push(`  screenshot: ${result.screenshot}`);
  return lines.join('\n');
}

const AUDIT_DIR = fileURLToPath(new URL('../../test-results/layout-audit/', import.meta.url));

/** Saves a screenshot of the current viewport for a failed audit and returns its path. */
export async function captureAuditScreenshot(page: Page, name = 'page'): Promise<string> {
  mkdirSync(AUDIT_DIR, { recursive: true });
  const safe = name.replace(/[^a-zA-Z0-9-_]+/g, '-').slice(0, 80) || 'page';
  const file = path.join(AUDIT_DIR, `${safe}-${Date.now()}.png`);
  await page.screenshot({ path: file });
  return file;
}

/** Like collectLayoutAudit, but writes a screenshot and throws when any violation exists. */
export async function assertLayoutAudit(page: Page, options: AuditOptions): Promise<AuditResult> {
  const result = await collectLayoutAudit(page, options);
  if (result.violations.length > 0) {
    result.screenshot = await captureAuditScreenshot(page, options.name);
    throw new Error(formatAuditFailure(result, options.name));
  }
  return result;
}
