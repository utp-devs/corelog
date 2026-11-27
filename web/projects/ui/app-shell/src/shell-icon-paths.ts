/** Names of the line icons of the shell. */
export type ShellIconName =
  | 'menu'
  | 'plus'
  | 'caret'
  | 'search'
  | 'bell'
  | 'help'
  | 'close'
  | 'board'
  | 'backlog'
  | 'sprints'
  | 'calendar'
  | 'permissions'
  | 'settings'
  | 'members'
  | 'invitations'
  | 'profile'
  | 'security'
  | 'appearance'
  | 'fold'
  | 'unfold'
  | 'folder'
  | 'issue'
  | 'dashboard'
  | 'team'
  | 'clock'
  | 'check'
  | 'logout'
  | 'sun'
  | 'moon'
  | 'monitor'
  | 'keyboard'
  | 'book'
  | 'megaphone'
  | 'lifebuoy'
  | 'density'
  | 'building'
  | 'external'
  | 'star';

/** Stroke paths on a 24 px grid. Each icon may use several paths. */
export const ICON_PATHS: Readonly<Record<ShellIconName, readonly string[]>> = {
  menu: ['M4 7h16', 'M4 12h16', 'M4 17h16'],
  plus: ['M12 5v14', 'M5 12h14'],
  caret: ['M6 9l6 6 6-6'],
  search: ['M10.5 3.5a7 7 0 1 0 0 14 7 7 0 1 0 0-14z', 'M20 20l-4.3-4.3'],
  bell: ['M6 16V11a6 6 0 0 1 12 0v5l2 2H4z', 'M10 21h4'],
  help: [
    'M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18z',
    'M9.5 9.5a2.5 2.5 0 1 1 3.6 2.2c-.7.4-1.1 1-1.1 1.8',
    'M12 17v.01',
  ],
  close: ['M6 6l12 12', 'M18 6L6 18'],
  board: ['M4 4h7v7H4z', 'M13 4h7v7h-7z', 'M4 13h7v7H4z', 'M13 13h7v7h-7z'],
  backlog: ['M9 6h11', 'M9 12h11', 'M9 18h11', 'M4 6h.01', 'M4 12h.01', 'M4 18h.01'],
  sprints: ['M13 2L4 14h7l-1 8 9-12h-7z'],
  calendar: ['M4 6h16v14H4z', 'M4 10h16', 'M8 3v4', 'M16 3v4'],
  permissions: ['M12 3l8 3v6c0 5-3.5 8-8 9-4.5-1-8-4-8-9V6z'],
  settings: [
    'M12 9a3 3 0 1 0 0 6 3 3 0 0 0 0-6z',
    'M4 12h2M18 12h2M12 4v2M12 18v2M6.3 6.3l1.4 1.4M16.3 16.3l1.4 1.4M6.3 17.7l1.4-1.4M16.3 7.7l1.4-1.4',
  ],
  members: [
    'M9 11a3 3 0 1 0 0-6 3 3 0 0 0 0 6z',
    'M3 20c0-3 3-5 6-5s6 2 6 5',
    'M16 11a3 3 0 1 0 0-6',
    'M17 15c2 .5 4 2 4 5',
  ],
  invitations: ['M3 6h18v12H3z', 'M3 7l9 6 9-6'],
  profile: ['M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8z', 'M4 21c0-4 4-6 8-6s8 2 8 6'],
  security: ['M6 11h12v9H6z', 'M8 11V8a4 4 0 0 1 8 0v3', 'M12 15v2'],
  appearance: [
    'M12 3a9 9 0 1 0 0 18c1.5 0 2-1 2-2 0-1.5-1-1.5-1-3a2 2 0 0 1 2-2h3a3 3 0 0 0 3-3c0-4.4-4-8-9-8z',
    'M7.5 11h.01',
    'M10 7.5h.01',
    'M14.5 7.5h.01',
  ],
  fold: ['M15 6l-6 6 6 6'],
  unfold: ['M9 6l6 6-6 6'],
  folder: ['M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z'],
  issue: ['M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18z', 'M8.5 12.5l2.5 2.5 4.5-5'],
  dashboard: ['M4 4h7v9H4z', 'M13 4h7v5h-7z', 'M13 11h7v9h-7z', 'M4 15h7v5H4z'],
  team: [
    'M12 11a3 3 0 1 0 0-6 3 3 0 0 0 0 6z',
    'M6 20c0-3 2.7-5 6-5s6 2 6 5',
    'M5 11a2 2 0 1 0 0-4',
    'M19 11a2 2 0 1 0 0-4',
  ],
  clock: ['M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18z', 'M12 7v5l3 2'],
  check: ['M5 12.5l4.5 4.5L19 7.5'],
  logout: ['M10 4H5v16h5', 'M14 8l4 4-4 4', 'M18 12H9'],
  sun: [
    'M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8z',
    'M12 2v2M12 20v2M2 12h2M20 12h2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4',
  ],
  moon: ['M20 14.5A8 8 0 0 1 9.5 4 8 8 0 1 0 20 14.5z'],
  monitor: ['M3 5h18v11H3z', 'M8 20h8', 'M12 16v4'],
  keyboard: [
    'M3 6h18v12H3z',
    'M7 10h.01M11 10h.01M15 10h.01M17 10h.01',
    'M7 14h.01M17 14h.01',
    'M10 14h4',
  ],
  book: ['M5 4h11a3 3 0 0 1 3 3v13H8a3 3 0 0 1-3-3z', 'M5 17a3 3 0 0 1 3-3h11'],
  megaphone: ['M4 10v4h3l8 4V6L7 10z', 'M18 9a4 4 0 0 1 0 6'],
  lifebuoy: [
    'M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18z',
    'M12 9a3 3 0 1 0 0 6 3 3 0 0 0 0-6z',
    'M5.6 5.6l3.3 3.3M15.1 15.1l3.3 3.3M5.6 18.4l3.3-3.3M15.1 8.9l3.3-3.3',
  ],
  density: ['M4 6h16', 'M4 10h16', 'M4 14h16', 'M4 18h16'],
  building: ['M5 21V4h9v17', 'M14 9h5v12', 'M3 21h18', 'M8 8h2M8 12h2M8 16h2'],
  external: ['M14 4h6v6', 'M20 4l-9 9', 'M18 14v6H4V6h6'],
  star: ['M12 3.5l2.6 5.3 5.9.9-4.2 4.1 1 5.8L12 16.9l-5.3 2.7 1-5.8L3.5 9.7l5.9-.9z'],
};
