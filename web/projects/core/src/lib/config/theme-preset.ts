import { definePreset } from '@primeuix/themes';
import Aura from '@primeuix/themes/aura';

/**
 * z-index base values handed to PrimeNG. They mirror the --cl-z-* scale in corelog.scss: dialogs and drawers
 * start at the modal level, popup menus, selects and popovers above them, tooltips above everything.
 * The toast level is exported so a toast host can pass it as its base z-index.
 */
export const CL_Z_INDEX = {
  modal: 1100,
  overlay: 1200,
  menu: 1200,
  tooltip: 1500,
  toast: 1300,
} as const;

/** Reads a CoreLog design token. */
const cl = (name: string): string => `var(--cl-${name})`;

/** Mixes a token with the text color to build hover and pressed steps without extra tokens. */
const step = (color: string, percent: number): string =>
  `color-mix(in srgb, ${color}, var(--cl-text) ${percent}%)`;

/** Focus ring with the same look as --cl-shadow-focus. */
const FOCUS_RING = {
  width: '2px',
  style: 'solid',
  color: 'transparent',
  offset: '0',
  shadow: cl('shadow-focus'),
};

/** A solid severity of the button, badge and tag families, taken from the status tokens. */
function solidSeverity(background: string) {
  const hover = step(background, 12);
  const active = step(background, 22);
  return {
    background,
    hoverBackground: hover,
    activeBackground: active,
    borderColor: background,
    hoverBorderColor: hover,
    activeBorderColor: active,
    color: cl('text-on-brand'),
    hoverColor: cl('text-on-brand'),
    activeColor: cl('text-on-brand'),
    focusRing: { color: 'transparent', shadow: cl('shadow-focus') },
  };
}

/** An outlined or text severity: tinted hover, colored label. */
function tintedSeverity(color: string, border: string) {
  return {
    hoverBackground: cl('bg-hover'),
    activeBackground: cl('bg-selected'),
    borderColor: border,
    color,
  };
}

const SOLID_SEVERITIES = {
  info: cl('brand'),
  success: cl('success-text'),
  warn: cl('warning-text'),
  danger: cl('danger'),
  help: cl('type-epic-text'),
};

/** Message and toast severities, from the status tokens. */
const MESSAGE_SEVERITIES = {
  info: { bg: cl('brand-subtle'), border: cl('brand'), text: cl('cat-progress-text') },
  success: { bg: cl('success-bg'), border: cl('success'), text: cl('success-text') },
  warn: { bg: cl('warning-bg'), border: cl('warning'), text: cl('warning-text') },
  error: { bg: cl('danger-bg'), border: cl('danger'), text: cl('danger-text') },
  secondary: { bg: cl('bg-sunken'), border: cl('border'), text: cl('text') },
  contrast: { bg: cl('text'), border: cl('text'), text: cl('bg-surface') },
};

function messageSeverities() {
  return Object.fromEntries(
    Object.entries(MESSAGE_SEVERITIES).map(([name, c]) => [
      name,
      {
        background: c.bg,
        borderColor: c.border,
        color: c.text,
        shadow: 'none',
        closeButton: {
          hoverBackground: cl('bg-hover'),
          focusRing: { color: 'transparent', shadow: cl('shadow-focus') },
        },
        outlined: { color: c.text, borderColor: c.border },
        simple: { color: c.text },
      },
    ]),
  );
}

function toastSeverities() {
  return Object.fromEntries(
    Object.entries(MESSAGE_SEVERITIES).map(([name, c]) => [
      name,
      {
        background: c.bg,
        borderColor: c.border,
        color: c.text,
        detailColor: c.text,
        shadow: cl('elev-overlay'),
        closeButton: {
          hoverBackground: cl('bg-hover'),
          focusRing: { color: 'transparent', shadow: cl('shadow-focus') },
        },
      },
    ]),
  );
}

/** Minimum heights by control size; the touch overrides live in the control tokens themselves. */
const CONTROL_CSS = `
.p-button, .p-select, .p-inputtext, .p-togglebutton, .p-autocomplete, .p-password, .p-inputnumber {
  min-height: var(--cl-control-h-md);
}
.p-button-sm, .p-select-sm, .p-inputtext-sm, .p-togglebutton-sm {
  min-height: var(--cl-control-h-sm);
}
.p-button-lg, .p-select-lg, .p-inputtext-lg, .p-togglebutton-lg {
  min-height: var(--cl-control-h-lg);
}
.p-button-icon-only { min-width: var(--cl-control-h-md); }
.p-button-icon-only.p-button-sm { min-width: var(--cl-control-h-sm); }
.p-button-icon-only.p-button-lg { min-width: var(--cl-control-h-lg); }
`;

