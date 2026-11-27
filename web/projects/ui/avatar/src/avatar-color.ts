export const AVATAR_COLOR_COUNT = 10;
export const PALETTE_COUNT = 12;

/** FNV-1a 32-bit hash: the same id gives the same value in every session. */
export function hashString(value: string): number {
  let hash = 0x811c9dc5;
  for (let i = 0; i < value.length; i += 1) {
    hash ^= value.charCodeAt(i);
    hash = Math.imul(hash, 0x01000193);
  }
  return hash >>> 0;
}

/** Index 1..10 of the --cl-avatar-N background for a user id. */
export function avatarColorIndex(id: string): number {
  return (hashString(id) % AVATAR_COLOR_COUNT) + 1;
}

/** Index 1..12 of the --cl-palette-N color for a key (projects, teams). */
export function paletteIndex(key: string): number {
  return (hashString(key) % PALETTE_COUNT) + 1;
}
