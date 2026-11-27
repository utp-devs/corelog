/** Default cover photographs, abstract and without people. */
export const PROJECT_COVERS: readonly string[] = [
  '/images/covers/cover-01-blue-glass.webp',
  '/images/covers/cover-02-teal-concrete.webp',
  '/images/covers/cover-03-violet-fabric.webp',
  '/images/covers/cover-04-amber-wood.webp',
  '/images/covers/cover-05-green-leaf.webp',
  '/images/covers/cover-06-graphite-metal.webp',
];

/** Stable 32-bit FNV-1a hash of a text. */
function hash(text: string): number {
  let value = 0x811c9dc5;
  for (let i = 0; i < text.length; i++) {
    value ^= text.charCodeAt(i);
    value = Math.imul(value, 0x01000193) >>> 0;
  }
  return value;
}

/**
 * Cover photograph of a project. It is chosen from the project color key (so projects that share a color share
 * a look) and falls back to the project key; the same input always gives the same cover.
 */
export function projectCover(color: string | null | undefined, key: string): string {
  const seed = color?.trim() || key.trim().toUpperCase();
  return PROJECT_COVERS[hash(seed) % PROJECT_COVERS.length];
}
