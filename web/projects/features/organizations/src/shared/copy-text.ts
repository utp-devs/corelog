/** Copies text to the clipboard. Resolves false when the clipboard is unavailable (insecure context, denied). */
export async function copyText(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    return false;
  }
}
