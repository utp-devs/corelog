import type { Page } from '@playwright/test';

/**
 * Picks an option of a select: opens the combobox found by its label, then clicks the option with that
 * visible name from the open list.
 */
export async function pickSelectOption(
  page: Page,
  comboboxName: string | RegExp,
  optionText: string,
): Promise<void> {
  await page.getByRole('combobox', { name: comboboxName }).click();
  await page.getByRole('option', { name: optionText, exact: true }).click();
}
