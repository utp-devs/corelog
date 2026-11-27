/** Stroke paths on a 24 px grid for every icon key of the catalog palette. */
export const CATALOG_ICON_PATHS: Readonly<Record<string, readonly string[]>> = {
  task: ['M4 4h16v16H4z', 'M8 12l3 3 5-6'],
  bug: [
    'M8 10a4 5 0 0 1 8 0v4a4 5 0 0 1-8 0z',
    'M8 11H4',
    'M8 15H4',
    'M16 11h4',
    'M16 15h4',
    'M9 6L8 4',
    'M15 6l1-2',
  ],
  story: ['M5 4h11a3 3 0 0 1 3 3v13H8a3 3 0 0 1-3-3z', 'M5 17a3 3 0 0 1 3-3h11'],
  epic: ['M13 2L5 14h6l-1 8 8-12h-6z'],
  subtask: ['M5 4h6v5H5z', 'M13 15h6v5h-6z', 'M8 9v8h5'],
  star: ['M12 3l2.7 5.6 6.1.9-4.4 4.3 1 6.1L12 17l-5.4 2.9 1-6.1L3.2 9.5l6.1-.9z'],
  flag: ['M5 21V4', 'M5 4h12l-2 4 2 4H5'],
  bolt: ['M13 3L6 13h5l-1 8 7-10h-5z'],
  alert: ['M12 3l10 18H2z', 'M12 10v5', 'M12 18v.01'],
  check: ['M5 12l5 5 9-10'],
  layers: ['M12 3l9 5-9 5-9-5z', 'M3 13l9 5 9-5'],
  tag: ['M3 12V4h8l10 10-8 8z', 'M7.5 8h.01'],
  book: [
    'M4 5h7a2 2 0 0 1 1 .3 2 2 0 0 1 1-.3h7v14h-7a2 2 0 0 0-1 .3 2 2 0 0 0-1-.3H4z',
    'M12 6v13',
  ],
  rocket: [
    'M12 3c4 2 6 6 6 10l-3 3H9l-3-3c0-4 2-8 6-10z',
    'M9 16l-2 5 4-2',
    'M15 16l2 5-4-2',
    'M12 9v.01',
  ],
  shield: ['M12 3l8 3v6c0 5-3.5 8-8 9-4.5-1-8-4-8-9V6z'],
  clock: ['M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18z', 'M12 7v5l3 2'],
  circle: ['M12 4a8 8 0 1 0 0 16 8 8 0 0 0 0-16z'],
  'arrow-up': ['M12 20V5', 'M6 11l6-6 6 6'],
  'arrow-down': ['M12 4v15', 'M6 13l6 6 6-6'],
  minus: ['M5 12h14'],
  'chevrons-up': ['M6 12l6-6 6 6', 'M6 19l6-6 6 6'],
  'chevrons-down': ['M6 5l6 6 6-6', 'M6 12l6 6 6-6'],
};

/** Icon used when a key is not in the palette. */
export const CATALOG_ICON_FALLBACK = 'circle';
