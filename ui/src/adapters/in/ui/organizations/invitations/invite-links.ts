/** Copies a link; the clipboard may be unavailable (insecure context), in which case the caller tells the user. */
export async function copyInviteLink(url: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(url)
    return true
  } catch {
    return false
  }
}
