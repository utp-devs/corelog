import type { Page } from '@playwright/test';

/**
 * Picks an option of an antd Select: opens the combobox found by its label, then clicks the option's
 * visible text. antd option nodes are not exposed as role=option in the rendered list.
 */
export async function pickSelectOption(
  page: Page,
  comboboxName: string | RegExp,
  optionText: string,
): Promise<void> {
  await page.getByRole('combobox', { name: comboboxName }).click();
  // The open dropdown is rendered last in the DOM; earlier matches are the current values of other selects.
  await page.getByText(optionText, { exact: true }).last().click();
}
