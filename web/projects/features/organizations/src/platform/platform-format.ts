import type { EmailOutboxStatus, ScheduledJobRunStatus } from '@corelog/core';
import type { Tone } from '@corelog/ui/tone-tag';

const relative = new Intl.RelativeTimeFormat('es', { numeric: 'auto' });

const UNITS: readonly { unit: Intl.RelativeTimeFormatUnit; seconds: number }[] = [
  { unit: 'day', seconds: 86_400 },
  { unit: 'hour', seconds: 3_600 },
  { unit: 'minute', seconds: 60 },
  { unit: 'second', seconds: 1 },
];

/** "hace 5 minutos" / "dentro de 2 horas"; a missing or invalid date shows a dash. */
export function relativeTimeLabel(iso: string | null | undefined, now: Date = new Date()): string {
  if (!iso) return '-';
  const time = new Date(iso).getTime();
  if (Number.isNaN(time)) return '-';
  const diff = Math.round((time - now.getTime()) / 1000);
  const abs = Math.abs(diff);
  for (const { unit, seconds } of UNITS) {
    if (abs >= seconds || unit === 'second') {
      return relative.format(Math.trunc(diff / seconds), unit);
    }
  }
  return '-';
}

/** Exact date and time, shown as the tooltip of a relative label. */
export function exactTimeLabel(iso: string | null | undefined): string {
  if (!iso) return '';
  const date = new Date(iso);
  return Number.isNaN(date.getTime())
    ? ''
    : new Intl.DateTimeFormat('es', { dateStyle: 'medium', timeStyle: 'medium' }).format(date);
}

export const OUTBOX_STATUS_LABELS: Readonly<Record<EmailOutboxStatus, string>> = {
  pending: 'En cola',
  sent: 'Enviado',
  dead: 'Fallido',
};

export const OUTBOX_STATUS_TONES: Readonly<Record<EmailOutboxStatus, Tone>> = {
  pending: 'neutral',
  sent: 'green',
  dead: 'red',
};

export const RUN_STATUS_LABELS: Readonly<Record<ScheduledJobRunStatus, string>> = {
  running: 'En curso',
  ok: 'Correcta',
  error: 'Con error',
  abandoned: 'Abandonada',
};

export const RUN_STATUS_TONES: Readonly<Record<ScheduledJobRunStatus, Tone>> = {
  running: 'blue',
  ok: 'green',
  error: 'red',
  abandoned: 'orange',
};

/** Duration of a run: "120 ms" or "3,2 s". */
export function durationLabel(ms: number | null | undefined): string {
  if (ms === null || ms === undefined) return '-';
  if (ms < 1000) return `${ms} ms`;
  return `${(ms / 1000).toLocaleString('es', { maximumFractionDigits: 1 })} s`;
}

/** Spanish reading of a schedule such as "every 10s" or "every 24h"; unknown shapes are kept as they are. */
export function scheduleLabel(schedule: string): string {
  const daily = /^daily at\s+(\d{1,2}:\d{2})\s*(UTC)?$/i.exec(schedule.trim());
  if (daily) return `Cada día a las ${daily[1]} UTC`;
  const match = /^every\s+(\d+)\s*(ms|s|m|h|d)$/i.exec(schedule.trim());
  if (!match) return schedule;
  const value = Number(match[1]);
  const unit = match[2].toLowerCase();
  const names: Record<string, [string, string]> = {
    ms: ['milisegundo', 'milisegundos'],
    s: ['segundo', 'segundos'],
    m: ['minuto', 'minutos'],
    h: ['hora', 'horas'],
    d: ['día', 'días'],
  };
  const [one, many] = names[unit];
  return value === 1 ? `Cada ${one}` : `Cada ${value} ${many}`;
}
