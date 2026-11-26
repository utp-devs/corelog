import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it } from 'vitest'

import { useOrgStore } from '@/adapters/in/state/org.store'

import { OnboardingPage } from './OnboardingPage'

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <OnboardingPage />
    </MemoryRouter>,
  )
}

function currentStep() {
  const items = screen.getAllByRole('listitem').filter((item) => item.closest('nav[aria-label="Pasos del asistente"]'))
  return items.find((item) => item.getAttribute('aria-current') === 'step')
}

describe('OnboardingPage step indicator', () => {
  beforeEach(() => {
    localStorage.clear()
    useOrgStore.getState().reset()
  })

  it('labels the step list and marks the first step as current', () => {
    renderAt('/onboarding')
    expect(screen.getByRole('navigation', { name: 'Pasos del asistente' })).toBeInTheDocument()
    expect(currentStep()).toHaveTextContent('Elegir')
  })

  it('marks the organization step as current when started with ?new=1', () => {
    renderAt('/onboarding?new=1')
    expect(currentStep()).toHaveTextContent('Organización')
    expect(screen.getByRole('heading', { name: 'Datos de la organización' })).toBeInTheDocument()
  })
})
