import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  ElementRef,
  computed,
  inject,
  input,
  model,
  output,
  signal,
  viewChild,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import {
  CoreqlCursorContext,
  CoreqlProblem,
  applyCompletion,
  coreqlCursorContext,
} from '@corelog/core';
import { Observable, Subject, catchError, debounceTime, of, switchMap } from 'rxjs';

import { errorSegments, replaceRange } from './coreql-text';

/** One entry of the autocomplete list. `insert` is what goes into the text. */
export interface CoreqlSuggestion {
  readonly label: string;
  readonly insert: string;
  readonly kind: string;
  readonly detail: string;
}

export interface CoreqlSuggestRequest {
  readonly text: string;
  readonly cursor: number;
  readonly context: CoreqlCursorContext;
}

export type CoreqlSuggestProvider = (
  request: CoreqlSuggestRequest,
) => Observable<readonly CoreqlSuggestion[]>;

/** Delay between the last keystroke and the autocomplete question, in milliseconds. */
export const SUGGEST_DEBOUNCE_MS = 150;

const KIND_LABELS: Readonly<Record<string, string>> = {
  field: 'Campo',
  operator: 'Operador',
  keyword: 'Palabra clave',
  function: 'Función',
  value: 'Valor',
};

let nextId = 0;

/**
 * Editor of CoreQL text: underlines the problems the server reported, lists them with their suggested fixes
 * and offers autocomplete (fields, operators, values) as a listbox under the text. Ctrl+Enter runs the query.
 */
@Component({
  selector: 'app-coreql-input',
  templateUrl: './coreql-input.html',
  styleUrl: './coreql-input.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class CoreqlInput {
  private readonly destroyRef = inject(DestroyRef);
  private readonly field = viewChild.required<ElementRef<HTMLTextAreaElement>>('field');
  private readonly requests = new Subject<{ text: string; cursor: number }>();
  protected readonly id = `coreql-input-${(nextId += 1)}`;

  readonly value = model('');
  readonly problems = input<readonly CoreqlProblem[]>([]);
  readonly provider = input<CoreqlSuggestProvider | null>(null);
  readonly placeholder = input(
    'Escribe una consulta, por ejemplo: project = KAN AND statusCategory = todo',
  );
  readonly ariaLabel = input('Consulta avanzada');
  /** Ctrl+Enter, or Enter on a single line query. */
  readonly run = output<void>();

  protected readonly suggestions = signal<readonly CoreqlSuggestion[]>([]);
  protected readonly open = signal(false);
  protected readonly active = signal(0);
  private context: CoreqlCursorContext | null = null;

  protected readonly segments = computed(() => errorSegments(this.value(), this.problems()));
  protected readonly activeId = computed(() =>
    this.open() && this.suggestions().length > 0 ? `${this.id}-option-${this.active()}` : null,
  );

  constructor() {
    this.requests
      .pipe(
        debounceTime(SUGGEST_DEBOUNCE_MS),
        switchMap((request) => this.ask(request.text, request.cursor)),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe((items) => this.show(items));
  }

  protected kindLabel(kind: string): string {
    return KIND_LABELS[kind] ?? kind;
  }

  protected onInput(event: Event): void {
    const area = event.target as HTMLTextAreaElement;
    this.value.set(area.value);
    this.requests.next({ text: area.value, cursor: area.selectionStart });
  }

  protected onKeyDown(event: KeyboardEvent): void {
    if (event.key === 'Enter' && (event.ctrlKey || event.metaKey)) {
      event.preventDefault();
      this.close();
      this.run.emit();
      return;
    }
    if (event.key === ' ' && event.ctrlKey) {
      event.preventDefault();
      const area = this.field().nativeElement;
      this.ask(area.value, area.selectionStart).subscribe((items) => this.show(items));
      return;
    }
    if (!this.open() || this.suggestions().length === 0) {
      if (event.key === 'Enter' && !event.shiftKey) {
        // The query is a single logical line: Enter runs it instead of adding a line break.
        event.preventDefault();
        this.run.emit();
      }
      return;
    }
    const count = this.suggestions().length;
    switch (event.key) {
      case 'ArrowDown':
        event.preventDefault();
        this.active.update((index) => (index + 1) % count);
        break;
      case 'ArrowUp':
        event.preventDefault();
        this.active.update((index) => (index - 1 + count) % count);
        break;
      case 'Enter':
      case 'Tab':
        event.preventDefault();
        this.accept(this.active());
        break;
      case 'Escape':
        event.preventDefault();
        event.stopPropagation();
        this.close();
        break;
    }
  }

  protected onBlur(): void {
    this.close();
  }

  /** Mouse selection keeps the focus in the text (mousedown, not click). */
  protected accept(index: number, event?: Event): void {
    event?.preventDefault();
    const item = this.suggestions()[index];
    const context = this.context;
    if (!item || !context) return;
    const area = this.field().nativeElement;
    const result = applyCompletion(area.value, context, item.insert, item.kind);
    this.value.set(result.text);
    this.close();
    queueMicrotask(() => {
      area.focus();
      area.setSelectionRange(result.cursor, result.cursor);
      this.requests.next({ text: result.text, cursor: result.cursor });
    });
  }

  /** Applies the fix a problem suggests. */
  protected applyFix(problem: CoreqlProblem, replacement: string): void {
    const result = replaceRange(this.value(), problem.position, problem.length, replacement);
    this.value.set(result.text);
    const area = this.field().nativeElement;
    queueMicrotask(() => {
      area.focus();
      area.setSelectionRange(result.cursor, result.cursor);
    });
  }

  private ask(text: string, cursor: number): Observable<readonly CoreqlSuggestion[]> {
    const provider = this.provider();
    if (!provider || text.trim() === '') return of([]);
    this.context = coreqlCursorContext(text, cursor);
    return provider({ text, cursor, context: this.context }).pipe(catchError(() => of([])));
  }

  private show(items: readonly CoreqlSuggestion[]): void {
    const typed = this.context?.prefix.toLowerCase() ?? '';
    // A single suggestion that is exactly what is typed has nothing left to offer.
    const useful = items.filter((item) => item.insert.toLowerCase() !== typed || items.length > 1);
    this.suggestions.set(useful);
    this.active.set(0);
    this.open.set(useful.length > 0 && document.activeElement === this.field().nativeElement);
  }

  private close(): void {
    this.open.set(false);
  }
}
