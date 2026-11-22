import styles from './ToneTag.module.css'

export type Tone = 'neutral' | 'blue' | 'gold' | 'green' | 'orange' | 'red'

type ToneTagProps = {
  label: string
  tone: Tone
}

/** Chip de color semántico. Los colores salen de tokens --cl-*, así que el modo oscuro funciona sin reglas extra. */
export function ToneTag({ label, tone }: ToneTagProps) {
  return <span className={`${styles.tag} ${styles[tone]}`}>{label}</span>
}
