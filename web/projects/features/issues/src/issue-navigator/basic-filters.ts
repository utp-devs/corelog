import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  inject,
  input,
  output,
  signal,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormsModule } from '@angular/forms';
import {
  ASSIGNED_TO_ME,
  BasicFilters,
  SearchQueryApiService,
  UNASSIGNED,
  type CoreqlValueOption,
  type SuggestField,
} from '@corelog/core';
import { InputText } from 'primeng/inputtext';
import { MultiSelect } from 'primeng/multiselect';
import { Subject, debounceTime, distinctUntilChanged } from 'rxjs';

export interface FilterOptionItem {
  readonly value: string;
  readonly label: string;
}

const STATUS_CATEGORY_OPTIONS: FilterOptionItem[] = [
  { value: 'todo', label: 'Por hacer' },
  { value: 'in_progress', label: 'En curso' },
  { value: 'done', label: 'Hecho' },
];

const PEOPLE_EXTRAS: FilterOptionItem[] = [
  { value: ASSIGNED_TO_ME, label: 'Yo' },
  { value: UNASSIGNED, label: 'Sin asignar' },
];

/** Delay before typed text becomes a filter, in milliseconds. */
export const TEXT_DEBOUNCE_MS = 400;

type ListKey =
  | 'projects'
  | 'types'
  | 'statusCategories'
  | 'assignees'
  | 'reporters'
  | 'labels'
  | 'components'
  | 'fixVersions';
type DateKey = 'createdFrom' | 'createdTo' | 'updatedFrom' | 'updatedTo';

/** Basic filters of the navigator: selectors for the usual fields, a text box and date ranges. */
@Component({
  selector: 'app-basic-filters',
  imports: [FormsModule, InputText, MultiSelect],
  templateUrl: './basic-filters.html',
  styleUrl: './basic-filters.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class BasicFiltersPanel {
  private readonly api = inject(SearchQueryApiService);
  private readonly destroyRef = inject(DestroyRef);
  private readonly typed = new Subject<string>();

  readonly filters = input.required<BasicFilters>();
  readonly filtersChange = output<BasicFilters>();

  protected readonly projects = signal<FilterOptionItem[]>([]);
  protected readonly types = signal<FilterOptionItem[]>([]);
  protected readonly people = signal<FilterOptionItem[]>([...PEOPLE_EXTRAS]);
  protected readonly labels = signal<FilterOptionItem[]>([]);
  protected readonly components = signal<FilterOptionItem[]>([]);
  protected readonly versions = signal<FilterOptionItem[]>([]);
  protected readonly statusCategories = STATUS_CATEGORY_OPTIONS;

  constructor() {
    this.load('project', this.projects, (option) => ({
      value: option.value,
      label: `${option.value} · ${option.label}`,
    }));
    this.load('type', this.types);
    this.load('assignee', this.people, undefined, PEOPLE_EXTRAS);
    this.load('labels', this.labels);
    this.load('component', this.components);
    this.load('fixVersion', this.versions);
    this.typed
      .pipe(
        debounceTime(TEXT_DEBOUNCE_MS),
        distinctUntilChanged(),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe((text) => this.update({ text }));
  }

  /** Editing the list of a field. */
  protected setList(key: ListKey, value: string[] | null): void {
    this.update({ [key]: value ?? [] });
  }

  protected setDate(key: DateKey, value: string): void {
    this.update({ [key]: value });
  }

  protected onText(value: string): void {
    this.typed.next(value);
  }

  protected commitText(value: string): void {
    this.update({ text: value });
  }

  private update(changes: Partial<BasicFilters>): void {
    this.filtersChange.emit({ ...this.filters(), ...changes });
  }

  private load(
    field: SuggestField,
    target: { set(value: FilterOptionItem[]): void },
    render: (option: CoreqlValueOption) => FilterOptionItem = (option) => ({
      value: option.value,
      label: option.label,
    }),
    leading: FilterOptionItem[] = [],
  ): void {
    this.api
      .values(field, '', 25)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (result) => target.set([...leading, ...result.values.map(render)]),
        error: () => target.set([...leading]),
      });
  }
}
