import { mergeObserved, writeKnownViolations } from './lib/known-violations.ts'

// Update mode only: `E2E_UPDATE_KNOWN=1` regenerates e2e/known-violations.json from what the smoke
// journey observed in this run (all roles, all viewport projects). Otherwise it does nothing.
export default function globalTeardown(): void {
  if (process.env.E2E_UPDATE_KNOWN !== '1') return
  const baseline = mergeObserved()
  writeKnownViolations(baseline)
  console.log(`known-violations.json regenerated with ${Object.keys(baseline).length} screen|viewport entries`)
}