const MENU_ITEM_CSS = `
.p-menu-item-link, .p-tieredmenu-item-link, .p-contextmenu-item-link, .p-menubar-item-link, .p-select-option {
  min-height: var(--cl-menu-item-h);
}
.p-menu-overlay, .p-tieredmenu-overlay, .p-contextmenu {
  min-width: var(--cl-menu-w-min);
  max-width: var(--cl-menu-w);
}
`;

const DIALOG_CSS = `
.p-dialog {
  width: var(--cl-dialog-w-md);
  max-width: calc(100vw - 2 * var(--cl-space-4));
}
.p-dialog.cl-dialog--sm { width: var(--cl-dialog-w-sm); }
.p-dialog.cl-dialog--lg { width: var(--cl-dialog-w-lg); }
`;

/**
 * PrimeNG preset on the CoreLog design tokens. Every color, size, radius, shadow and motion value is a
 * var(--cl-*) reference. Those tokens change with the data-theme attribute, so one mapping covers the light
 * and the dark scheme. The neutral surface ramp is the only static palette: components that are not mapped
 * below still read it, and it stays legible in both schemes.
 */
const PRESET_DEFINITION = {
  primitive: {
    // One radius for the whole product.
    borderRadius: {
      xs: cl('radius-md'),
      sm: cl('radius-md'),
      md: cl('radius-md'),
      lg: cl('radius-md'),
      xl: cl('radius-md'),
    },
  },
  semantic: {
    transitionDuration: cl('motion'),
    typography: {
      fontSize: cl('font-body'),
      fontWeight: cl('weight-regular'),
      lineHeight: cl('lh-body'),
    },
    disabledOpacity: cl('disabled-opacity'),
    anchorGutter: cl('space-1'),
    focusRing: FOCUS_RING,
    primary: {
      50: '#e9f2ff',
      100: '#deebff',
      200: '#b3d4ff',
      300: '#79b8ff',
      400: '#4c9aff',
      500: '#2684ff',
      600: '#0052cc',
      700: '#0747a6',
      800: '#06367f',
      900: '#042a60',
      950: '#021b40',
      color: cl('brand'),
      contrastColor: cl('text-on-brand'),
      hoverColor: cl('brand-hover'),
      activeColor: cl('brand-pressed'),
    },
    surface: {
      0: '#ffffff',
      50: '#f7f8fa',
      100: '#f1f2f4',
      200: '#dfe1e6',
      300: '#c1c7d0',
      400: '#a5adba',
      500: '#7a869a',
      600: '#5e6c84',
      700: '#44546f',
      800: '#2c333a',
      900: '#22272b',
      950: '#1d2125',
    },
    highlight: {
      background: cl('bg-selected'),
      focusBackground: step(cl('bg-selected'), 6),
      color: cl('text'),
      focusColor: cl('text'),
    },
    text: {
      color: cl('text'),
      hoverColor: cl('text'),
      mutedColor: cl('text-subtle'),
      hoverMutedColor: cl('text'),
    },
    formField: {
      fontWeight: cl('weight-regular'),
      paddingX: cl('space-3'),
      paddingY: cl('space-1'),
      sm: { fontSize: cl('font-small'), paddingX: cl('space-2'), paddingY: '2px' },
      lg: { fontSize: cl('font-body-strong'), paddingX: cl('space-4'), paddingY: cl('space-2') },
      borderRadius: '{border.radius.md}',
      focusRing: FOCUS_RING,
      transitionDuration: cl('motion-fast'),
      background: cl('bg-surface'),
      disabledBackground: cl('bg-sunken'),
      filledBackground: cl('bg-sunken'),
      filledHoverBackground: cl('bg-hover'),
      filledFocusBackground: cl('bg-surface'),
      borderColor: cl('border-strong'),
      hoverBorderColor: cl('text-subtle'),
      focusBorderColor: cl('brand'),
      invalidBorderColor: cl('danger'),
      color: cl('text'),
      disabledColor: cl('text-disabled'),
      placeholderColor: cl('text-placeholder'),
      invalidPlaceholderColor: cl('danger-text'),
      floatLabelColor: cl('text-placeholder'),
      floatLabelFocusColor: cl('brand'),
      floatLabelActiveColor: cl('text-placeholder'),
      floatLabelInvalidColor: cl('danger-text'),
      iconColor: cl('text-subtle'),
      shadow: 'none',
    },
    list: {
      padding: cl('space-1'),
      gap: '0',
      option: {
        padding: `${cl('space-2')} ${cl('space-3')}`,
        borderRadius: '{border.radius.md}',
        fontWeight: cl('weight-regular'),
        transitionDuration: cl('motion-fast'),
        focusBackground: cl('bg-hover'),
        selectedBackground: cl('bg-selected'),
        selectedFocusBackground: step(cl('bg-selected'), 6),
        color: cl('text'),
        focusColor: cl('text'),
        selectedColor: cl('text'),
        selectedFocusColor: cl('text'),
        selectedFontWeight: cl('weight-semibold'),
        icon: { color: cl('text-subtle'), focusColor: cl('text') },
      },
      optionGroup: {
        padding: `${cl('space-2')} ${cl('space-3')}`,
        fontWeight: cl('weight-semibold'),
        fontSize: cl('font-small'),
        color: cl('text-subtle'),
      },
    },
    content: {
      borderRadius: '{border.radius.md}',
      background: cl('bg-surface'),
      hoverBackground: cl('bg-hover'),
      borderColor: cl('divider'),
      color: cl('text'),
      hoverColor: cl('text'),
    },
    mask: {
      transitionDuration: cl('motion'),
      background: cl('overlay'),
      color: cl('text'),
    },
    navigation: {
      list: { padding: cl('space-1'), gap: '0' },
      item: {
        padding: `${cl('space-1')} ${cl('space-3')}`,
        borderRadius: '{border.radius.md}',
        gap: cl('space-2'),
        focusBackground: cl('bg-hover'),
        activeBackground: cl('bg-selected'),
        color: cl('text'),
        focusColor: cl('text'),
        activeColor: cl('text'),
        icon: { color: cl('text-subtle'), focusColor: cl('text'), activeColor: cl('text') },
        label: { fontWeight: cl('weight-regular'), fontSize: cl('font-body') },
        transitionDuration: cl('motion-fast'),
      },
      submenuLabel: {
        padding: `${cl('space-2')} ${cl('space-3')}`,
        fontWeight: cl('weight-semibold'),
        fontSize: cl('font-small'),
        color: cl('text-subtle'),
      },
      submenuIcon: { color: cl('text-subtle'), focusColor: cl('text'), activeColor: cl('text') },
    },
    overlay: {
      select: {
        borderRadius: '{border.radius.md}',
        shadow: cl('elev-overlay'),
        background: cl('bg-elevated'),
        borderColor: cl('divider'),
        color: cl('text'),
      },
      popover: {
        borderRadius: '{border.radius.md}',
        padding: cl('space-4'),
        shadow: cl('elev-overlay'),
        background: cl('bg-elevated'),
        borderColor: cl('divider'),
        color: cl('text'),
      },
      modal: {
        borderRadius: '{border.radius.md}',
        padding: cl('space-6'),
        shadow: cl('elev-modal'),
        background: cl('bg-elevated'),
        borderColor: cl('divider'),
        color: cl('text'),
      },
      navigation: { shadow: cl('elev-overlay') },
    },
  },
  components: {
    button: {
      root: {
        label: { fontWeight: cl('btn-weight') },
        raisedShadow: cl('elev-raised'),
        focusRing: { width: '2px', style: 'solid', offset: '0' },
        iconOnlyWidth: cl('control-h-md'),
        sm: { iconOnlyWidth: cl('control-h-sm') },
        lg: { iconOnlyWidth: cl('control-h-lg') },
        secondary: {
          background: cl('bg-sunken'),
          hoverBackground: step(cl('bg-sunken'), 8),
          activeBackground: step(cl('bg-sunken'), 14),
          borderColor: cl('bg-sunken'),
          hoverBorderColor: step(cl('bg-sunken'), 8),
          activeBorderColor: step(cl('bg-sunken'), 14),
          color: cl('text'),
          hoverColor: cl('text'),
          activeColor: cl('text'),
          focusRing: { color: 'transparent', shadow: cl('shadow-focus') },
        },
        primary: {
          focusRing: { color: 'transparent', shadow: cl('shadow-focus') },
        },
        ...Object.fromEntries(
          Object.entries(SOLID_SEVERITIES).map(([name, color]) => [name, solidSeverity(color)]),
        ),
        contrast: solidSeverity(cl('text')),
      },
      outlined: {
        primary: tintedSeverity(cl('brand'), cl('brand')),
        secondary: tintedSeverity(cl('text'), cl('border-strong')),
        plain: tintedSeverity(cl('text'), cl('border-strong')),
        danger: tintedSeverity(cl('danger-text'), cl('danger')),
        success: tintedSeverity(cl('success-text'), cl('success')),
        info: tintedSeverity(cl('brand'), cl('brand')),
        warn: tintedSeverity(cl('warning-text'), cl('warning')),
        help: tintedSeverity(cl('type-epic-text'), cl('type-epic')),
        contrast: tintedSeverity(cl('text'), cl('text')),
      },
      text: {
        primary: {
          hoverBackground: cl('bg-hover'),
          activeBackground: cl('bg-selected'),
          color: cl('brand'),
        },
        secondary: {
          hoverBackground: cl('bg-hover'),
          activeBackground: cl('bg-selected'),
          color: cl('text-subtle'),
        },
        plain: {
          hoverBackground: cl('bg-hover'),
          activeBackground: cl('bg-selected'),
          color: cl('text'),
        },
        danger: {
          hoverBackground: cl('danger-bg'),
          activeBackground: cl('danger-bg'),
          color: cl('danger-text'),
        },
        success: {
          hoverBackground: cl('success-bg'),
          activeBackground: cl('success-bg'),
          color: cl('success-text'),
        },
        info: {
          hoverBackground: cl('bg-hover'),
          activeBackground: cl('bg-selected'),
          color: cl('brand'),
        },
        warn: {
          hoverBackground: cl('warning-bg'),
          activeBackground: cl('warning-bg'),
          color: cl('warning-text'),
        },
        help: {
          hoverBackground: cl('bg-hover'),
          activeBackground: cl('bg-selected'),
          color: cl('type-epic-text'),
        },
        contrast: {
          hoverBackground: cl('bg-hover'),
          activeBackground: cl('bg-selected'),
          color: cl('text'),
        },
      },
      link: { color: cl('brand'), hoverColor: cl('brand-hover'), activeColor: cl('brand-pressed') },
      css: CONTROL_CSS,
    },
    inputtext: { css: CONTROL_CSS },
    select: {
      dropdown: { width: cl('control-h-md'), color: cl('text-subtle') },
      option: { padding: `${cl('space-2')} ${cl('space-3')}` },
      css: `${CONTROL_CSS}${MENU_ITEM_CSS}`,
    },
    datepicker: {
      panel: {
        background: cl('bg-elevated'),
        borderColor: cl('divider'),
        color: cl('text'),
        shadow: cl('elev-overlay'),
        padding: cl('space-4'),
      },
      header: { background: cl('bg-elevated'), borderColor: cl('divider'), color: cl('text') },
      dropdown: {
        width: cl('control-h-md'),
        background: cl('bg-sunken'),
        hoverBackground: cl('bg-hover'),
        activeBackground: cl('bg-selected'),
        color: cl('text'),
        hoverColor: cl('text'),
        activeColor: cl('text'),
      },
      date: {
        width: '2rem',
        height: '2rem',
        borderRadius: '{border.radius.md}',
        hoverBackground: cl('bg-hover'),
        selectedBackground: cl('brand'),
        selectedColor: cl('text-on-brand'),
        rangeSelectedBackground: cl('bg-selected'),
        rangeSelectedColor: cl('text'),
        color: cl('text'),
        hoverColor: cl('text'),
      },
      today: { background: cl('bg-sunken'), color: cl('text') },
      weekDay: { color: cl('text-subtle') },
    },
    dialog: {
      root: {
        background: cl('bg-elevated'),
        borderColor: cl('divider'),
        color: cl('text'),
        borderRadius: '{border.radius.md}',
        shadow: cl('elev-modal'),
      },
      header: { padding: `${cl('space-5')} ${cl('space-6')} ${cl('space-3')}`, gap: cl('space-2') },
      title: { fontSize: cl('font-h3'), fontWeight: cl('weight-semibold') },
      content: { padding: `0 ${cl('space-6')} ${cl('space-5')}` },
      footer: { padding: `0 ${cl('space-6')} ${cl('space-5')}`, gap: cl('space-2') },
      css: DIALOG_CSS,
    },
    menu: {
      root: {
        background: cl('bg-elevated'),
        borderColor: cl('divider'),
        color: cl('text'),
        shadow: cl('elev-overlay'),
      },
      separator: { borderColor: cl('divider') },
      css: MENU_ITEM_CSS,
    },
    tieredmenu: {
      root: {
        background: cl('bg-elevated'),
        borderColor: cl('divider'),
        color: cl('text'),
        shadow: cl('elev-overlay'),
      },
      separator: { borderColor: cl('divider') },
      css: MENU_ITEM_CSS,
    },
    contextmenu: {
      root: {
        background: cl('bg-elevated'),
        borderColor: cl('divider'),
        color: cl('text'),
        shadow: cl('elev-overlay'),
      },
      separator: { borderColor: cl('divider') },
      css: MENU_ITEM_CSS,
    },
    menubar: {
      root: {
        background: cl('bg-surface'),
        borderColor: cl('divider'),
        color: cl('text'),
      },
      submenu: {
        background: cl('bg-elevated'),
        borderColor: cl('divider'),
        shadow: cl('elev-overlay'),
      },
      separator: { borderColor: cl('divider') },
      css: MENU_ITEM_CSS,
    },
    popover: {
      root: {
        background: cl('bg-elevated'),
        borderColor: cl('divider'),
        color: cl('text'),
        shadow: cl('elev-overlay'),
      },
      content: { padding: cl('space-4') },
      css: `.p-popover { max-width: var(--cl-popover-w); }`,
    },
    tooltip: {
      root: {
        maxWidth: cl('tooltip-w'),
        padding: `${cl('space-1')} ${cl('space-2')}`,
        fontSize: cl('font-small'),
        background: cl('text'),
        color: cl('bg-surface'),
        shadow: cl('elev-overlay'),
      },
    },
    toast: {
      root: { width: cl('toast-w'), blur: '0', transitionDuration: cl('motion') },
      summary: { fontWeight: cl('weight-semibold'), fontSize: cl('font-body') },
      detail: { fontSize: cl('font-body') },
      normal: {
        background: cl('bg-elevated'),
        borderColor: cl('divider'),
        color: cl('text'),
        detailColor: cl('text-subtle'),
        shadow: cl('elev-overlay'),
        closeButton: {
          hoverBackground: cl('bg-hover'),
          focusRing: { color: 'transparent', shadow: cl('shadow-focus') },
        },
      },
      ...toastSeverities(),
    },
    message: messageSeverities(),
    tabs: {
      tablist: { background: cl('bg-surface'), borderColor: cl('divider') },
      tab: {
        color: cl('text-subtle'),
        hoverColor: cl('text'),
        activeColor: cl('brand'),
        padding: `${cl('space-3')} ${cl('space-4')}`,
        fontWeight: cl('weight-medium'),
      },
      tabpanel: { background: cl('bg-surface'), color: cl('text') },
      navButton: {
        background: cl('bg-surface'),
        color: cl('text-subtle'),
        hoverColor: cl('text'),
        shadow: 'none',
      },
      activeBar: { background: cl('brand') },
    },
    drawer: {
      root: {
        background: cl('bg-elevated'),
        borderColor: cl('divider'),
        color: cl('text'),
        shadow: cl('elev-modal'),
      },
      title: { fontSize: cl('font-h3'), fontWeight: cl('weight-semibold') },
    },
    datatable: {
      headerCell: {
        background: cl('bg-sunken'),
        hoverBackground: cl('bg-hover'),
        selectedBackground: cl('bg-selected'),
        borderColor: cl('divider'),
        color: cl('text-subtle'),
        hoverColor: cl('text'),
        selectedColor: cl('text'),
        padding: `${cl('space-2')} ${cl('space-3')}`,
      },
      columnTitle: { fontWeight: cl('weight-semibold'), fontSize: cl('font-small') },
      header: { background: cl('bg-surface'), borderColor: cl('divider'), color: cl('text') },
      row: {
        background: cl('bg-surface'),
        hoverBackground: cl('bg-hover'),
        selectedBackground: cl('bg-selected'),
        color: cl('text'),
        hoverColor: cl('text'),
        selectedColor: cl('text'),
        stripedBackground: cl('bg-page'),
      },
      bodyCell: {
        borderColor: cl('divider'),
        padding: `${cl('space-2')} ${cl('space-3')}`,
        fontSize: cl('font-body'),
        fontWeight: cl('weight-regular'),
        selectedBorderColor: cl('divider'),
      },
      footerCell: { background: cl('bg-surface'), borderColor: cl('divider'), color: cl('text') },
      sortIcon: { color: cl('text-subtle'), hoverColor: cl('text') },
    },
    selectbutton: {
      root: { borderRadius: '{border.radius.md}' },
    },
    togglebutton: {
      root: {
        background: cl('bg-sunken'),
        checkedBackground: cl('bg-surface'),
        hoverBackground: cl('bg-hover'),
        borderColor: cl('border-strong'),
        checkedBorderColor: cl('brand'),
        color: cl('text-subtle'),
        hoverColor: cl('text'),
        checkedColor: cl('text'),
        fontWeight: cl('weight-medium'),
        padding: cl('space-1'),
      },
      content: {
        checkedBackground: cl('bg-surface'),
        checkedShadow: cl('elev-raised'),
        padding: `${cl('space-1')} ${cl('space-3')}`,
      },
      icon: { color: cl('text-subtle'), hoverColor: cl('text'), checkedColor: cl('brand') },
    },
    toggleswitch: {
      root: {
        background: cl('border-strong'),
        hoverBackground: cl('text-subtle'),
        disabledBackground: cl('bg-sunken'),
        checkedBackground: cl('brand'),
        checkedHoverBackground: cl('brand-hover'),
        borderColor: 'transparent',
        slideDuration: cl('motion'),
      },
      handle: {
        background: cl('bg-surface'),
        hoverBackground: cl('bg-surface'),
        checkedBackground: cl('bg-surface'),
        checkedHoverBackground: cl('bg-surface'),
        disabledBackground: cl('text-disabled'),
        color: cl('text-subtle'),
        hoverColor: cl('text'),
        checkedColor: cl('brand'),
        checkedHoverColor: cl('brand-hover'),
      },
    },
    checkbox: {
      root: {
        borderRadius: cl('radius-sm'),
        checkedBackground: cl('brand'),
        checkedHoverBackground: cl('brand-hover'),
        checkedBorderColor: cl('brand'),
        checkedHoverBorderColor: cl('brand-hover'),
        checkedFocusBorderColor: cl('brand'),
      },
      icon: { checkedColor: cl('text-on-brand'), checkedHoverColor: cl('text-on-brand') },
    },
    radiobutton: {
      root: {
        checkedBackground: cl('brand'),
        checkedHoverBackground: cl('brand-hover'),
        checkedBorderColor: cl('brand'),
        checkedHoverBorderColor: cl('brand-hover'),
        checkedFocusBorderColor: cl('brand'),
      },
      icon: { checkedColor: cl('text-on-brand'), checkedHoverColor: cl('text-on-brand') },
    },
    tag: {
      root: { fontWeight: cl('weight-semibold'), fontSize: cl('font-small') },
      primary: { background: cl('brand-subtle'), color: cl('cat-progress-text') },
      secondary: { background: cl('bg-sunken'), color: cl('text-subtle') },
      success: { background: cl('success-bg'), color: cl('success-text') },
      info: { background: cl('brand-subtle'), color: cl('cat-progress-text') },
      warn: { background: cl('warning-bg'), color: cl('warning-text') },
      danger: { background: cl('danger-bg'), color: cl('danger-text') },
      contrast: { background: cl('text'), color: cl('bg-surface') },
    },
    badge: {
      primary: { background: cl('brand'), color: cl('text-on-brand') },
      secondary: { background: cl('bg-sunken'), color: cl('text-subtle') },
      success: { background: cl('success-text'), color: cl('text-on-brand') },
      info: { background: cl('brand'), color: cl('text-on-brand') },
      warn: { background: cl('warning-text'), color: cl('text-on-brand') },
      danger: { background: cl('danger'), color: cl('text-on-brand') },
      contrast: { background: cl('text'), color: cl('bg-surface') },
    },
    skeleton: {
      root: { background: cl('bg-sunken'), animationBackground: cl('bg-hover') },
    },
    progressbar: {
      root: { background: cl('bg-sunken') },
      value: { background: cl('brand') },
      label: { color: cl('text-on-brand') },
    },
    paginator: {
      root: { background: cl('bg-surface'), color: cl('text') },
      navButton: {
        hoverBackground: cl('bg-hover'),
        selectedBackground: cl('bg-selected'),
        color: cl('text-subtle'),
        hoverColor: cl('text'),
        selectedColor: cl('text'),
        width: cl('control-h-md'),
        height: cl('control-h-md'),
        borderRadius: '{border.radius.md}',
      },
      currentPageReport: { color: cl('text-subtle') },
    },
  },
};

/** The mapping on its own, without the Aura defaults (used by the preset tests). */
export const CORELOG_PRESET_DEFINITION = PRESET_DEFINITION;

export const CorelogPreset = definePreset(Aura, PRESET_DEFINITION);
