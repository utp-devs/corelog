import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { Button } from 'primeng/button';

interface StateCopy {
  readonly icon: string;
  readonly title: string;
  readonly description: string;
}

const NETWORK_COPY: StateCopy = {
  icon: 'pi pi-wifi',
  title: 'Sin conexión',
  description: 'No pudimos comunicarnos con el servidor. Revisa tu conexión e inténtalo de nuevo.',
};

const FORBIDDEN_COPY: StateCopy = {
  icon: 'pi pi-lock',
  title: 'No tienes acceso a este contenido',
  description: 'Tu cuenta no tiene permiso para verlo. Pide acceso a un administrador.',
};

const NOT_FOUND_COPY: StateCopy = {
  icon: 'pi pi-search',
  title: 'No encontramos lo que buscas',
  description: 'Puede que se haya movido o eliminado.',
};

const SERVER_COPY: StateCopy = {
  icon: 'pi pi-exclamation-triangle',
  title: 'Algo salió mal',
  description: 'Tuvimos un problema al cargar la información. Inténtalo de nuevo en unos minutos.',
};

const DEFAULT_COPY: StateCopy = {
  icon: 'pi pi-exclamation-circle',
  title: 'No se pudo cargar la información',
  description: 'Inténtalo de nuevo. Si el problema continúa, avisa a un administrador.',
};

/** Words that give away an untranslated API error message. */
const RAW_ENGLISH =
  /\b(the|failed|failure|error|not found|forbidden|unauthorized|internal|invalid|request|timeout|server|network|unable|cannot|could not|bad gateway|service)\b/i;

/** True when the text is plain ASCII with common English error words, so it must not reach users. */
export function isRawEnglishMessage(text: string): boolean {
  return /^[\x20-\x7E]+$/.test(text) && RAW_ENGLISH.test(text);
}

function copyForStatus(status: number | null | undefined): StateCopy | null {
  if (status === undefined || status === null) {
    return null;
  }
  if (status === 0) {
    return NETWORK_COPY;
  }
  if (status === 401 || status === 403) {
    return FORBIDDEN_COPY;
  }
  if (status === 404) {
    return NOT_FOUND_COPY;
  }
  return status >= 500 ? SERVER_COPY : null;
}

/**
 * Load error with a retry action. The host uses role alert so screen readers announce it.
 * The copy depends on the HTTP `status` (0 means no connection). A `message` written in Spanish by
 * the caller is shown as helper text; raw English API messages are replaced by friendly copy.
 * The retry button is shown only when `canRetry` is true and the `retry` output is bound.
 */
@Component({
  selector: 'app-error-state',
  imports: [Button],
  templateUrl: './error-state.html',
  styleUrl: './error-state.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ErrorState {
  /** Replaces the title chosen from the status. */
  readonly title = input<string>();
  readonly message = input<string>();
  /** HTTP status of the failure, when known. */
  readonly status = input<number | null>();
  readonly canRetry = input(false);
  readonly retry = output<void>();

  private readonly copy = computed(() => copyForStatus(this.status()) ?? DEFAULT_COPY);

  protected readonly icon = computed(() => this.copy().icon);
  protected readonly heading = computed(() => this.title() ?? this.copy().title);
  protected readonly helper = computed(() => {
    const message = this.message()?.trim();
    if (message && !isRawEnglishMessage(message)) {
      return message;
    }
    return this.copy().description;
  });
}
