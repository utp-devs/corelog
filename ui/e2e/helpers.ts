import { expect, type Page } from '@playwright/test'

// Credenciales de la cuenta de prueba del stack local. La contrasena nunca se versiona:
// defina E2E_USER_PASSWORD en el entorno o en ui/.env.e2e (ignorado por git).
const password = process.env.E2E_USER_PASSWORD
if (!password) {
  throw new Error('Defina E2E_USER_PASSWORD (contrasena de la cuenta de prueba) antes de correr los e2e')
}

export const credentials = {
  email: process.env.E2E_USER_EMAIL ?? 'admin@corelog.local',
  password,
}

/** Espera a que todas las imagenes de la pagina terminen de cargar (evita capturas a medias). */
export async function waitForImages(page: Page): Promise<void> {
  await page.waitForFunction(() => Array.from(document.images).every((img) => img.complete))
}

/** Inicia sesion desde la pantalla de login y espera la vista de incidencias. */
export async function login(page: Page): Promise<void> {
  await page.goto('/login')
  await page.locator('input[type="email"]').fill(credentials.email)
  await page.locator('input[type="password"]').fill(credentials.password)
  await page.getByRole('button', { name: 'Iniciar sesion' }).click()
  await expect(page).toHaveURL(/\/issues/)
}
