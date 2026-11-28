import type { Page } from '@playwright/test';

export interface RegisterData {
  readonly firstName: string;
  readonly lastName: string;
  readonly phone: string;
  readonly email: string;
  readonly password: string;
  /** Defaults to the password. */
  readonly confirmation?: string;
}

/** Fills the first step (data) and presses "Siguiente". */
export async function fillRegisterStep1(page: Page, data: RegisterData): Promise<void> {
  await page.getByLabel(/^Nombres/).fill(data.firstName);
  await page.getByLabel(/^Apellidos/).fill(data.lastName);
  await page.getByLabel(/^Teléfono/).fill(data.phone);
}

/** Fills the second step (account). */
export async function fillRegisterStep2(page: Page, data: RegisterData): Promise<void> {
  await page.getByLabel('Correo electrónico').fill(data.email);
  await page.getByLabel(/^Contraseña/).fill(data.password);
  await page.getByLabel(/^Confirmar contraseña/).fill(data.confirmation ?? data.password);
}

/**
 * Fills the registration steps pressing "Siguiente" between them and stops on the last step (privacy), with the
 * consents still to tick and "Crear cuenta" still to press.
 */
export async function fillRegisterSteps(page: Page, data: RegisterData): Promise<void> {
  await fillRegisterStep1(page, data);
  await page.getByTestId('register-next').click();
  await page.getByTestId('register-step-2').waitFor({ state: 'visible' });
  await fillRegisterStep2(page, data);
  await page.getByTestId('register-next').click();
  await page.getByTestId('register-step-3').waitFor({ state: 'visible' });
}
