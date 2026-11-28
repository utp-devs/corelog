/**
 * The only place where accessibility findings may be waived. Every entry names a third-party widget
 * internal that the application cannot change, with the reason. Application markup never goes here:
 * it is fixed in the component.
 */
export interface AxeExclusion {
  /** CSS selector of the third-party element left out of the scan. */
  readonly selector: string;
  /** Why the element cannot be fixed in this code base. */
  readonly reason: string;
}

/** Subtrees left out of every rule. */
export const AXE_EXCLUSIONS: readonly AxeExclusion[] = [];

export interface AxeRuleExclusion extends AxeExclusion {
  /** Axe rule id waived for nodes under the selector (the rest of the rules still run there). */
  readonly rule: string;
}

/** A single rule waived under a selector; every other rule keeps running on those nodes. */
export const AXE_RULE_EXCLUSIONS: readonly AxeRuleExclusion[] = [
  {
    rule: 'region',
    selector: '.cdk-overlay-container',
    reason:
      'Menus and popovers are rendered by the CDK overlay in a container appended to body, outside the page landmarks by design. They are transient widgets (role menu, listbox, dialog) that carry their own role and name, so a landmark region would add nothing.',
  },
  {
    rule: 'region',
    selector: '.p-overlay, .p-select-overlay, .p-popover, .p-datepicker, .p-multiselect-overlay',
    reason:
      'PrimeNG overlay panels are appended to body by the library; same reasoning as the CDK overlay container.',
  },
];
