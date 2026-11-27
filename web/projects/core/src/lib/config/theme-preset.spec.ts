import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { describe, expect, it } from 'vitest';
import { MenuItem } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { DialogModule } from 'primeng/dialog';
import { InputTextModule } from 'primeng/inputtext';
import { MenuModule } from 'primeng/menu';
import { providePrimeNG } from 'primeng/config';

import corelogStyles from '../../../../ui/styles/corelog.scss' with { loader: 'text' };
import { CL_Z_INDEX, CORELOG_PRESET_DEFINITION, CorelogPreset } from './theme-preset';

type Tokens = Record<string, string>;

/** Declarations of the --cl-* custom properties inside the first block that starts with the selector. */
function readBlock(source: string, selector: string): Tokens {
  const start = source.indexOf(`${selector} {`);
  const end = source.indexOf('\n}', start);
  const tokens: Tokens = {};
  for (const match of source.slice(start, end).matchAll(/(--cl-[a-z0-9-]+):\s*([^;]+);/g)) {
    tokens[match[1]] = match[2].trim();
  }
  return tokens;
}

const LIGHT = readBlock(corelogStyles, ':root');
const DARK = { ...LIGHT, ...readBlock(corelogStyles, ":root[data-theme='dark']") };
const THEMES: Record<string, Tokens> = { light: LIGHT, dark: DARK };

function luminance(hex: string): number {
  const value = hex.replace('#', '');
  const [r, g, b] = [0, 2, 4]
    .map((i) => parseInt(value.slice(i, i + 2), 16) / 255)
    .map((c) => (c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4));
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

function contrast(a: string, b: string): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
}

