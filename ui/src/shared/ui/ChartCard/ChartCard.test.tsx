import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { ChartCard } from './ChartCard'
import { CHART_SERIES, seriesColor } from './palette'

const table = {
  headers: ['Estado', 'Incidencias'],
  rows: [
    ['Abiertas', 12],
    ['Cerradas', 5],
  ],
}

describe('ChartCard', () => {
  it('renders the title, the chart and a labelled section', () => {
    render(
      <ChartCard title="Incidencias por estado" table={table}>
        <div>gráfico</div>
      </ChartCard>,
    )
    expect(screen.getByRole('region', { name: 'Incidencias por estado' })).toBeInTheDocument()
    expect(screen.getByText('gráfico')).toBeInTheDocument()
  })

  it('toggles the data table with the "Ver datos" button', () => {
    render(
      <ChartCard title="Incidencias" table={table}>
        <div />
      </ChartCard>,
    )
    const button = screen.getByRole('button', { name: 'Ver datos' })
    expect(button).toHaveAttribute('aria-expanded', 'false')
    expect(screen.getByRole('table', { name: 'Datos: Incidencias' })).toBeInTheDocument()
    fireEvent.click(button)
    expect(button).toHaveAttribute('aria-expanded', 'true')
  })

  it('exposes the table rows with real headers', () => {
    render(
      <ChartCard title="Incidencias" table={table}>
        <div />
      </ChartCard>,
    )
    expect(screen.getByRole('columnheader', { name: 'Incidencias' })).toBeInTheDocument()
    expect(screen.getByRole('rowheader', { name: 'Abiertas' })).toBeInTheDocument()
    expect(screen.getByRole('cell', { name: '12' })).toBeInTheDocument()
  })

  it('shows the empty text instead of the chart and the data toggle', () => {
    render(
      <ChartCard title="Vacío" table={{ headers: [], rows: [] }} isEmpty>
        <div>no debería verse</div>
      </ChartCard>,
    )
    expect(screen.getByText('Sin datos para mostrar')).toBeInTheDocument()
    expect(screen.queryByText('no debería verse')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Ver datos' })).not.toBeInTheDocument()
  })
})

describe('chart palette', () => {
  it('returns token references only and cycles through the series', () => {
    for (const color of CHART_SERIES) {
      expect(color).toMatch(/^var\(--cl-[a-z-]+\)$/)
    }
    expect(seriesColor(0)).toBe(seriesColor(CHART_SERIES.length))
    expect(seriesColor(-1)).toBe(CHART_SERIES[CHART_SERIES.length - 1])
  })
})
