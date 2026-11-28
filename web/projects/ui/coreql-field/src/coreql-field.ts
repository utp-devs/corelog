import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  effect,
  inject,
  input,
  model,
  output,
  signal,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { SearchQueryApiService, type CoreqlProblem } from '@corelog/core';
import { CoreqlInput, type CoreqlSuggestProvider } from '@corelog/ui/coreql-input';
import { Subject, catchError, debounceTime, map, of, switchMap } from 'rxjs';

/** Delay between the last keystroke and the validation request, in milliseconds. */
const VALIDATE_DEBOUNCE_MS = 400;

/**
 * CoreQL text field with live validation and autocomplete: the editor of the advanced search wired to the
 * server, for forms that store a query (automation conditions, webhook filters, exports). Problems are
 * underlined and listed under the text; `valid` reports whether the current text parses (empty counts as valid).
 */
@Component({
  selector: 'app-coreql-field',
  imports: [CoreqlInput],
  templateUrl: './coreql-field.html',
  styleUrl: './coreql-field.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class CoreqlField {
  private readonly api = inject(SearchQueryApiService);
  private readonly destroyRef = inject(DestroyRef);
  private readonly requests = new Subject<string>();

  readonly value = model('');
  readonly ariaLabel = input('Consulta CoreQL');
  readonly placeholder = input('Por ejemplo: priority = High AND statusCategory != done');
  /** Whether the text is valid; null while it has not been checked. */
  readonly valid = output<boolean>();

  protected readonly problems = signal<readonly CoreqlProblem[]>([]);

  protected readonly provider: CoreqlSuggestProvider = ({ text, cursor }) =>
    this.api.complete(text, cursor, 10).pipe(
      map((found) =>
        found.completions.map((item) => ({
          label: item.label,
          insert: item.label,
          kind: item.kind,
          detail: item.detail,
        })),
      ),
    );

  constructor() {
    this.requests
      .pipe(
        debounceTime(VALIDATE_DEBOUNCE_MS),
        switchMap((text) =>
          text.trim() === ''
            ? of({ valid: true, errors: [] as readonly CoreqlProblem[] })
            : this.api.validate(text).pipe(catchError(() => of(null))),
        ),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe((result) => {
        this.problems.set(result?.errors ?? []);
        if (result) this.valid.emit(result.valid);
      });

    effect(() => this.requests.next(this.value()));
  }
}