/** Resolves a token to its final hex value, following var() references. */
function color(tokens: Tokens, name: string): string {
  let value = tokens[`--cl-${name}`];
  while (value?.startsWith('var(')) {
    value = tokens[value.slice(4, -1)];
  }
  expect(value, `token --cl-${name}`).toMatch(/^#[0-9a-f]{6}$/i);
  return value;
}

describe.each(Object.keys(THEMES))('contrast of the %s theme', (theme) => {
  const tokens = THEMES[theme];
  const TEXT_PAIRS: [string, string][] = [
    ['text', 'bg-surface'],
    ['text', 'bg-page'],
    ['text', 'bg-elevated'],
    ['text', 'bg-hover'],
    ['text', 'bg-selected'],
    ['text-subtle', 'bg-surface'],
    ['text-subtle', 'bg-elevated'],
    ['text-placeholder', 'bg-surface'],
    ['text-on-brand', 'brand'],
    ['text-on-brand', 'brand-hover'],
    ['text-on-brand', 'brand-pressed'],
    ['danger-text', 'bg-surface'],
    ['danger-text', 'danger-bg'],
    ['success-text', 'success-bg'],
    ['warning-text', 'warning-bg'],
  ];

  it.each(TEXT_PAIRS)('%s over %s reaches 4.5:1', (fg, bg) => {
    expect(contrast(color(tokens, fg), color(tokens, bg))).toBeGreaterThanOrEqual(4.5);
  });

  it.each(['bg-surface', 'bg-page', 'bg-elevated'])(
    'control border is at least 3:1 over %s',
    (surface) => {
      expect(
        contrast(color(tokens, 'border-strong'), color(tokens, surface)),
      ).toBeGreaterThanOrEqual(3);
    },
  );

  it('declares the tokens that PrimeNG consumes', () => {
    for (const name of ['bg-elevated', 'bg-hover', 'bg-selected', 'divider', 'control-h-md']) {
      expect(tokens[`--cl-${name}`], name).toBeTruthy();
    }
  });
});

describe('dark theme', () => {
  it('draws borders at least 3:1 over the dark surfaces', () => {
    for (const surface of ['bg-surface', 'bg-page']) {
      expect(contrast(color(DARK, 'border'), color(DARK, surface))).toBeGreaterThanOrEqual(3);
    }
  });

  it('lifts overlays above the page and uses its own shadows', () => {
    expect(color(DARK, 'bg-elevated')).not.toBe(color(DARK, 'bg-page'));
    expect(DARK['--cl-shadow-3']).not.toBe(LIGHT['--cl-shadow-3']);
  });
});

/** Collects every string leaf of a preset branch with its path. */
function leaves(node: unknown, path: string[] = []): [string, string][] {
  if (typeof node === 'string') return [[path.join('.'), node]];
  if (node && typeof node === 'object') {
    return Object.entries(node).flatMap(([key, child]) => leaves(child, [...path, key]));
  }
  return [];
}

describe('CorelogPreset', () => {
  const preset = CORELOG_PRESET_DEFINITION as unknown as {
    primitive: Record<string, Record<string, string>>;
    semantic: Record<string, Record<string, unknown>>;
    components: Record<string, unknown>;
  };
  // The primary and surface ramps are palettes, not themed values.
  const { primary: _primary, surface: _surface, ...themedSemantic } = preset.semantic;
  const themed = [
    ...leaves(themedSemantic, ['semantic']),
    ...leaves(preset.components, ['components']),
  ];

  it('never hard-codes a color outside the palettes', () => {
    const hardcoded = themed.filter(([, value]) => /#[0-9a-f]{3,8}\b|rgba?\(/i.test(value));
    expect(hardcoded).toEqual([]);
  });

  it('references only tokens declared in both themes', () => {
    const referenced = new Set(
      themed.flatMap(([, value]) =>
        [...value.matchAll(/var\((--cl-[a-z0-9-]+)\)/g)].map((match) => match[1]),
      ),
    );
    expect(referenced.size).toBeGreaterThan(30);
    for (const theme of Object.keys(THEMES)) {
      for (const token of referenced) {
        expect(THEMES[theme][token], `${token} in ${theme}`).toBeDefined();
      }
    }
  });

  it('maps surfaces, text, borders, radius and focus to tokens', () => {
    const { formField, text, primary, focusRing } = preset.semantic;
    expect(formField['background']).toBe('var(--cl-bg-surface)');
    expect(formField['borderColor']).toBe('var(--cl-border-strong)');
    expect(formField['placeholderColor']).toBe('var(--cl-text-placeholder)');
    expect(text['color']).toBe('var(--cl-text)');
    expect(primary['color']).toBe('var(--cl-brand)');
    expect(focusRing['shadow']).toBe('var(--cl-shadow-focus)');
    expect(preset.primitive['borderRadius']['md']).toBe('var(--cl-radius-md)');
  });

  it('keeps the PrimeNG z-index base values on the token scale', () => {
    expect(CL_Z_INDEX.overlay).toBeGreaterThan(CL_Z_INDEX.modal);
    expect(CL_Z_INDEX.toast).toBeGreaterThan(CL_Z_INDEX.overlay);
    expect(CL_Z_INDEX.tooltip).toBeGreaterThan(CL_Z_INDEX.toast);
    expect(LIGHT['--cl-z-modal']).toBe(String(CL_Z_INDEX.modal));
    expect(LIGHT['--cl-z-overlay']).toBe(String(CL_Z_INDEX.overlay));
    expect(LIGHT['--cl-z-toast']).toBe(String(CL_Z_INDEX.toast));
    expect(LIGHT['--cl-z-tooltip']).toBe(String(CL_Z_INDEX.tooltip));
  });
});

@Component({
  selector: 'app-theme-host',
  imports: [ButtonModule, InputTextModule, DialogModule, MenuModule],
  template: `
    <p-button label="Guardar" />
    <input pInputText aria-label="Nombre" />
    <p-menu [model]="items" />
    <p-dialog header="Titulo" [visible]="true" [modal]="false" />
  `,
})
class ThemeHostComponent {
  readonly items: MenuItem[] = [{ label: 'Editar' }];
}

describe('PrimeNG design variables', () => {
  it('resolve to the token variables instead of fixed colors', async () => {
    TestBed.configureTestingModule({
      providers: [
        providePrimeNG({
          theme: {
            preset: CorelogPreset,
            options: { darkModeSelector: "[data-theme='dark']", cssLayer: false },
          },
        }),
      ],
    });
    const fixture = TestBed.createComponent(ThemeHostComponent);
    fixture.detectChanges();
    await fixture.whenStable();

    const css = Array.from(document.head.querySelectorAll('style'))
      .map((style) => style.textContent ?? '')
      .join('\n')
      .replace(/\s+/g, '');

    const expected: Record<string, string> = {
      '--p-form-field-background': 'var(--cl-bg-surface)',
      '--p-form-field-border-color': 'var(--cl-border-strong)',
      '--p-form-field-placeholder-color': 'var(--cl-text-placeholder)',
      '--p-text-color': 'var(--cl-text)',
      '--p-primary-color': 'var(--cl-brand)',
      '--p-focus-ring-shadow': 'var(--cl-shadow-focus)',
      '--p-overlay-modal-background': 'var(--cl-bg-elevated)',
      '--p-overlay-select-shadow': 'var(--cl-elev-overlay)',
      '--p-content-border-color': 'var(--cl-divider)',
      '--p-button-label-font-weight': 'var(--cl-btn-weight)',
      '--p-border-radius-md': 'var(--cl-radius-md)',
    };
    for (const [name, value] of Object.entries(expected)) {
      expect(css, name).toContain(`${name}:${value}`);
      // The referenced token exists in both themes, so the variable resolves in light and dark.
      for (const theme of Object.keys(THEMES)) {
        expect(THEMES[theme][value.slice(4, -1)], `${value} in ${theme}`).toBeDefined();
      }
    }
    expect(css).not.toContain('--p-form-field-background:light-dark');
    expect(css).not.toContain('--p-text-color:light-dark');
  });
});
