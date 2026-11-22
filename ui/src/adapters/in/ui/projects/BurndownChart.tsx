import { CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'

import type { BurndownPoint } from '@/core/projects/domain/sprint'

// Burndown: trabajo restante real frente a la linea ideal. Los colores salen de los tokens de tema.
export function BurndownChart({ points }: { points: BurndownPoint[] }) {
  return (
    <ResponsiveContainer width="100%" height={260}>
      <LineChart data={points} margin={{ top: 8, right: 16, bottom: 8, left: 0 }}>
        <CartesianGrid stroke="var(--cl-border)" strokeDasharray="3 3" />
        <XAxis dataKey="date" stroke="var(--cl-text-subtle)" />
        <YAxis stroke="var(--cl-text-subtle)" allowDecimals={false} />
        <Tooltip />
        <Line type="linear" dataKey="ideal" name="Ideal" stroke="var(--cl-text-subtle)" strokeDasharray="5 5" dot={false} />
        <Line type="monotone" dataKey="remaining" name="Restante" stroke="var(--cl-brand)" strokeWidth={2} />
      </LineChart>
    </ResponsiveContainer>
  )
}
