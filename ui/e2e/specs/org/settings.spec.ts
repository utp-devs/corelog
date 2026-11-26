import { orgIdBySlug } from '../../lib/api.ts'
import { pickSelectOption } from '../../lib/ui.ts'
import { expect, test } from '../../fixtures/test.ts'

const ACME_PATH = '/organization/settings'

test.describe('configuracion de la organizacion (S-20)', () => {
  test('owner cambia la visibilidad por defecto y la ve tras recargar', async ({ as, api }) => {
    const owner = await api('owner')
    const acme = await orgIdBySlug(owner, 'acme')
    const before = ((await (await owner.get(`organizations/${acme}`)).json()) as { data: { settings: { default_project_visibility: string } } }).data.settings
      .default_project_visibility
    const target = before === 'private' ? 'open' : 'private'
    const label = target === 'private' ? 'Privada: solo para quienes se añaden' : 'Abierta: visible para todos los miembros'

    try {
      const page = await as('owner')
      await page.goto(ACME_PATH)
      await pickSelectOption(page, 'Visibilidad por defecto de los proyectos', label)
      await page.getByRole('button', { name: 'Guardar cambios' }).click()
      await expect(page.getByText('Configuración guardada')).toBeVisible()

      await page.reload()
      await expect(page.getByRole('combobox', { name: 'Visibilidad por defecto de los proyectos' })).toBeVisible()
      await expect(page.getByText(label, { exact: true })).toBeVisible()
      const after = ((await (await owner.get(`organizations/${acme}`)).json()) as { data: { settings: { default_project_visibility: string } } }).data.settings
        .default_project_visibility
      expect(after).toBe(target)
    } finally {
      await owner.patch(`organizations/${acme}`, { data: { settings: { default_project_visibility: before } } })
    }
  })

  test('owner ve la zona de peligro', async ({ as }) => {
    const page = await as('owner')
    await page.goto(ACME_PATH)
    await expect(page.getByRole('heading', { name: 'Zona de peligro' })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Eliminar organización' })).toBeVisible()
  })

  test('orgadmin ve la configuracion pero no las acciones de propietario', async ({ as }) => {
    const page = await as('orgadmin')
    await page.goto(ACME_PATH)
    await expect(page.getByRole('heading', { name: 'Configuración de la organización' })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Eliminar organización' })).toHaveCount(0)
    await expect(page.getByRole('button', { name: 'Transferir propiedad' })).toHaveCount(0)
    await expect(page.getByText('Solo el propietario puede transferir la propiedad o eliminar la organización.')).toBeVisible()
  })

  for (const role of ['member', 'guest'] as const) {
    test(`${role} recibe la pagina 403`, async ({ as }) => {
      const page = await as(role)
      await page.goto(ACME_PATH)
      await expect(page.getByRole('heading', { name: 'No tienes acceso a esta página' })).toBeVisible()
    })
  }

  test('el API rechaza el cambio de politica a un miembro', async ({ api }) => {
    const member = await api('member')
    const acme = await orgIdBySlug(member, 'acme')
    const response = await member.patch(`organizations/${acme}`, { data: { settings: { who_can_create_teams: 'admins' } } })
    expect(response.status()).toBe(403)
  })
})
