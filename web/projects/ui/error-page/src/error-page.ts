import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { RouterLink } from '@angular/router';
import { Button } from 'primeng/button';

export type ErrorPageVariant = 403 | 404 | 500;

type VariantCopy = {
  readonly title: string;
  readonly description: string;
  /** PrimeIcons classes. */
  readonly icon: string;
};

const COPY: Record<ErrorPageVariant, VariantCopy> = {
  403: {
    title: 'No tienes acceso a esta página',
    description:
      'Tu cuenta no tiene permiso para ver este contenido. Pide acceso a un administrador de la organización.',
    icon: 'pi pi-lock',
  },
  404: {
    title: 'No encontramos esta página',
    description: 'La dirección no existe o el contenido se ha movido o eliminado.',
    icon: 'pi pi-question-circle',
  },
  500: {
    title: 'Algo salió mal',
    description: 'No se pudo cargar esta página. Inténtalo de nuevo o vuelve al inicio.',
    icon: 'pi pi-exclamation-triangle',
  },
};

/**
 * Full-page error for 403, 404 and 500, with a link back home and, for 500 only,
 * an optional retry action (shown when `canRetry` is true and `retry` is bound).
 */
@Component({
  selector: 'app-error-page',
  imports: [Button, RouterLink],
  templateUrl: './error-page.html',
  styleUrl: './error-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ErrorPage {
  readonly variant = input.required<ErrorPageVariant>();
  /** Replaces the default Spanish title for the variant. */
  readonly title = input<string>();
  /** Replaces the default Spanish description for the variant. */
  readonly description = input<string>();
  /** Enables the "Reintentar" action, which exists on the 500 variant only. */
  readonly canRetry = input(false);
  /** Draws the message inside a bordered surface that fills the page area (for pages with a header). */
  readonly framed = input(false);
  readonly retry = output<void>();

  protected readonly copy = computed(() => COPY[this.variant()]);
}
