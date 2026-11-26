/**
 * Injection point for the active organization id. The state layer registers its provider at module
 * init (adapters/in/state/org.store.ts); shared code reads the id through getOrgId() and never imports
 * the adapters or the stores, so the dependency direction stays shared -> nothing.
 */
type OrgIdProvider = () => string | null

let provider: OrgIdProvider = () => null

/** Registers the function that returns the active organization id. Last registration wins. */
export function setOrgIdProvider(fn: OrgIdProvider): void {
  provider = fn
}

/** Active organization id, or null when none is selected (or no provider is registered yet). */
export function getOrgId(): string | null {
  return provider()
}
