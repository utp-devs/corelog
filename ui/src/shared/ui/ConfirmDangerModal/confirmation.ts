/** The danger action is enabled only when the typed text equals the expected name exactly. */
export function isConfirmationMatch(typed: string, expected: string): boolean {
  return expected.length > 0 && typed === expected
}
