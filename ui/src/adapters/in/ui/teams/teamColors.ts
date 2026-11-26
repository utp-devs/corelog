import { isHexColor, paletteIndexOf } from '@/shared/ui/ColorPicker/colorValue'

/**
 * Converts the ColorPicker value into the hex color the API stores. Custom hex values pass through; a palette
 * swatch ('palette-N') is resolved to the hex of its --cl-palette-N-bg token. Returns null when there is no
 * color or the token cannot be read, so the team keeps its derived palette color.
 */
export function resolveAvatarColor(value: string | null): string | null {
  if (value === null) return null
  if (isHexColor(value)) return value
  const index = paletteIndexOf(value)
  if (index === null || typeof document === 'undefined') return null
  const resolved = window.getComputedStyle(document.documentElement).getPropertyValue(`--cl-palette-${index}-bg`).trim()
  return isHexColor(resolved) ? resolved : null
}
