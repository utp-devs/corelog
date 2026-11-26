import { render, screen } from '@testing-library/react'
import { isValidElement } from 'react'
import { createMemoryRouter, RouterProvider, type RouteObject } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'

import { OrgShellLayout } from '@/adapters/in/ui/organizations/OrgShellLayout'

import { router } from './router'

vi.mock('@/adapters/in/ui/teams', () => ({
  TeamsPage: () => <p>Página de equipos</p>,
  TeamPage: () => <p>Página de equipo</p>,
}))

type RouteWithAncestors = { route: RouteObject; ancestors: RouteObject[] }

/** Depth-first search of the route tree that also returns the chain of parent routes. */
function findRoute(routes: readonly RouteObject[], path: string, ancestors: RouteObject[] = []): RouteWithAncestors | undefined {
  for (const route of routes) {
    if (route.path === path) return { route, ancestors }
    if (route.children) {
      const found = findRoute(route.children, path, [...ancestors, route])
      if (found) return found
    }
  }
  return undefined
}

describe('router: team page', () => {
  it('registers teams/:id under the organization shell', () => {
    const found = findRoute(router.routes as RouteObject[], 'teams/:id')
    expect(found).toBeDefined()
    const inShell = found?.ancestors.some((ancestor) => isValidElement(ancestor.element) && ancestor.element.type === OrgShellLayout)
    expect(inShell).toBe(true)
  })

  it('renders the team page for /teams/:id', async () => {
    const found = findRoute(router.routes as RouteObject[], 'teams/:id')
    const memory = createMemoryRouter(
      [{ path: '/teams/:id', element: found?.route.element }],
      { initialEntries: ['/teams/t-1'] },
    )
    render(<RouterProvider router={memory} />)
    expect(await screen.findByText('Página de equipo')).toBeInTheDocument()
  })
})
